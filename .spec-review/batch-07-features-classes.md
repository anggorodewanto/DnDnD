# Batch 07: Feature Effect System + class features (Phases 44–53)

## Summary

The Feature Effect System (Phase 44) is built out comprehensively: all 19 effect
types and all 8 spec trigger points are declared, conditions are filtered, and
a single-pass priority-sorted processor produces a `ProcessorResult`. Class
features (Rage, Wild Shape, Monk Ki, Bardic Inspiration, Channel Divinity,
Divine Smite, Lay on Hands, Action Surge) all ship with service implementations
and Discord wiring under `/bonus`, `/action`, and post-hit prompts. The Feature
Effect System integration into the attack pipeline (Phase 45) covers Sneak
Attack, fighting styles, Pack Tactics, Uncanny Dodge, and Evasion.

Three concerns stand out: (1) two **incompatible serialization shapes** for
`feature_uses` are in use — combat reads flat `{"rage": 3}` ints while rest
reads structured `{current, max, recharge}`; this will break either rest
recharge of the combat-written features or vice-versa. (2) `UsedThisTurn` is
never populated from persisted turn state in `populateAttackFES`, so Sneak
Attack's `once_per_turn` filter relies on test-only wiring at runtime. (3)
`grant_resistance` is collected but no consumer path applies the resistances
list to the damage pipeline — see `ProcessorResult.Resistances` callers (none
in damage.go).

## Per-phase findings

### Phase 44 — Feature Effect System Core Engine
- Status: Matches.
- Key files: `internal/combat/effect.go`, `internal/combat/effect_test.go`.
- Findings:
  - All 19 effect types declared (`EffectModifyAttackRoll`, …, `EffectDMResolution`); `IsValid()` covers each.
  - All 8 trigger points declared (`on_attack_roll`, `on_damage_roll`, `on_take_damage`, `on_save`, `on_check`, `on_turn_start`, `on_turn_end`, `on_rest`).
  - `EffectConditions` covers `WhenRaging`, `WhenConcentrating`, `WeaponProperty(ies)`, `AttackType`, `AbilityUsed`, `TargetCondition`, `AllyWithin`, `UsesRemaining`, `OncePerTurn`, `HasAdvantage`, `AdvantageOrAllyWithin`, plus armor/style flags.
  - `ResolutionPriority` enforces immunities → R/V → flat → dice → adv/disadv (`EffectPriority`).
  - `ProcessEffects` is the single-pass processor: `CollectEffects` → `SortByPriority` → switch dispatch. Advantage/disadvantage cancellation is delegated to `resolveMode`.
  - Divergence (minor): `EffectAura` and `EffectDMResolution` are collected into `AuraEffects` / `DMResolutions` but have no production consumer (comment notes this). Spec calls these "Reserved API" only in the implementation, not the spec.

### Phase 45 — Feature Effect System ↔ Class Feature Integration
- Status: Mostly matches with one runtime gap.
- Key files: `internal/combat/feature_integration.go`, `internal/combat/attack.go` (`populateAttackFES`, `ResolveAttack`), `internal/combat/feature_integration_test.go`.
- Findings:
  - Sneak Attack: `SneakAttackFeature` declares `extra_damage_dice` with `WeaponProperties:[finesse, ranged]`, `AdvantageOrAllyWithin:5`, `OncePerTurn:true`. Dice = `(rogueLevel+1)/2` d6.
  - Evasion: `EvasionFeature` + `ApplyEvasion(damage, saveSuccess)` returns 0 on success, `damage/2` on fail.
  - Uncanny Dodge: `UncannyDodgeFeature` declares a `reaction_trigger`; the Discord layer (`class_feature_prompt.go`) posts a Halve/Skip prompt, and the attack pipeline carries `UncannyDodge`-eligible flags. (Phase 45 done condition mentions Rogue 5+; level gate is checked in prompt service path.)
  - Fighting Styles: `ArcheryFeature` (+2 ranged), `DefenseFeature` (+1 AC, `WearingArmor:true`), `DuelingFeature` (+2 melee, `OneHandedMeleeOnly:true`), `GreatWeaponFightingFeature` (`EffectReplaceRoll` + `ApplyGreatWeaponFighting` rerolls 1s & 2s).
  - Pack Tactics: `PackTacticsFeature` declares conditional advantage when `AllyWithin:5`.
  - Divergence: `populateAttackFES` does not set `AttackInput.UsedThisTurn`. Only tests populate it. Result: in production, Sneak Attack's `OncePerTurn` filter passes every attack in the same turn, allowing repeated SA bonus dice. The combat service does not appear to persist a per-turn "sneak attack used" flag anywhere.

### Phase 46 — Rage (Barbarian)
- Status: Matches; thorough.
- Key files: `internal/combat/rage.go`, `internal/combat/rage_test.go`, `internal/discord/bonus_handler.go` (`case "rage"` / `case "end-rage"`).
- Findings:
  - `RageFeature` declares `modify_damage_roll` (+2/+3/+4 by level, melee+STR), `grant_resistance` for B/P/S, `conditional_advantage` on STR checks & saves.
  - `ApplyRageToCombatant` sets `IsRaging`, `RageRoundsRemaining=10`, and the two per-round trackers.
  - Auto-end: `ShouldRageEndOnTurnEnd` (no attack + no damage this round), `ShouldRageEndOnUnconscious` (HP <= 0), `ShouldRageEndOnTurnStart` (rounds expired), voluntary `EndRage`.
  - Heavy armor block via `ValidateRageActivation`.
  - Spellcasting block + concentration drop: `ActivateRage` calls `breakStoredConcentration(ctx, ragedCombatant, "raging")` (line 281). Concentration breaks correctly. The `/cast` block while raging is not in the rage code itself — caller would need to check `IsRaging` in the cast handler; this likely exists elsewhere (not verified in this batch).
  - Rage uses-per-day: `RageUsesPerDay` matches PHB (2/3/4/5/6/unlimited).
  - Divergence (data shape): writes to `feature_uses` as flat `int` map (see cross-cutting concerns).

### Phase 47 — Wild Shape (Druid)
- Status: Matches with one minor gap.
- Key files: `internal/combat/wildshape.go`, `internal/combat/wildshape_test.go`, `internal/discord/bonus_handler.go` (`wild-shape`, `revert-wild-shape`).
- Findings:
  - CR limit: `WildShapeCRLimit` covers standard (1/4 → 1/2 → 1) and Circle of the Moon (1 at lvl 2, `level/3` at 6+).
  - Stat swap: `ApplyBeastFormToCombatant` sets HP/HP max/AC and stores beast ref; `SnapshotCombatantState` snapshots HP/AC/speed/ability scores; `RevertWildShape` restores and applies overflow damage.
  - Retained INT/WIS/CHA: not enforced explicitly — the snapshot stores all 6 ability scores and the beast HP/AC/STR/DEX/CON path is via `ApplyBeastFormToCombatant`. However, `ApplyBeastFormToCombatant` does NOT actually overwrite STR/DEX/CON on the combatant — those live on the character row. This means physical ability checks during Wild Shape are not yet swapped to beast values. Mental retention is therefore correct by default; physical replacement is not.
  - Spellcasting block (except `Beast Spells` at 18+): `CanWildShapeSpellcast(druidLevel)` is defined but no caller checks it inside `/cast`. Need to verify in spellcasting handler (out of scope).
  - Concentration maintenance: not explicitly handled — the spec says concentration is maintained through Wild Shape but I see no concentration-preservation logic in `wildshape.go`.
  - Auto-revert at 0 HP: `AutoRevertWildShape` wraps `RevertWildShape` with overflow. Wired into damage pipeline? Not verified here.
  - Voluntary revert costs a bonus action (matches spec).
  - Token change: no token-side change found in `wildshape.go`.

### Phase 48a — Monk Martial Arts & Unarmored Defense/Movement
- Status: Matches.
- Key files: `internal/combat/monk.go`, `internal/discord/bonus_handler.go` (`martial-arts`).
- Findings:
  - Martial Arts die scaling: `MartialArtsDieSides` = 4/6/8/10 by level (1-4/5-10/11-16/17+).
  - DEX/STR auto-select for monk weapons + unarmed: `attackAbilityUsed(scores, weapon, monkLevel)` (in attack.go) — verified called when `MonkLevel > 0`.
  - `MonkDamageExpression` picks max of weapon die vs. martial arts die for monk weapons; always martial arts die for unarmed.
  - Bonus unarmed strike: `MartialArtsBonusAttack` requires Attack action used this turn and costs a bonus action.
  - Unarmored Defense: spec mentions `10 + DEX + WIS`. The monk file does not declare a `modify_ac` effect for this — verify in `internal/character/stats.go` (out of immediate scope) — AC formula for monks is typically computed at character-sheet level, not in combat.
  - Unarmored Movement: `UnarmoredMovementFeature` declares `modify_speed` (+10/15/20/25/30 by level) with `NotWearingArmor:true`. Speed application happens at `on_turn_start` and the processor returns `SpeedModifier`.
  - `IsMonkWeapon` correctly excludes heavy/two-handed simple melee weapons.

### Phase 48b — Monk Ki Abilities
- Status: Matches.
- Key files: `internal/combat/monk.go`, `internal/discord/bonus_handler.go` (`flurry`, `step-of-the-wind`, `patient-defense`), `internal/combat/class_feature_prompt.go` (StunningStrike prompt).
- Findings:
  - `deductKi` + `spendKi` validate monk class + ki remaining, deduct 1 ki, use bonus action.
  - Flurry of Blows: requires Attack action used; 2 unarmed strikes (`UnarmedStrike()`); deducts 1 ki + bonus action.
  - Patient Defense: applies `dodge` condition with `ExpiresOn:start_of_turn`.
  - Step of the Wind: `dash` doubles `MovementRemainingFt`; `disengage` sets `HasDisengaged=true`. Mode validation rejects other values.
  - Stunning Strike: DC = 8 + prof + WIS mod; CON save; on fail applies `stunned` condition `ExpiresOn:end_of_turn` for 1 round. Wired to a post-hit prompt via `PromptStunningStrike` after melee hit.
  - Ki tracking: stored under `feature_uses["ki"]` (`FeatureKeyKi`).

### Phase 49 — Bardic Inspiration
- Status: Matches.
- Key files: `internal/combat/bardic_inspiration.go`, `internal/discord/bonus_handler.go` (`bardic-inspiration`), `internal/discord/class_feature_prompt.go` (`PromptBardicInspiration`), `internal/combat/attack.go` (post-hit eligibility flags).
- Findings:
  - Dice scaling: `BardicInspirationDie` = d6/d8/d10/d12 at 1-4/5-9/10-14/15+.
  - Max uses: `BardicInspirationMaxUses(cha)` = max(1, CHA mod).
  - Recharge: `BardicInspirationRechargeType` returns `"short"` at level 5+ (Font of Inspiration), else `"long"`.
  - Grant rejects self-target and target with active die.
  - 10-minute real-time expiration: `BardicInspirationExpirationDuration = 10 * time.Minute`; `sweepExpiredBardicInspirations` walks the encounter and clears stale grants.
  - 30s prompt timeout: handled by the `ReactionPromptStore` (binary post helper). Test: `TestClassFeaturePromptPoster_BardicInspiration_30sTimeout`.
  - Single-use: `ClearBardicInspirationFromCombatant` invoked on use.
  - Combat log lines match spec emoji/format.
  - Turn status display: `FormatBardicInspirationStatus(die)` exists but caller not searched.

### Phase 50 — Channel Divinity (Cleric / Paladin)
- Status: Matches.
- Key files: `internal/combat/channel_divinity.go`, `internal/discord/action_handler.go` (`dispatchChannelDivinity`), `internal/combat/channel_divinity_integration_test.go`.
- Findings:
  - Uses progression: `ChannelDivinityMaxUses` returns 1/2/3 for Cleric (lvl 2/6/18) and 1/2 for Paladin (lvl 3/15). Matches spec.
  - Turn Undead: scans creatures within 30ft, undead type filter, WIS save vs DC = 8 + prof + WIS mod. On fail: Turned (10 rounds, `ExpiresOn:end_of_turn`).
  - Destroy Undead: `DestroyUndeadCRThreshold` = 1/2/1/2/3/4 at lvl 5/8/11/14/17. On destroy, applies full HP damage via `ApplyDamage(Override=true)` to skip R/I/V — matches spec intent.
  - Preserve Life (Life Domain): validates 5×level budget, targets within 30ft, not already above half max HP, no over-heal past half max. Applies HP and triggers `MaybeResetDeathSavesOnHeal`.
  - Sacred Weapon (Devotion Paladin): applies `sacred_weapon` condition for 10 rounds, returns `CHAModifier` for surfacing in attack rolls. The actual `modify_attack_roll` effect from condition is not declared in `sacred_weapon` mechanical_effects (would need a `conditions_ref` row). Likely surfaced through condition_effects.go — not verified here.
  - Vow of Enmity (Vengeance Paladin): 10ft range check, applies `vow_of_enmity` to target for 10 rounds.
  - DM-resolved options: `ChannelDivinityDMQueue` deducts use + action and emits a log directing DM, but I see no actual write to a `dm_queue_items` row. (`#dm-queue` posting may live in a separate DM-queue poster — not verified.)
  - Divergence: `ExpiresOn:end_of_turn` for a 10-round (1-minute) effect is suspicious — turning conditions on combatants generally expire after `DurationRounds` rounds, but the spec language "ends early if the creature takes damage" for Turned is not implemented here (no hook in damage pipeline clears the `turned` condition on damage).

### Phase 51 — Divine Smite (Paladin)
- Status: Matches.
- Key files: `internal/combat/divine_smite.go`, `internal/discord/attack_handler.go` (post-hit dispatch), `internal/discord/class_feature_prompt.go` (`PromptDivineSmite`).
- Findings:
  - Eligibility: `IsSmiteEligible` requires `Hit && IsMelee`.
  - Dice formula: `SmiteDiceCount(slot)` = `min(1+slot, 5)`, so 2/3/4/5/5 at 1st–5th slot. +1d8 vs undead/fiend (`isUndeadOrFiend`). Crit doubles total dice (matches spec).
  - Slot selection prompt: `PromptDivineSmite` posts one button per available slot level + Skip; 30s timeout managed by `ReactionPromptStore`.
  - `AvailableSmiteSlots` filters slots with `Current > 0` and returns sorted ascending. Wired into `result.PromptDivineSmiteSlots`.
  - Slot deduction persists via `UpdateCharacterSpellSlots` after `Roll(diceStr)`.
  - Combat log via `FormatSmiteCombatLog` differentiates crit/undead/both.

### Phase 52 — Lay on Hands (Paladin)
- Status: Matches.
- Key files: `internal/combat/lay_on_hands.go`, `internal/discord/action_handler.go` (`dispatchLayOnHands`).
- Findings:
  - Pool: `LayOnHandsPoolMax = 5 * paladinLevel`. Pool tracked under `feature_uses["lay-on-hands"]`.
  - Adjacency: skipped for self-target (`cmd.Paladin.ID == cmd.Target.ID`), else `combatantDistance ≤ 5`.
  - Undead/construct rejection via `isUndeadOrConstruct`.
  - Cure poison/disease: each costs 5 HP from pool; flags `CurePoison` / `CureDisease`; only removes condition if present (`HasCondition`).
  - Death-save reset on heal via `MaybeResetDeathSavesOnHeal`.
  - Combat log emits all three line variants (heal, cure poison, cure disease) per spec.

### Phase 53 — Action Surge (Fighter)
- Status: Matches.
- Key files: `internal/combat/action_surge.go`, `internal/discord/action_handler.go` (`dispatchActionSurge`).
- Findings:
  - Double-surge prevention: `cmd.Turn.ActionSurged` flag persisted on turn row.
  - Fighter level 2+ gate via `ClassLevelFromJSON(classes, "Fighter")`.
  - Resets `ActionUsed=false` and `AttacksRemaining = resolveAttacksPerAction(char)` (covers Extra Attack scaling at 5/11/20).
  - Use tracked under `feature_uses["action-surge"]` — but only level-2-grants-1 is enforced upstream when feature_uses is seeded; the spec calls for 2 uses at level 17. The level-17 progression must come from class seeding, not from this code; the implementation handles whatever the seed says.
  - Combat log line matches spec.

## Cross-cutting concerns

1. **`feature_uses` schema mismatch.** `combat.ParseFeatureUses` unmarshals to
   `map[string]int` (e.g. tests use `{"rage": 3}`). `internal/character/types.go`
   defines `FeatureUse{Current, Max, Recharge}` and the rest service reads this
   structured form to recharge on short/long rest. These two formats are
   mutually unparseable. If a character is created via the dashboard/character
   sheet (structured), combat's rage/ki/etc deductions will fail to unmarshal.
   If combat seeds the flat shape, rest recharge will fail to unmarshal and no
   recharge happens. Needs unification on the structured form, with
   `ParseFeatureUses` returning `Current` and an internal write helper that
   preserves `Max`/`Recharge`.

2. **Sneak Attack OncePerTurn enforcement.** `populateAttackFES` never sets
   `AttackInput.UsedThisTurn`. The `EffectConditions.OncePerTurn` filter only
   trips when the caller threads in a non-nil map. Result: in production every
   attack within a turn that qualifies for Sneak Attack adds sneak dice.

3. **`grant_resistance` consumer absent.** `ProcessorResult.Resistances` is
   populated for Rage and similar effects but the damage pipeline does not
   appear to consume it. Verify in `internal/combat/damage.go` — Rage's
   resistance to bludgeon/pierce/slash may not be applied. (Pending check;
   resistance handling may be on the combatant row directly.)

4. **Turned ends-on-damage.** Spec says Turned ends early when the creature
   takes damage. No hook in `damage.go` searches for and clears the `turned`
   condition. Affects Phase 50 fidelity.

5. **Wild Shape physical stat swap.** `ApplyBeastFormToCombatant` swaps HP/AC
   but does not write beast STR/DEX/CON to the combatant or character. Physical
   ability checks/saves during Wild Shape will still use the druid's scores.

6. **Channel Divinity DM-queue routing.** `ChannelDivinityDMQueue` emits a
   combat-log string but does not write a `dm_queue_items` row — verify the DM
   sees it in `#dm-queue` channel (likely depends on a separate poster path).

7. **`/cast` block while raging / wild-shaped.** Rage drops concentration and
   the comment in `ActivateRage` mentions a spellcasting block, but the actual
   block is presumed to live in the cast handler. Not verified.

## Critical items

- **C-1:** Unify `feature_uses` JSON shape across combat and rest. Currently
  divergent — a real character will fail one path or the other at runtime.
- **C-2:** Populate `AttackInput.UsedThisTurn` from persisted turn state in
  `populateAttackFES` (or persist a per-turn used-features set on the turn
  row). Without this, Sneak Attack's once-per-turn rule is unenforced.
- **C-3:** Verify Rage damage resistance is actually applied in the damage
  pipeline. `ProcessorResult.Resistances` has no apparent consumer.
- **C-4:** Implement "Turned ends on damage" hook in `damage.go`.
- **C-5:** Wild Shape physical-stat swap: either persist beast STR/DEX/CON on
  the combatant or compute them on demand in checks/saves.
