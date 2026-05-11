# Chunk 4 Review — Phases 39–57: Conditions, Death, FES, Class Features, Standard Actions

Generated: 2026-05-10. Source of truth: `docs/dnd-async-discord-spec.md`, `docs/phases.md` lines 221–319, Coverage Map lines 833–931.

## Summary

Almost every Phase 39–57 service-layer module is implemented at the package level inside `internal/combat`, with strong unit-test coverage (typically 30–100 tests per file). The core conditions engine (39, 40), prone movement (41), death-save state machine (43), FES processor (44), class-feature definitions (45), grapple/shove (56), hide (57), Action Surge (53), and the various class-action services (rage 46, wild shape 47, monk 48a/48b, bardic inspiration 49, channel divinity 50, divine smite 51, lay on hands 52) all have full Go implementations.

**However, four large integration gaps mean most of these phases are not actually reachable from gameplay:**

1. **FES integration into the attack pipeline is broken.** `internal/combat/attack.go:456` checks `if len(input.Features) > 0 { ProcessEffects(...) }`, but `input.Features` is never assigned anywhere in production code (`grep "Features = " internal/combat/*.go` only matches tests). `Attack()` (line 773) and `attackImprovised()` (line 872) build the input via `buildAttackInput()` (line 1037) which does not populate `Features`. The `BuildAttackEffectContext()` call inside `ProcessEffects` only sets `Weapon` (line 458), so `IsRaging`, `HasAdvantage`, `AllyWithinFt`, `WearingArmor`, `OneHandedMeleeOnly`, `AbilityUsed`, `UsedThisTurn` are all zero-valued. Net effect: Sneak Attack, Rage damage bonus, Archery/Defense/Dueling, Pack Tactics never fire when a real attack is rolled. `BuildFeatureDefinitions()` is only called from tests and `internal/magicitem/integration_test.go:183`.

2. **Damage processing (Phase 42) is not in the damage path.** `ApplyDamageResistances`, `AbsorbTempHP`, and `GrantTempHP` (`internal/combat/damage.go:13/49/57`) are only called from `damage_test.go`. The production damage path (`applyDamageHP` in `concentration.go:281`, plus AoE/turn-builder/dashboard callers) writes the raw damage straight to `hp_current` with no R/I/V handling, no temp-HP absorption, and no exhaustion HP halving.

3. **Slash commands for almost every class-feature command are stubs.** `internal/discord/router.go:198` lists the game commands and `:215` wires them to `StatusAwareStubHandler` whose `Handle()` (`registration_handler.go:418`) returns "/<name> is not yet implemented." There are no `SetAttackHandler`, `SetCastHandler`, `SetBonusHandler`, `SetShoveHandler`, `SetDeathSaveHandler`, `SetInteractHandler`, `SetCommandHandler` (summon is the only `command` set), or `SetUndoHandler`/`SetPrepareHandler`/`SetRetireHandler` methods on `CommandRouter`. `/action` is wired to a handler that only does freeform text + cancel — none of `/action dash|disengage|dodge|help|hide|stand|drop-prone|escape|grapple|lay-on-hands|surge|channel-divinity` reach the Service. `cmd/dndnd/discord_handlers.go:195 attachPhase105Handlers` only attaches: move, fly, distance, done, check, save, rest, summon, recap, use, reaction, status, whisper, action, loot.

4. **Opportunity attacks (Phase 55) have full detection logic but no /move integration.** `DetectOpportunityAttacks` and `OATrigger` are only referenced inside `internal/combat/opportunity_attack*.go`. `internal/discord/move_handler.go` never calls them, so OAs do not fire on movement. The reaction system is generic declaration-based and has no auto-prompt path.

The character-card rendering note in `MEMORY.md/project_character_card_deferred_fields.md` is still accurate: the format struct *has* `Conditions/Concentration/Exhaustion` fields (`internal/charactercard/format.go:33-35`) and the formatter uses them, but `buildCardData` (`service.go:203`) never populates any of them. All character cards always show `Conditions: —`, `Concentration: —`, no Exhaustion line. The status panel (`/status`) does populate exhaustion, rage, wild-shape, bardic inspiration via `internal/discord/status_handler.go:170-197`, so the data is reachable — it just hasn't been threaded into the persistent card.

## Per-phase findings

### Phase 39 — Condition System (CRUD, duration, auto-expiration, indefinite, log) — Mostly OK
- Implementation: `internal/combat/condition.go` (CRUD, `isExpired`, `CheckExpiredConditions`, `ApplyCondition`, `RemoveConditionFromCombatant`, `ProcessTurnStart/End[WithLog]`).
- Indefinite handling: `condition.go:74` (`DurationRounds <= 0` never expires).
- Auto-expiration timing: `condition.go:82-85` defaults to `start_of_turn`; `:86` filters expiry on `expires_on`. Both timings supported.
- Source-bound expiry: `condition.go:78` enforces `SourceCombatantID == triggerCombatantID`. ✅
- Combat log: `formatConditionApplied` (`:296`), `RemoveConditionFromCombatant` log line (`:236`), expiry message (`:350`). ✅
- Hookups: `ProcessTurnStartWithLog`/`ProcessTurnEndWithLog` invoked from `initiative.go:388,656`. ✅
- Condition immunity at apply: `:149` calls `CheckConditionImmunity`. ✅
- Phase 118 incapacitation auto-breaks concentration: `:174-181` works on a stored condition. ✅
- Tests: 52 in `condition_test.go`.

### Phase 40 — Condition Effects on saves/checks/attacks/speed/action blocking — ⚠️ Partial integration
- Save effects: `condition_effects.go:22 CheckSaveConditionEffects` covers paralyzed/stunned/unconscious/petrified auto-fail STR/DEX, restrained DEX disadv, dodge DEX adv. ✅
- Check effects: `:67 CheckAbilityCheckEffects` handles blinded sight, deafened hearing, frightened (with FearSourceVisible), poisoned. ✅
- Speed effects: `:102 EffectiveSpeed` zeros for grappled/restrained. ✅
- Incapacitation/auto-skip: `:121 IsIncapacitated`, `auto_skip.go:13 FormatAutoSkipMessage`, used at `initiative.go:505-506` and `done_handler.go:399`. ✅
- Charmed attack restriction: `:143 IsCharmedBy` exists, but no callers in production code (`grep -rn IsCharmedBy internal/combat` shows the test plus the function only). ❌ Charmed actor can still target their charmer in `Attack()` — no enforcement.
- Frightened movement guard: `:155 ValidateFrightenedMovement` exists but `move_handler.go` does not call it. Verify: `grep ValidateFrightenedMovement internal/discord` returns nothing. ⚠️ Spec line 1207 calls for the prohibition; not enforced.
- Save/check helpers reach the check/save services: `internal/save/save.go:64` calls `CheckSaveWithExhaustion`, `internal/check/check.go:81` calls `CheckAbilityCheckWithExhaustion`. ✅
- Attack-side application: condition effects on attack rolls run through `internal/combat/advantage.go DetectAdvantage`. (Verified by `grep AttackerConditions advantage.go` matches.) ✅
- Tests: 73 in `condition_effects_test.go`.

### Phase 41 — Moving while Prone (Stand & Move vs Crawl) — ✅
- `internal/combat/movement.go:147 ValidateProneMoveStandAndMove` deducts `StandFromProneCost(maxSpeed)` then runs normal pathfinding.
- `:180 ValidateProneMoveCrawl` reuses the prone-aware A* (double cost) and stacks with difficult-terrain via the pathfinding cost map (`pathfinding.go` IsProne handling).
- Discord wiring: `move_handler.go:207-226` shows the [Stand & Move] / [Crawl] buttons when `isProne && !turn.HasStoodThisTurn`. `:574,624` invoke the validators. `:666` sets `HasStoodThisTurn = true` after Stand & Move. ✅
- Tests: 28 in `movement_test.go` including stand+move, crawl, stand-cost-vs-remaining edges.

### Phase 42 — Damage Processing (R/I/V, Temp HP, Exhaustion, Cond Immunity) — ❌ Partial: helpers exist but unused
- `damage.go:13 ApplyDamageResistances` correctly implements I trumps all → R+V cancel → R alone → V alone → petrified-as-resistance. **Unused outside tests.** ❌
- `damage.go:49 AbsorbTempHP`, `:57 GrantTempHP` correct semantics. **Unused outside tests.** ❌
- Exhaustion ladder: `damage.go:66 ExhaustionEffectiveSpeed` (level 2 halves, 5+ zeros), `:78 ExhaustionEffectiveMaxHP` (level 4+ halves), `:88 ExhaustionRollEffect` (level 1+ check disadv, 3+ attack/save disadv), `:99 IsExhaustedToDeath` (6 = death). Only the speed-and-saves slice is wired (via `condition_effects.go:202-207, 212-224, 228-240` and `save/save.go:64`, `check/check.go:81`). Max-HP halving (level 4+) and death (level 6) are never applied — no caller. ❌
- Condition immunity: `:104 CheckConditionImmunity` is called from `condition.go:149 ApplyCondition`. ✅
- Tests: 67 in `damage_test.go` (cover every R/I/V interaction, all six exhaustion levels, temp-HP edge cases).
- Net: the R/V/temp-HP/HP-halving slices of Phase 42 are essentially dead code in production. Any damage applied via `applyDamageHP` (concentration.go:281) skips these helpers entirely. This is the highest-value follow-up in this chunk.

### Phase 43 — Death Saves & Unconsciousness — ⚠️ State machine present, command not wired, partial integration
- Full state machine in `internal/combat/deathsave.go`: `CheckInstantDeath` (`:50`), `ProcessDropToZeroHP` (`:57`), `RollDeathSave` (`:79` — handles nat 20 → 1 HP, nat 1 → 2 fails, ≥10 success, <10 fail), `ApplyDamageAtZeroHP` (`:150`, +2 fails on crit), `HealFromZeroHP` (`:187`), `StabilizeTarget` (`:200`, sets 3 successes), `GetTokenState` (`:213`, alive/dying/dead/stable), `ConditionsForDying` (`:236`, applies unconscious + prone). ✅
- Wiring of `RollDeathSave`: only `internal/combat/timer_resolution.go:151` (auto-roll on timer expiry). The `/deathsave` slash command is unwired (`router.go:201` lists `deathsave` but no `SetDeathsaveHandler` exists). ❌
- `ProcessDropToZeroHP` is **not** called from any HP-update path. `applyDamageHP` (`concentration.go:281`) handles the unconscious-at-0 hook for concentration but does not emit the death-save begin message, nor does it apply `ConditionsForDying` (unconscious + prone) automatically. Tests `concentration_integration_test.go` confirm only the concentration save fires. ❌
- `ApplyDamageAtZeroHP` (the +1/+2-fail-per-hit-while-down rule) — never called. ❌
- Stabilization via Medicine DC 10 / Spare the Dying: `StabilizeTarget` defined; `spare-the-dying` cantrip seeded (`internal/refdata/seed_spells_cantrips.go:27`). No service method consumes either. ❌
- Tests: 47 in `deathsave_test.go` (cover nat-1, nat-20, 3-success path, 3-fail death, healing reset).

### Phase 44 — Feature Effect System Core — ✅ Engine; ⚠️ Some EffectTypes only weakly handled
- `internal/combat/effect.go` defines all 19 EffectTypes (`:13-32`), 8 TriggerPoints (`:53-61`), `EffectConditions` filter struct (`:76-94`), `Effect`/`FeatureDefinition` (`:97-119`), 5-tier resolution priority (`:126-131`), `EvaluateConditions` (`:190`), `CollectEffects` (`:277`), `SortByPriority` (`:299`), single-pass `ProcessEffects` (`:307-391`).
- Priority ordering Immunity → Resistance → Flat → Dice → Adv — implemented via `EffectPriority` mapping (`:134`). ✅
- Filters supported: when_raging, when_concentrating, weapon properties (single + AND-OR list), attack_type, ability_used, target_condition, ally_within, advantage_or_ally_within, has_advantage, wearing_armor, not_wearing_armor, one_handed_melee_only, once_per_turn, uses_remaining (`:190-244`). ✅
- Tests: 39 in `effect_test.go` covering each type, priority, multi-effect aggregation.
- Caveat: `EffectAura`, `EffectDMResolution`, `EffectReplaceRoll`, `EffectGrantProficiency` collect into the result (`:379-385`) but no caller actually consumes `result.AuraEffects`/`DMResolutions`/`ReplacedRoll`/`Proficiencies` outside of unit tests. Spec lines 1521-1528 require these to be functional pathways; they are stubs at the integration layer.

### Phase 45 — FES Class-Feature Integration — ❌ Definitions present, attack pipeline never sees them
- Definitions in `internal/combat/feature_integration.go`: `SneakAttackFeature` (`:80`), `EvasionFeature` (`:101`)+`ApplyEvasion` (`:121`), `UncannyDodgeFeature` (`:130`)+`ApplyUncannyDodge` (`:145`), `ArcheryFeature` (`:151`), `DefenseFeature` (`:170`), `DuelingFeature` (`:189`), `GreatWeaponFightingFeature` (`:209`)+`ApplyGreatWeaponFighting` (`:229`), `PackTacticsFeature` (`:243`).
- `BuildFeatureDefinitions` (`:264`) maps mechanical_effect strings to definitions. ✅ in isolation.
- **Critical:** `attack.go:456` reads `input.Features`, but no caller ever assigns it. `BuildFeatureDefinitions` is referenced only from tests + `internal/magicitem/integration_test.go:183`. ❌
- `BuildAttackEffectContext` is invoked at `attack.go:457` with only `Weapon` populated — `HasAdvantage/AllyWithinFt/WearingArmor/OneHandedMeleeOnly/IsRaging/AbilityUsed/UsedThisTurn` are all zero. Even if Features were wired, Sneak Attack's `AdvantageOrAllyWithin: 5` filter would never pass in production. ❌
- `ApplyEvasion`, `ApplyUncannyDodge` have no callers in production code. The reaction system (`reaction.go`) is purely declaration-based — no Uncanny Dodge auto-prompt. ❌
- `ApplyGreatWeaponFighting` has no callers (the damage-roll path in `attack.go` uses `dice.Roller` directly with no reroll-on-1s-and-2s hook). ❌
- TWF fighting style is hardcoded at `attack.go:966 HasFightingStyle("two_weapon_fighting")` — bypasses the FES, so consistent but bypasses the data-driven contract.
- Tests: 47 in `feature_integration_test.go` validate the FES *standalone*, but there are no `Attack()` integration tests proving the features fire end-to-end.

### Phase 46 — Rage (Barbarian) — ⚠️ Service complete, FES integration broken, missing rules
- `internal/combat/rage.go`: `RageDamageBonus` (`:12`), `RageUsesPerDay` (`:24`), `RageFeature` (`:44`, declares 4 effects: damage bonus, B/P/S resistance, STR check adv, STR save adv), `ValidateRageActivation` (`:96`, blocks heavy armor), `ActivateRage` (`:196`), `EndRage` (`:286`), `DecrementRageRound` (`:157`), `ShouldRageEndOnTurnEnd` (`:141`, no-attack-no-damage rule), `ShouldRageEndOnTurnStart` (`:149`), `ShouldRageEndOnUnconscious` (`:166`).
- 10-round duration: `RageRounds = 10` (`:92`). ✅
- Unlimited at level 20: `:24`. ✅
- Rage damage and resistance only fire if `IsRaging` and `WhenRaging` are matched in the FES. Since `Attack()` never sets `Features` or `IsRaging` in `EffectContext`, the damage bonus and resistance **never apply in real combat**. ❌
- `/cast` block while raging: spec 1414+ requires "block /cast and drop concentration while raging." `grep IsRaging spellcasting.go concentration.go` returns nothing. ❌ Not enforced.
- Heavy-armor restriction in `ValidateRageActivation`: only checked at activation. If the player equips heavy armor *while* raging, no auto-end. ⚠️ minor.
- Slash command wiring: `/bonus rage` is a stub (router.go status-aware). ❌ Not callable.
- Auto-end conditions on turn-end (no attack, no damage) — there's no turn-end hook calling `ShouldRageEndOnTurnEnd`. `grep ShouldRageEndOnTurnEnd internal` shows definition + tests only. ❌
- Tests: 43 in `rage_test.go`.

### Phase 47 — Wild Shape (Druid) — ⚠️ Service complete, not wired
- `internal/combat/wildshape.go`: `WildShapeCRLimit` standard + Moon (`:19`), `SnapshotCombatantState` (`:46`), `ApplyBeastFormToCombatant` (`:63`), `RevertWildShape` w/ overflow (`:75`), `ValidateWildShapeActivation` (CR cap, beast-only, fly/swim level gates, `:102`), `CanWildShapeSpellcast` (`:145`, level 18+ only), `ActivateWildShape` (`:229`), `RevertWildShapeService` (`:341`), `AutoRevertWildShape` (`:173`).
- Stat swap: HP/AC/IsWildShaped/WildShapeCreatureRef updated. Speed snapshotted (`:46`) but the new beast walk speed is reported (`:312 getBeastWalkSpeed`) — combatant.SpeedFt isn't on the schema as a column, so speed restoration depends on the snapshot. ⚠️ Worth verifying speed actually applies in movement code.
- Concentration maintained: nothing in code drops concentration on activation. ✅ implicit.
- Spellcasting block: `CanWildShapeSpellcast(:145)` is defined but no caller. `internal/combat/spellcasting.go` does not check `IsWildShaped`. ❌ A wild-shaped Druid can still cast (level <18).
- HP overflow handling: `RevertWildShape(:75)` subtracts overflow from snapshot HP correctly.
- Auto-revert at 0 HP: `AutoRevertWildShape` exists but isn't called from `applyDamageHP`. ❌
- Slash command `/bonus wild-shape`: stub. ❌
- Tests: 46 in `wildshape_test.go`.

### Phase 48a — Monk Martial Arts & Unarmored Defense/Movement — ⚠️ Half-integrated
- Martial Arts die: `MartialArtsDie/MartialArtsDieSides` (`monk.go:492-509`), `MonkDamageExpression` (`:514`) chooses higher of weapon vs MA die.
- DEX/STR auto-select for monk weapons: `MonkLevel` field in `AttackInput` and `IsMonkWeapon` (`:550`). `attack.go:856-858` populates `input.MonkLevel` from class. ✅
- Bonus unarmed strike post-Attack: `MartialArtsBonusAttack` (`:48`). Validation `:17` requires Attack action used.
- Unarmored Defense: handled in character AC-formula path, not visible in combat package. (Out of chunk scope for verification.)
- `UnarmoredMovementFeature` (`:473`) — generated by `BuildFeatureDefinitions`, but since FES isn't wired into attack/turn, the speed bonus never fires. The /status panel doesn't show the bonus either. ❌
- Slash command `/bonus martial-arts` is a stub. ❌
- Tests: 98 in `monk_test.go`.

### Phase 48b — Monk Ki Abilities — ⚠️ Service complete, commands stubbed
- `FlurryOfBlows` (`monk.go:109`), `PatientDefense` (`:230`, applies dodge condition), `StepOfTheWind` (`:411`, dash adds movement, disengage flag), `StunningStrike` (`:336`, CON save → stunned 1 round). All deduct 1 ki, validate Monk class, validate ki ≥ 1.
- Stunning Strike DC: `StunningStrikeDC = 8 + prof + WIS mod` (`:268`). ✅
- `deductKi` (`:184`) reuses `FeatureKeyKi`.
- Stunning Strike auto-prompt after melee hit — Discord-side prompt is missing. The service only fires when called. Spec 707-755 expects an ephemeral prompt after hit; no prompt path. ❌
- Slash commands all stubbed. ❌

### Phase 49 — Bardic Inspiration — ⚠️ Service complete, no usage prompt, no expiry sweep
- `bardic_inspiration.go`: die scaling (`:16` d6→d12), max-uses by CHA mod ≥1 (`:31`), recharge-by-level (`:42 short` at 5+), validation (`:49`), `GrantBardicInspiration` (`:142`), `UseBardicInspiration` (`:230`).
- 10-minute real-time expiration: `:93 BardicInspirationExpirationDuration = 10*time.Minute`, `:95 IsBardicInspirationExpired`. **No caller sweeps expired inspirations** (`grep IsBardicInspirationExpired` returns just definition + test). ❌
- Turn-status visibility: integrated via `BuildResourceListWithInspiration` (`turnresources.go:293`) and called from `FormatTurnStartPrompt` and `FormatRemainingResources`. ✅
- 30-second usage prompt timeout: not implemented anywhere (`grep "30 \* time.Second\|30s" internal/combat/bardic*` returns nothing). The `UseBardicInspiration` service exists; the Discord ephemeral prompt + timeout is absent. ❌
- Slash command `/bonus bardic-inspiration` is stubbed. ❌

### Phase 50 — Channel Divinity — ⚠️ Five options implemented, no slash command, undead destruction wired
- `channel_divinity.go`: `ChannelDivinityMaxUses` (`:17` cleric and paladin), `DestroyUndeadCRThreshold` (`:46`, 0.5/1/2/3/4 by level), `SpellSaveDC` (`:86`).
- Turn Undead (`:158`): WIS save, 30ft range, applies "turned" condition (10 rounds). Destroy Undead destroys via `applyDamageHP` so concentration hooks still fire (`:269`). ✅
- Preserve Life (`:556`): half-max-HP cap, 30ft range, budget validation. ✅
- Sacred Weapon (`:356`): applies `sacred_weapon` condition for 10 rounds, but the +CHA-to-attack effect itself isn't wired into `Attack()`. ❌
- Vow of Enmity (`:436`): applies `vow_of_enmity` condition with 10ft range check. The advantage-on-attack effect isn't wired into `DetectAdvantage`. ❌
- DM-queue routing (`:508 ChannelDivinityDMQueue`): persists usage and posts a routing log line; doesn't actually post to dm-queue (no notifier dependency). ⚠️ Half-implemented.
- Slash command `/action channel-divinity` is stubbed. ❌

### Phase 51 — Divine Smite (Paladin) — ⚠️ Damage logic present, prompt/timeout missing
- `divine_smite.go`: `SmiteDiceCount` (`:19` 2d8→5d8 cap), `SmiteDamageFormula` (`:59`, +1d8 vs undead/fiend, doubles on crit), `IsSmiteEligible` (`:53` melee hit only), `DivineSmite` (`:162`, validates feat, slot, melee hit; consumes slot; rolls damage).
- `AvailableSmiteSlots` (`:30`) sorts available slot levels — would feed the Discord prompt buttons.
- `FormatSmiteCombatLog` (`:115`) builds the combat log line.
- Crit doubling: `SmiteDamageFormula` doubles dice when `isCrit`. ✅
- Undead/fiend +1d8: `:154 isUndeadOrFiend` looks up creature.type. ✅
- 30-second prompt timeout: not implemented (no Discord prompt at all). ❌
- `resource_on_hit` effect-type plumbing: spec 704 wants the prompt driven by FES; in code, `SmiteDamage` is invoked directly. The FES `EffectResourceOnHit` collects into `result.ResourceTriggers` (`effect.go:374`) but no consumer exists. ❌

### Phase 52 — Lay on Hands — ⚠️ Service complete, command stubbed
- `lay_on_hands.go`: pool 5×level (`:12 LayOnHandsPoolMax`), undead/construct rejection (`:63`), self-targeting skip on adjacency check (`:69`), 5ft range (`:71`), cure poison/disease via `RemoveConditionFromCombatant` (`:147,153`), 5 HP per cure cost (`:97-102`).
- Pool tracking via `feature_uses["lay-on-hands"]` (`:88-93`) using `DeductFeaturePool` (`feature_integration.go:49`).
- Slash command stubbed. ❌

### Phase 53 — Action Surge (Fighter) — ✅ Service correct; ⚠️ command stubbed
- `action_surge.go`: validates `ActionSurged` flag (`:26`), Fighter level 2+ (`:39`), uses-remaining (`:47`), deducts use, resets `ActionUsed=false` and `AttacksRemaining` to per-action count (`:56-58`), sets `ActionSurged=true`. ✅
- Per-turn double-surge prevention via `Turn.ActionSurged` (turn DB column wired in `turnresources.go:180`). ✅
- `resolveAttacksPerAction` (`turnresources.go:236`) honors multiclass. ✅
- Tests: 15 in `action_surge_test.go`.
- Slash command `/action surge` stubbed. ❌
- Reset on new turn: turn rows are created fresh per turn, so `ActionSurged` defaults false on a new turn — implicitly handled.

### Phase 54 — Standard Actions — ⚠️ All services implemented, none reachable
- `standard_actions.go` implements: Dash (`:40`, costs action, +base speed), Disengage (`:107`, sets HasDisengaged), Dodge (`:155`, applies dodge condition 1 round), Help (`:227`, 5ft adjacency, applies help_advantage), Hide (`:312`, stealth vs PP), Stand (`:583`, half movement, removes prone, sets HasStoodThisTurn), DropProne (`:647`, applies prone), Escape (`:705`, contested vs grappler, removes grappled).
- Cunning Action (`:816`): validates Rogue level 2+; supports dash/disengage/hide via bonus action. ✅
- All actions invoke `CanActRaw` (`:41,108,156…`) so incapacitation correctly blocks them. ✅
- Resource costs match spec: Stand and DropProne don't consume action; everything else uses an action; cunning-action uses bonus action. ✅
- All commands are routed to `StatusAwareStubHandler` ("not yet implemented"). ❌ Critical gap.

### Phase 55 — Opportunity Attacks — ❌ Detection only, never invoked
- `opportunity_attack.go`: `DetectOpportunityAttacks` (`:63`) walks the path, finds reach-exit tile per hostile, respects `HasDisengaged` (`:71`), respects `ReactionUsed` per-hostile turn (`:92`), supports NPC reach via `creatureAttacks` (`:131-141`).
- `findReachExit` (`:159`) requires the mover to start in reach (`:165`) and returns the last tile still in reach (`:175`).
- `FormatOAPrompt` (`:197`) builds the prompt text.
- **No call sites in production code** outside the file itself. `move_handler.go` does not invoke OA detection at any point. ❌
- Reach-weapon support: implementation reads max reach from a creature's attacks but PCs always use 5ft (`:142`), since there is no per-character `equipped_main_hand` reach lookup here. ⚠️ PC reach weapons (glaive, halberd, whip) won't extend OA reach.
- Queue-and-continue / ping-in-#your-turn / DM-dashboard prompt: none implemented (would need OA wiring). ❌
- "If OA kills target, DM handles retroactive correction" — moot since OA never fires. ❌

### Phase 56 — Grapple, Shove, Dragging — ⚠️ All services implemented, no slash commands; drag prompt not wired
- `grapple_shove.go`: `Grapple` (`:46`) checks free hand (`:55 checkFreeHand` rejects when both hands occupied), size limit (`:68 +1 max`), 5ft adjacency, contested STR vs higher of target Athletics/Acrobatics. ✅
- `Shove` (`:179`): same checks; for `--push` mode validates destination unoccupancy *before* the contest (`:213-233`). ✅
- `CheckDragTargets` (`:338`), `FormatDragPrompt` (`:363`), `DragMovementCost` (`:373` always ×2 regardless of count, matching spec), `ReleaseDrag` (`:384`).
- Slash commands `/action grapple`, `/shove`, drag-vs-release prompt — all stubbed. ❌
- Drag prompt not invoked from `move_handler.go` (no `CheckDragTargets` callers in production). ❌
- Tests: 29 in `grapple_shove_test.go`.

### Phase 57 — Stealth & Hiding — ⚠️ Service complete, /action hide stubbed
- `Hide` (`standard_actions.go:312`) → `resolveHide` (`:332`): rolls Stealth vs highest hostile passive Perception (`:344-356`), checks armor stealth disadvantage (`:413-422`), respects Medium Armor Master feat negation (`:418`), persists `IsVisible=false` on success.
- Passive Perception calc: `passivePerception` (`:492`) = 10 + skill mod (proficiency, expertise, JoaT for PCs; pre-baked or WIS for creatures). ✅
- Hidden-attacker advantage and auto-reveal-on-attack: handled in `attack.go:1070 AttackerHidden` + `:749` reveal. ✅ This part of Phase 57 IS actually integrated.
- Slash command `/action hide` and `/bonus cunning-action hide` stubbed. ❌
- Tests: covered in `standard_actions_test.go` (94 tests) and `attack_test.go`.

## Cross-cutting risks

### FES drift — engine/data divergence
`internal/combat/effect.go` defines a generous EffectType vocabulary, but `Attack()` and `applyDamageHP()` do not call `ProcessEffects` for triggers `on_take_damage`, `on_save`, `on_check`, `on_turn_start`, `on_turn_end`, `on_rest`. Effects of types `aura`, `dm_resolution`, `replace_roll`, `grant_proficiency`, `extra_attack` (beyond hardcoded), `modify_hp`, `modify_range`, `modify_speed` collect into `ProcessorResult` but no consumer reads them. As more class features are seeded into `classes.features_by_level` they will silently no-op.

### Damage-pipeline shortcut bypasses Phase 42 entirely
Every damage path (`internal/combat/aoe.go:554`, `channel_divinity.go:269`, `concentration.go:281`, `dm_dashboard_undo.go:189,369`, `dm_dashboard_handler.go:338`, `turn_builder_handler.go:291`) writes raw damage directly to `hp_current` via `applyDamageHP`. None route through `ApplyDamageResistances` or `AbsorbTempHP`. Fixing this is the single biggest correctness improvement available — it would re-enable Rage's BPS resistance, Heavy Armor Master, Tiefling fire resistance, monster damage immunities, and temp-HP absorption all at once.

### Slash-command stub gap
Of the 28 game commands in `router.go:198`, only 15 have real handlers. The 13 stubs include the *primary mechanical actions of every spell/feature in this chunk*: `attack`, `cast`, `bonus`, `shove`, `interact`, `deathsave`, `command` (resolved), `equip` (resolved actually — see `SetEquipHandler:132`), `inventory` (resolved), `give` (resolved), `attune`/`unattune` (resolved), `prepare`, `retire`, `undo`, `character`, `help`, `loot` (resolved). Recheck reveals the actual hot list still stubbed: `/attack`, `/cast`, `/bonus`, `/shove`, `/deathsave`, `/interact`, `/undo`, `/prepare`, `/retire`. That means Phases 46–53, 55–57 are all unreachable from gameplay.

### Condition cleanup gaps
- **Charmed actor's attack restriction** (Phase 40) — `IsCharmedBy` exists but `Attack()` never consults it. Spec line 1207 explicitly bans the charmed creature from attacking the charmer. Not enforced.
- **Frightened approach restriction** — `ValidateFrightenedMovement` exists but `move_handler.go` does not call it. Net: a frightened creature can move toward its fear source.
- **Condition 1-round dodge expiry** — Dodge condition has `DurationRounds:1, ExpiresOn:start_of_turn`. Initiative pipeline calls `ProcessTurnStart` (✅), so this expires correctly.
- **Help condition expiry** — Help applies `help_advantage` with `DurationRounds:1, ExpiresOn:start_of_turn`. Same treatment. But the *consumption-on-use* pattern (Help is supposed to expire after one attack OR on next turn, whichever comes first) is not enforced — there is no check-and-clear when the helped ally attacks. ⚠️
- **Sacred Weapon / Vow of Enmity conditions** — applied for 10 rounds, but the actual mechanical effect (extra attack mod / advantage) is not consulted in `Attack()` since FES isn't wired. So the conditions exist on the combatant but do nothing.

### Missing class-feature combinations
- Monk's `bonus_action_unarmed_strike` is consumed (`feature_integration.go:303`) via comment "handled by MartialArtsBonusAttack"; that's fine — but for Monk + Rogue multiclass, Sneak Attack damage should fire on the Monk's bonus unarmed strike (it's a melee strike). Since Sneak Attack via FES doesn't fire at all, this is moot for now, but worth tracking once #1 is fixed.
- Druid in Wild Shape: spell-block check missing (`CanWildShapeSpellcast` unused). Also rage + wild-shape combination (Barbarian/Druid multiclass) — both modify HP; auto-revert overflow logic is correct in isolation but never tested in combination.
- Paladin Smite + crit + undead — `SmiteDamageFormula(:59)` correctly stacks `+1d8 undead` then `*2 on crit`. Spec 700 says "all smite dice are doubled" on crit; current code doubles total `count` after the +1d8. That matches spec. ✅
- Action Surge while raging — Action Surge resets `ActionUsed`+`AttacksRemaining` but does not re-check rage's per-round `RageAttackedThisRound` flag. If the Fighter/Barbarian uses Surge to take a second action in a round but doesn't attack, would rage end mid-turn? `ShouldRageEndOnTurnEnd` checks at end-of-turn so this is not a concrete bug, but the per-round attack tracking is implicit.

## Recommended follow-ups

1. **Wire `Features` and the `EffectContext` through `Attack()`.** Inside `s.Attack()` (`attack.go:773`), build `BuildFeatureDefinitions(classes, charFeatures, magicItemDefs)` and assign `input.Features = ...`. Populate `BuildAttackEffectContext` with the real `IsRaging` (from combatant), `HasAdvantage` (after `DetectAdvantage`), `AllyWithinFt` (compute from grid), `WearingArmor` (from char.EquippedArmor), `OneHandedMeleeOnly`, `AbilityUsed` (str/dex), and `UsedThisTurn` (from per-turn tracking). After this, Sneak Attack, Rage damage, fighting styles, Pack Tactics start to work. (Phases 45, 46.)

2. **Route every damage write through a new `ApplyDamage(...)` helper that calls `ApplyDamageResistances` → `AbsorbTempHP` before `applyDamageHP`.** Replace each direct `applyDamageHP` call site (8 production locations) with the new helper, supplying the target's resistances/immunities/vulnerabilities/conditions. Add exhaustion HP-halving and level-6 death. (Phase 42.)

3. **Wire opportunity attacks into `move_handler.go`.** After `ValidateMove` returns a path, call `DetectOpportunityAttacks(mover, path, allCombatants, moverTurn, hostileTurns, creatureAttacks)`. For each trigger: post `FormatOAPrompt` to the hostile player's `#your-turn` channel (or DM dashboard for NPCs), record a pending reaction, mark reaction_used on consumption. Add reach-weapon resolution for PCs. (Phase 55.)

4. **Add real handlers for the stubbed game commands.** Build `BonusHandler`, `AttackHandler`, `CastHandler`, `ShoveHandler`, `DeathSaveHandler`, `InteractHandler`, `UndoHandler`, `PrepareHandler`, `RetireHandler`, plus extend `ActionHandler` to dispatch the structured sub-actions (dash/disengage/dodge/help/hide/stand/drop-prone/escape/grapple/lay-on-hands/surge/channel-divinity) before falling through to freeform. Add the corresponding `Set*Handler` methods on `CommandRouter`. (Phases 46–53, 55–57.)

5. **Wire the auto-prompt features.** Stunning Strike post-melee-hit prompt, Divine Smite post-melee-hit prompt with available slot buttons, Uncanny Dodge incoming-damage prompt, Bardic Inspiration use-on-roll prompt with 30-second timeout. Drive Smite/Stunning Strike from FES `resource_on_hit` collection (`ProcessorResult.ResourceTriggers`), which already collects them. (Phases 48b, 49, 51, 45.)

6. **Populate Conditions/Concentration/Exhaustion in character cards.** Fix `internal/charactercard/service.go:203 buildCardData` to read combatant-side state (will require the combatant lookup or moving these fields onto the character, not the combatant). Update the related portion of MEMORY.md once done. (Memory note: project_character_card_deferred_fields.md.)

7. **Add `/cast` rage block, frightened movement check, charmed-attack restriction, expired Bardic Inspiration sweep.**
   - In a future `CastSpell` implementation, return error if `IsRaging`. Drop concentration on `ActivateRage` (Phase 46).
   - In `move_handler.go`, call `ValidateFrightenedMovement` before computing the path (Phase 40).
   - In `Attack()`, call `IsCharmedBy(target.ID)` and reject (Phase 40).
   - Add a periodic sweep (or check at use-time) of `BardicInspirationGrantedAt + 10min` (Phase 49).

8. **Rage end-of-turn auto-end.** Add a turn-end hook calling `ShouldRageEndOnTurnEnd`. Currently the duration counter expires at 10 rounds via `DecrementRageRound` — but the no-attack-no-damage exit is unreachable. (Phase 46.)

9. **Wild Shape spellcasting block + auto-revert.** `CanWildShapeSpellcast` should be checked from `CastSpell`. `AutoRevertWildShape` should be invoked from `applyDamageHP` when `IsWildShaped && hpCurrent <= 0`. (Phase 47.)

10. **Help condition consume-on-use.** When `Attack()` sees a `help_advantage` condition where the helper matches the help source, consume it after the attack roll resolves. Currently it just sits until next turn. (Phase 54.)
