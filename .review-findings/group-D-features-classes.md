# Group D — Feature Effect System + Class Features (Phases 44–53)

Read-only correctness review against `docs/dnd-async-discord-spec.md` (Feature Effect System §1499–1640; Channel Divinity + Lay on Hands §744–848) and `docs/phases.md` §246–300.

## [Critical] Rage damage resistance never fires for seed-created barbarians
- **Location:** /home/ab/projects/DnDnD/internal/combat/feature_integration.go:347 and /home/ab/projects/DnDnD/internal/refdata/seed_classes.go:23
- **Spec/Phase ref:** spec §Feature Effect System example "Rage (Barbarian)"; Phase 46.
- **D&D rule:** Rage grants resistance to bludgeoning/piercing/slashing while raging.
- **Problem:** The `Rage` class feature is seeded with `mechanical_effect: "advantage_str_checks_saves,resistance_bludgeoning_piercing_slashing,bonus_rage_damage"`, but `BuildFeatureDefinitions` only emits a `RageFeature` for the literal token `"rage"`. The two never match, so a barbarian created from the standard class seeds gets `IsRaging=true` but no `RageFeature` enters the FES — resistance, +damage, and STR adv are all dropped. Tests use `MechanicalEffect: "rage"` (rage_test.go:235), masking the bug.
- **Suggested fix:** Either alias all three seed tokens (`resistance_bludgeoning_piercing_slashing`, `advantage_str_checks_saves`, `bonus_rage_damage`) to the `RageFeature` builder, or replace the seed `mechanical_effect` with `"rage"`. Add an integration test that runs `BuildFeatureDefinitions` over real seed features.

## [Critical] Feature uses never initialized at character creation
- **Location:** /home/ab/projects/DnDnD/internal/portal/builder_store_adapter.go:125 (CreateCharacterParams omits FeatureUses) and /home/ab/projects/DnDnD/internal/combat/feature_integration.go:64 (SetFeaturePool preserves Max+Recharge it never seeds)
- **Spec/Phase ref:** spec §Channel Divinity recharge "short"; Phase 46/48b/49/50/52/53 — every class feature uses `feature_uses`.
- **D&D rule:** Rage uses, ki, channel divinity, bardic inspiration, lay-on-hands pool, action surge all have per-rest pools.
- **Problem:** Neither the portal builder nor the dashboard charcreate path writes any `feature_uses` JSON when creating a character. `ParseFeatureUses` then returns `Current=0` for every key, so a freshly built barbarian fails with `no rage uses remaining (0/2)`, monks have 0 ki, paladins can't lay on hands or channel divinity, and fighters can't action surge. Worse, when something does eventually write a value through `SetFeaturePool`, it preserves the existing (empty) `Max` and `Recharge` fields — so even after the value is set manually, short/long rests can never recharge them (rest.go:225 / rest.go:402 gate on `Recharge == "short"`/`"long"`).
- **Suggested fix:** Populate `FeatureUses` in `BuilderStoreAdapter.CreateCharacterRecord` (and the dashboard charcreate adapter) by walking the character's classes/level and seeding `{Current, Max, Recharge}` for every limited-use feature (rage, ki, wild-shape, channel-divinity, bardic-inspiration, lay-on-hands, action-surge, second-wind, sorcery-points, …). Also have the activation paths self-heal `Max`/`Recharge` if missing.

## [Critical] Rage advantage on STR ability checks never wired
- **Location:** /home/ab/projects/DnDnD/internal/check/check.go (no FES integration) and /home/ab/projects/DnDnD/internal/combat/rage.go:71
- **Spec/Phase ref:** spec §Feature Effect System "Rage … conditional_advantage on str_check on_check"; Phase 46.
- **D&D rule:** While raging you have advantage on Strength checks (and STR saves).
- **Problem:** `RageFeature` emits a `TriggerOnCheck` conditional-advantage effect, but the check service never builds a `FeatureDefinition` list, never builds an `EffectContext`, and never calls `ProcessEffects` with `TriggerOnCheck`. A grep confirms no consumer of `TriggerOnCheck` anywhere in the repo. The result: a raging barbarian shoves an enemy with no rage advantage on the athletics check.
- **Suggested fix:** Mirror the save handler pattern — add `FeatureEffects` + `EffectContext` (with `IsRaging`, `AbilityUsed`) to `check.SingleCheckInput`, then call `combat.ProcessEffects(_, TriggerOnCheck, _)` and fold the resulting RollMode into the check's d20 mode.

## [Critical] Save handler never sets IsRaging in EffectContext
- **Location:** /home/ab/projects/DnDnD/internal/discord/save_handler.go:199
- **Spec/Phase ref:** spec §Feature Effect System example "Rage" trigger `on_save` with `when_raging`; Phase 46.
- **D&D rule:** While raging you have advantage on Strength saving throws.
- **Problem:** The `EffectContext` built for `/save` populates only `AbilityUsed` and `WearingArmor`. `IsRaging` is left as the zero value (`false`), so the rage save-advantage effect is filtered out by `EvaluateConditions` (`c.WhenRaging && !ctx.IsRaging`). A raging barbarian rolling a STR save (e.g. against a banishment) loses the rage advantage.
- **Suggested fix:** Look up the saver's active combatant via the existing `combatantLookup`, copy `IsRaging` (and ideally `IsConcentrating`) into the `EffectContext`. The check handler will need the same fix when the previous finding lands.

## [High] Step of the Wind dash adds remaining movement, not base speed
- **Location:** /home/ab/projects/DnDnD/internal/combat/monk.go:444
- **Spec/Phase ref:** Phase 48b; PHB Monk "Step of the Wind".
- **D&D rule:** Dash bonus action grants extra movement equal to your speed for the turn.
- **Problem:** `case "dash": updatedTurn.MovementRemainingFt += cmd.Turn.MovementRemainingFt` adds whatever is currently left, not the monk's speed. A monk who has already moved half their speed before invoking Step of the Wind gets only half of the dash bonus they are owed. The standard `Service.Dash` (standard_actions.go:48) correctly uses `resolveBaseSpeed(...)`.
- **Suggested fix:** Replace the doubling with `speed, _ := s.resolveBaseSpeed(ctx, cmd.Combatant); updatedTurn.MovementRemainingFt += speed`.

## [High] Dodge condition grants no defensive disadvantage to attackers
- **Location:** /home/ab/projects/DnDnD/internal/combat/advantage.go:104 (switch on `c.Condition` for target)
- **Spec/Phase ref:** Phase 48b (Patient Defense applies "dodge"); PHB Dodge action.
- **D&D rule:** While dodging, attack rolls against you have disadvantage if you can see the attacker.
- **Problem:** Patient Defense, `/action dodge`, and the AdvanceTurn timer fallback all stamp the `"dodge"` condition on the combatant, but the attack-side `DetectAdvantage` switch lists no `"dodge"` case. The dodging creature gets advantage on DEX saves (condition_effects.go:41) but attackers suffer no disadvantage, so the core benefit of Dodge is missing.
- **Suggested fix:** Add `case "dodge": disadvReasons = append(disadvReasons, "target dodging")` to the target-condition switch in advantage.go. Gate on visibility (no benefit while attacker is blinded) once Phase 48b's prerequisites land.

## [High] Auto-ability selection for finesse weapons silently disables rage damage
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1583 (`attackAbilityUsed`)
- **Spec/Phase ref:** spec §Feature Effect System "Rage … ability_used: str"; Phase 46.
- **D&D rule:** Player chooses STR or DEX for finesse weapons; rage damage only applies on STR attacks.
- **Problem:** `attackAbilityUsed` picks the higher of STR/DEX modifiers for finesse weapons and reports that to FES. A raging barbarian wielding a rapier with `STR 14 / DEX 16` is force-assigned `dex`, the rage `ability_used: str` filter fails, and the +2/+3/+4 melee damage is silently dropped — even though the player would clearly prefer STR while raging. The Sneak Attack `WeaponProperties: ["finesse", "ranged"]` filter is also dependent on this label and has the same blind-spot for the inverse case.
- **Suggested fix:** When the attacker is raging (or otherwise has a STR-only damage rider) and the weapon supports both, prefer the ability that maximizes total damage for that attack — or expose `/attack <target> --ability str|dex` and let the player decide.

## [High] Monk Unarmored Defense not invalidated by shield
- **Location:** /home/ab/projects/DnDnD/internal/combat/equip.go:416 and /home/ab/projects/DnDnD/internal/character/stats.go:63
- **Spec/Phase ref:** Phase 48a; PHB Monk "Unarmored Defense".
- **D&D rule:** Monk's Unarmored Defense (AC = 10 + DEX + WIS) requires no armor AND no shield. (Barbarian's version allows a shield.)
- **Problem:** `RecalculateAC` and `character.CalculateAC` add +2 for shield on top of any `ac_formula`-derived AC, with no class check. The Monk-existing `TestCalculateAC_UnarmoredDefense_WithShield` even asserts this: monk + shield = 17. A monk picking up a shield therefore keeps Unarmored Defense and gains +2 AC, which is impossible by RAW.
- **Suggested fix:** When `ac_formula` is the monk variant, skip the shield bonus (or invalidate the formula entirely when a shield is equipped). The class can be inferred from the formula tokens (presence of `WIS` is the easy heuristic) or from features in scope.

## [High] Monk Unarmored Movement not gated on "no shield"
- **Location:** /home/ab/projects/DnDnD/internal/combat/monk.go:487 (`UnarmoredMovementFeature`) and /home/ab/projects/DnDnD/internal/combat/turnresources.go:275 (EffectContext build)
- **Spec/Phase ref:** Phase 48a — "Unarmored Movement (+speed scaling when no armor/shield)"; PHB Monk.
- **D&D rule:** Unarmored Movement applies only while wearing no armor and not using a shield.
- **Problem:** `UnarmoredMovementFeature` only filters on `NotWearingArmor: true`. The turn-start `EffectContext` builder only fills `WearingArmor`. A monk wielding a shield (already incorrectly allowed by the previous finding) still gets the +10–30 ft speed bonus.
- **Suggested fix:** Add a `HasShield` field to `EffectContext` and a `NotUsingShield` / `RequiresNoShield` condition flag, populate from `EquippedOffHand`, and AND it into the Unarmored Movement filter.

## [High] Wild Shape on-revert does not restore the druid's speed snapshot
- **Location:** /home/ab/projects/DnDnD/internal/combat/wildshape.go:181 (`RevertWildShape`)
- **Spec/Phase ref:** Phase 47 — "Stat swap: snapshot original, … HP in beast form, overflow damage on revert."
- **D&D rule:** When Wild Shape ends, the druid reverts to their previous statistics — including speed.
- **Problem:** `WildShapeSnapshot` stores `SpeedFt` and `AbilityScores`, but `RevertWildShape` only writes `HpMax`/`HpCurrent`/`Ac`. The snapshot's speed/ability fields are never read on revert. Today this happens to work because the live speed is sourced from `char.SpeedFt` and overridden in `turnresources.go:239` while wild-shaped — but the snapshot is functionally dead code, and any future migration that copies speed onto the combatant will silently break revert.
- **Suggested fix:** Either drop the unused snapshot fields or actually consume them in `RevertWildShape` (and add a regression test that flips speed/ability before/after wild shape + revert).

## [High] Wild Shape activation does not block druid spellcasting
- **Location:** /home/ab/projects/DnDnD/internal/combat/spellcasting.go:381 and /home/ab/projects/DnDnD/internal/combat/wildshape.go (no spellcast gate)
- **Spec/Phase ref:** Phase 47 — "Spellcasting blocked (except Beast Spells 18+)"; PHB Druid Wild Shape.
- **D&D rule:** A wild-shaped druid can't cast spells (Beast Spells lifts this restriction at L18).
- **Problem:** The `/cast` path rejects raging casters but has no check for `IsWildShaped`. `CanWildShapeSpellcast(druidLevel)` is defined in wildshape.go but never called by the cast pipeline. A wild-shaped druid below level 18 can still `/cast` freely.
- **Suggested fix:** In `Service.Cast`, after the `IsRaging` check, look up the druid level and reject when `caster.IsWildShaped && !CanWildShapeSpellcast(druidLevel)`.

## [High] Channel Divinity action validation is duplicated and racy across DM-queue + auto-resolved paths
- **Location:** /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:160, :366, :446, :520, :590
- **Spec/Phase ref:** spec §744–848; Phase 50.
- **D&D rule:** Channel Divinity costs the action and one use per short rest.
- **Problem:** Each Channel Divinity option re-validates the class/level/uses independently. The DM-queue path (`ChannelDivinityDMQueue`) deducts the use even if the DM later rejects the effect — there is no rollback handle returned to the queue notifier. Since the DM-queue post is "best-effort" (no error surfaced when `s.dmNotifier == nil`), a Cleric using e.g. "Knowledge of the Ages" with no notifier wired will burn a use silently with no follow-up.
- **Suggested fix:** Either (a) defer the use deduction until the DM resolves, or (b) require a notifier to be wired before allowing the deduction (return an error from `ChannelDivinityDMQueue` if `s.dmNotifier == nil`). Add a test for "notifier-less DM queue burns no uses".

## [Medium] Divine Smite crit bonus computed twice when target is undead and crit
- **Location:** /home/ab/projects/DnDnD/internal/combat/divine_smite.go:59 (`SmiteDamageFormula`)
- **Spec/Phase ref:** spec §758 — "+1d8 vs undead/fiend — doubled on crit"; Phase 51.
- **D&D rule:** Divine Smite: 2d8 + 1d8/slot above 1st (max 5d8) + 1d8 vs undead/fiend; crit doubles all smite dice.
- **Problem:** Code does `count = SmiteDiceCount(slot); if isUndead { count++ }; if isCrit { count *= 2 }`. That matches RAW (+1d8 vs undead is part of the smite dice, so it doubles too) — and the log string says "(doubled) +2d8 vs undead". However, the log says "+2d8 vs undead" not "+2d8 vs undead/fiend"; the formula treats fiend identically (via `isUndeadOrFiend`) so the message is misleading when target is a demon. Low-impact text bug.
- **Suggested fix:** Change "vs undead" suffix to "vs undead/fiend" or surface the actual creature type.

## [Medium] Divine Smite eligibility doesn't enforce "weapon attack"
- **Location:** /home/ab/projects/DnDnD/internal/combat/divine_smite.go:52 (`IsSmiteEligible`) and class_feature_prompt.go:58
- **Spec/Phase ref:** spec §758 — "After melee weapon hit"; PHB Divine Smite.
- **D&D rule:** Divine Smite triggers only on a melee weapon attack hit, not on a melee spell attack (e.g., Inflict Wounds).
- **Problem:** `IsSmiteEligible` checks `result.Hit && result.IsMelee` only. Today this is harmless because `/cast` doesn't go through `Service.Attack`, but unarmed strikes (which RAW debates whether they qualify) and any future "melee spell attack" routed through Attack would trigger an incorrect smite prompt. Worth a defensive check.
- **Suggested fix:** Add a `WeaponID != "" && !IsSpellAttack` guard in `IsSmiteEligible`, or pass weapon category into `AttackResult` and gate on it.

## [Medium] Action Surge resets `AttacksRemaining` from current character data instead of remembering the action's attack count
- **Location:** /home/ab/projects/DnDnD/internal/combat/action_surge.go:58
- **Spec/Phase ref:** Phase 53; PHB Fighter "Action Surge".
- **D&D rule:** Action Surge grants one additional action (with its normal attack count).
- **Problem:** `updatedTurn.AttacksRemaining = int32(s.resolveAttacksPerAction(ctx, char))` overwrites the remaining count instead of *adding* it. A fighter with multiattack who used 1 of 2 attacks before surging keeps `AttacksRemaining = 2` rather than `1 + 2 = 3` — that's correct only because the spec's "additional action" resets the count. But if the fighter hadn't used their action yet (had 2 remaining), surging now also resets to 2 — they lose one attack on the floor. Marginal but observable.
- **Suggested fix:** `updatedTurn.AttacksRemaining += int32(s.resolveAttacksPerAction(ctx, char))`. Same fix for `ActionUsed`: only reset if `cmd.Turn.ActionUsed == true`.

## [Medium] Bardic Inspiration self-grant rejected even when out of combat
- **Location:** /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:151
- **Spec/Phase ref:** Phase 49; PHB Bardic Inspiration.
- **D&D rule:** Bard must choose "a creature other than yourself within 60 feet". So self-targeting is correctly disallowed in combat — but the check happens before validating that the command came from a /bonus action context anyway.
- **Problem:** Rejection is correct, but the error message ("cannot grant Bardic Inspiration to yourself") would be clearer with a hint that the target must be an ally — minor UX.
- **Suggested fix:** Update the error text to "Bardic Inspiration must target another creature (not yourself)".

## [Medium] Bardic Inspiration: no 60ft range validation
- **Location:** /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go (no distance check)
- **Spec/Phase ref:** Phase 49; PHB Bardic Inspiration (target must be within 60ft).
- **D&D rule:** Target a creature within 60 feet who can hear you.
- **Problem:** `GrantBardicInspiration` validates uses, target-doesn't-already-have-die, and not-self, but never verifies the target is within 60 ft. A bard can inspire an ally across the map.
- **Suggested fix:** Add `combatantDistance(cmd.Bard, cmd.Target) > 60` check after the self/already-has guards.

## [Medium] PreserveLife heal target validation can mutate map iteration order under errors
- **Location:** /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:625
- **Spec/Phase ref:** Phase 50; PHB Life Domain "Preserve Life".
- **D&D rule:** Distribute 5×cleric_level HP, each target restored to at most half max HP.
- **Problem:** The first loop validates each target against its `target.HpMax / 2` ceiling. If any one target is over half max, the entire call fails — but the caller has no way to know which target was the offender from the error string alone (the name is included, but the partial state — which targets would have succeeded — isn't surfaced). Minor: also `target.HpCurrent >= halfMax` uses `>=` so a target *exactly* at half is rejected; RAW says "creatures restored to no more than half of their hit points" — so a target *at* half should be ineligible (already at the cap). That actually matches RAW; just noting.
- **Suggested fix:** Return a structured validation error listing each ineligible target, rather than failing on the first one.

## [Medium] Turn Undead does not differentiate "can see or hear" requirement
- **Location:** /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:213
- **Spec/Phase ref:** spec §750 — "Each undead within 30ft that can see or hear the Cleric"; PHB Turn Undead.
- **D&D rule:** Only undead that can see or hear the cleric are forced to save.
- **Problem:** The implementation iterates all undead within 30 ft and forces a save — no deafened/blinded check on the target, no Silence-zone check on the cleric. RAW: an undead that is both blinded and deafened, or a deafened undead while the cleric is in a Silence zone, should be unaffected. Spec line 750 explicitly calls out "can see or hear".
- **Suggested fix:** Skip targets whose conditions include both `blinded` and `deafened`; also skip when the cleric is silenced and the target is blinded.

## [Medium] Wild Shape concentration retention not implemented
- **Location:** /home/ab/projects/DnDnD/internal/combat/wildshape.go:333 (`ActivateWildShape`)
- **Spec/Phase ref:** Phase 47 — "Concentration maintained" on Wild Shape; PHB Druid (concentration carries through Wild Shape).
- **D&D rule:** Entering Wild Shape does not break concentration on an existing spell.
- **Problem:** `ActivateRage` explicitly drops concentration via `breakStoredConcentration` (rage.go:281). `ActivateWildShape` neither drops nor protects concentration — it just sets the new form. There's no test asserting concentration is *preserved* across Wild Shape. If any downstream code (e.g. AC change or HP swap) ever triggers a concentration save, the druid could be silently broken.
- **Suggested fix:** Add a regression test asserting concentration metadata is unchanged across `ActivateWildShape` and `RevertWildShape`.

## [Medium] Stunning Strike duration uses `"end_of_turn"` with `DurationRounds: 1`
- **Location:** /home/ab/projects/DnDnD/internal/combat/monk.go:398
- **Spec/Phase ref:** Phase 48b — "1 ki, CON save or stunned for 1 round".
- **D&D rule:** Stunned until end of the monk's next turn.
- **Problem:** The condition is built with `DurationRounds: 1` and `ExpiresOn: "end_of_turn"` but `StartedRound: cmd.CurrentRound`. This expires at the end of the *target's* next turn (whichever comes first in initiative order), not the *monk's* next turn. Edge case but observable when target acts before the monk in initiative — they'd lose the stun a full round early.
- **Suggested fix:** Tag the condition with `source_combatant_id` (already done) and have the turn-end resolver match the source combatant's turn-end specifically.

## [Medium] Rage rounds counter underflows below 0
- **Location:** /home/ab/projects/DnDnD/internal/combat/rage.go:158 (`DecrementRageRound`)
- **Spec/Phase ref:** Phase 46.
- **D&D rule:** Rage lasts 1 minute = 10 rounds.
- **Problem:** `DecrementRageRound` does `c.RageRoundsRemaining.Int32--` without a floor. `ShouldRageEndOnTurnStart` triggers at `<= 0` but the decrement still happens first, so the persisted value can go to -1, -2, etc. Cosmetic but visible if `/status` ever surfaces the raw value.
- **Suggested fix:** `if c.RageRoundsRemaining.Int32 > 0 { c.RageRoundsRemaining.Int32-- }`.

## [Medium] Resolution priority places `EffectModifyAttackRoll` and `EffectModifySave` at the same priority as immunities-after — but advantage cancellation is a later step
- **Location:** /home/ab/projects/DnDnD/internal/combat/effect.go:134 (`EffectPriority`)
- **Spec/Phase ref:** spec §1554 — Resolution Priority: Immunities → R/V → Flat → Dice → Adv/Disadv.
- **D&D rule:** Flat modifiers sum, then dice modifiers stack, then advantage cancels.
- **Problem:** Order is correct, but `EffectAura` / `EffectReplaceRoll` / `EffectDMResolution` / `EffectResourceOnHit` are all bucketed at `PriorityFlatModifier`. The Aura/DMResolution buckets aren't applied in the switch anyway, but `EffectReplaceRoll` runs in the same pass and can clobber the d20 *after* the flat modifier sum has been computed — surprising if two ReplaceRoll effects (e.g. Portent + Lucky) layer on the same d20.
- **Suggested fix:** Document the priority for `ReplaceRoll` explicitly and add a tie-break (e.g., "lower numeric value wins" so a Lucky-rerolled 18 doesn't get clobbered by a Portent 4).

## [Medium] EffectGrantImmunity stores condition immunities in the same slice as damage immunities
- **Location:** /home/ab/projects/DnDnD/internal/combat/effect.go:322
- **Spec/Phase ref:** spec §Resolution Priority "Immunities — condition immunities prevent effects from applying"; Phase 44.
- **D&D rule:** Damage immunities and condition immunities are separate kinds — a fire-immune creature is not also charmed-immune.
- **Problem:** `case EffectGrantImmunity: result.Immunities = append(result.Immunities, e.DamageTypes...); if e.On != "" { result.Immunities = append(result.Immunities, e.On) }`. Both damage types and a single `On` (condition name) flow into the same `Immunities []string`. Downstream `ApplyDamageResistances` filters by damage type and will treat e.g. a `"frightened"` token as a damage immunity that never matches anyway. The condition-immunity payload is never consulted by any condition-application path.
- **Suggested fix:** Split into `DamageImmunities` and `ConditionImmunities` slices in `ProcessorResult`, and have `ApplyCondition` consult the latter.

## [Medium] Resource_on_hit prompt only fired by Divine Smite, not by feature declarations
- **Location:** /home/ab/projects/DnDnD/internal/combat/class_feature_prompt.go and /home/ab/projects/DnDnD/internal/combat/effect.go:373
- **Spec/Phase ref:** spec §1535 — "resource_on_hit: Trigger resource use on hit/damage. Battlemaster maneuvers, Divine Smite on hit"; Phase 51.
- **D&D rule:** Divine Smite is one of many on-hit prompts; future features (Eldritch Smite, maneuvers) need the same plumbing.
- **Problem:** `EffectResourceOnHit` in `ProcessEffects` only appends the feature name to `ResourceTriggers`. There is no consumer of `ResourceTriggers` — the Divine Smite prompt is hard-coded into `populatePostHitPrompts` rather than driven by the effect declaration. Phase 51 calls out "Driven by `resource_on_hit` effect type" but the implementation is still hardcoded.
- **Suggested fix:** Have `populatePostHitPrompts` consume `result.ResourceTriggers` to fire generic on-hit prompts, and re-express Divine Smite as a `resource_on_hit` effect declaration.

## [Medium] Lay on Hands self-targeting can heal undead/construct PCs without rejection
- **Location:** /home/ab/projects/DnDnD/internal/combat/lay_on_hands.go:58
- **Spec/Phase ref:** Phase 52; PHB Lay on Hands.
- **D&D rule:** Lay on Hands "has no effect on undead or constructs."
- **Problem:** Undead/construct rejection only runs when `cmd.Target.CreatureRefID.Valid`, i.e. for NPCs. A PC race that is itself a construct (e.g. Warforged via homebrew) bypasses the check. Today PCs aren't typed, so this is theoretical, but the check should run uniformly.
- **Suggested fix:** Inspect the target's character race / creature type regardless of whether `CreatureRefID` is populated.

## [Medium] Channel Divinity DC uses WIS for both Cleric and Paladin
- **Location:** /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:186
- **Spec/Phase ref:** Phase 50; PHB Spell Save DC by class.
- **D&D rule:** Paladin save DCs use CHA; Cleric save DCs use WIS.
- **Problem:** `TurnUndead` (Cleric) correctly uses WIS. But `SacredWeapon` and `VowOfEnmity` (both Paladin) don't compute any save DC, and `Preserve Life` (Cleric) uses no DC. So this isn't broken in those paths today. However the `SpellSaveDC` helper is class-agnostic and accepts whatever ability score the caller passes — if a future paladin Channel Divinity option needs a save (Oath of Vengeance "Abjure Enemy") the helper will silently accept WIS if the caller passes the wrong score. Trap waiting to spring.
- **Suggested fix:** Add `SpellSaveDCFor(class, scores)` that hardcodes the right ability per class and use it from all paths.

## [Low] PaladinAuraRadiusFt = 30 only at L18+, spec says L18
- **Location:** /home/ab/projects/DnDnD/internal/combat/feature_integration.go:290
- **Spec/Phase ref:** spec line 2587 referenced in the code comment.
- **D&D rule:** Aura grows to 30 ft at paladin level 18.
- **Problem:** Code is correct (`>= 18`). Just confirming. No fix needed.

## [Low] Bardic Inspiration uses CHA mod min 1 — correct, but doesn't account for negative CHA
- **Location:** /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:33
- **Spec/Phase ref:** Phase 49.
- **D&D rule:** Uses per rest = CHA modifier, minimum 1.
- **Problem:** Code uses `if mod < 1 { return 1 }`, so a CHA 8 bard gets 1 use. Correct per RAW. No fix needed. Noting for completeness.

## [Low] FormatBardicInspirationUse hardcodes the `+` sign
- **Location:** /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:75
- **Spec/Phase ref:** Phase 49.
- **D&D rule:** Bardic Inspiration die roll is always positive.
- **Problem:** Bardic Inspiration cannot roll a negative result, so the `+%d` format is always fine. No fix needed.

## [Low] WildShape CR limit returns float64 — comparisons against parsed CR strings are exact
- **Location:** /home/ab/projects/DnDnD/internal/combat/wildshape.go:59
- **Spec/Phase ref:** Phase 47.
- **D&D rule:** Druid Wild Shape CR cap by level.
- **Problem:** `WildShapeCRLimit(8, false)` returns `1` (float). `ParseCR("1")` returns `1.0`. Equality holds. `ParseCR("1/4") = 0.25`. All standard CR strings work; only an unparseable CR would silently round to 0.0 and pass the CR check. Minor robustness gap.
- **Suggested fix:** Have `ParseCR` return an error on unparseable input and reject Wild Shape activation when the beast's CR can't be read.

## [Low] Channel Divinity DM-queue routes class name as `cmd.ClassName` without normalization
- **Location:** /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:533
- **Spec/Phase ref:** Phase 50.
- **D&D rule:** Channel Divinity is a Cleric / Paladin feature.
- **Problem:** `ClassLevelFromJSON(char.Classes, cmd.ClassName)` is case-insensitive (via `strings.EqualFold` in `classLevel`), but `ValidateChannelDivinity` lowercases `cmd.ClassName` for its own switch. Mixed-case callers work, but a typo like "Klerik" silently returns level 0 and fails with the misleading "requires Cleric or Paladin class" instead of "unknown class".
- **Suggested fix:** Validate the class name against an allow-list at the handler edge.

## [Low] Action Surge `ResourceBonusAction` not checked
- **Location:** /home/ab/projects/DnDnD/internal/combat/action_surge.go:25
- **Spec/Phase ref:** Phase 53.
- **D&D rule:** Action Surge grants an action, NOT a bonus action or reaction.
- **Problem:** `ActionSurge` doesn't reset `BonusActionUsed` or `ReactionUsed` (correct). It also does not require any resource to be free before granting the extra action — but RAW says the surge is "on your turn", so as long as the fighter is in their turn and `!ActionSurged` the cost is just the use. No fix needed; this is correct.

---

## Phase status

- **Phase 44 (Feature Effect Engine):** Mostly OK. Effect-type vocabulary, trigger points, condition filters, and priority all present. Bugs: `EffectGrantImmunity` conflates damage + condition immunities; `EffectResourceOnHit` has no consumer.
- **Phase 45 (Class Feature Integration):** Mostly OK, but the seed-to-mechanical-effect mapping is broken for Rage (Critical above). Sneak Attack / Evasion / Uncanny Dodge / Fighting Styles wired correctly in tests.
- **Phase 46 (Rage):** Broken — see four Critical findings (seed mismatch, STR check, STR save IsRaging, finesse auto-ability).
- **Phase 47 (Wild Shape):** Partially broken — concentration not protected, spellcasting not blocked, revert snapshot fields unused.
- **Phase 48a (Monk Martial Arts / Unarmored Defense / Movement):** Mostly OK, but shield doesn't invalidate Unarmored Defense and doesn't gate Unarmored Movement.
- **Phase 48b (Ki Abilities):** Partially broken — Step of the Wind dash uses wrong base, Dodge condition has no defensive effect (both PD and standard dodge).
- **Phase 49 (Bardic Inspiration):** Mostly OK — missing 60-ft range validation; expiration + status surfacing wired.
- **Phase 50 (Channel Divinity):** Mostly OK — uses table correct; Turn Undead see/hear gate missing; DM-queue path burns uses without notifier wired.
- **Phase 51 (Divine Smite):** Mostly OK — slot/crit/undead math correct; prompt only fires for paladins with the feature; not driven by `resource_on_hit` declaration as Phase 51 calls out.
- **Phase 52 (Lay on Hands):** OK — pool, adjacency, cure-poison/disease, self-targeting all correct (modulo construct-PC theoretical gap).
- **Phase 53 (Action Surge):** Mostly OK — same-turn re-surge prevented; minor over-reset of `AttacksRemaining` documented.
