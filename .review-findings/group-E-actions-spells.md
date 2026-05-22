# Group E Review — Standard Actions + Spells (Phases 54–67)

Read-only correctness review against `docs/dnd-async-discord-spec.md` and
`docs/phases.md`. Findings sorted Critical → Low.

---

## [Critical] Single-target spell casts never apply damage or healing
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:584-598`, `/home/ab/projects/DnDnD/internal/discord/cast_handler.go` (no Apply path)
- **Spec/Phase ref:** Phase 58 "Spell Casting — Basic"; spec §891-1072 (Spell attack rolls vs AC; healing fields)
- **D&D rule:** A successful spell attack deals the spell's damage; healing spells restore HP equal to dice rolled.
- **Problem:** `Cast()` computes `ScaledDamageDice` and `ScaledHealingDice` as **strings** and emits them in the combat log but never rolls the dice nor calls `UpdateCombatantHP` / `ApplyDamage`. Fire Bolt, Inflict Wounds, Guiding Bolt, Cure Wounds, Healing Word, etc. all just print the dice string with no HP change on the target. Lay-on-Hands by contrast does persist HP (`lay_on_hands.go:132`).
- **Suggested fix:** After step 12 (spell attack roll) in `Cast()`, roll the scaled damage dice on hit and route through `s.ApplyDamage`. For healing spells, roll `ScaledHealingDice` and call `UpdateCombatantHP` (clamped to HpMax). Mirror Lay-on-Hands' update path.

## [Critical] AoE damage path ignores upcasting and cantrip scaling
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:851` (`ResolveAoEPendingSaves` reads `dmgInfo.Dice` raw); `aoe.go:967` (`ResolveAoESaves` rolls `input.DamageDice` directly).
- **Spec/Phase ref:** Phase 60 "Upcasting, Ritual, Cantrip Scaling"; spec §891-1072 (cantrip scaling, upcasting).
- **D&D rule:** Fireball upcast to 4th level rolls 9d6 (+1d6/level above 3rd). Cantrip AoEs (Thunderclap, Acid Splash) scale dice with character level (×2 at L5, ×3 at L11, ×4 at L17).
- **Problem:** `CastAoE` does compute `effectiveSlotLevel` but never calls `ScaleSpellDice` on the damage. The AoE damage pipeline uses the base `dmgInfo.Dice` string verbatim, so a 5th-level Fireball still rolls 8d6, and Thunderclap stays at 1d6 forever.
- **Suggested fix:** In `ResolveAoEPendingSaves` (or stash the scaled dice on the pending row), call `ScaleSpellDice(dmgInfo, spellLevel, effectiveSlotLevel, charLevel)` and pass the result into `AoEDamageInput.DamageDice`. Char level must be looked up from the original caster.

## [Critical] Dodge condition does not impose disadvantage on attackers
- **Location:** `/home/ab/projects/DnDnD/internal/combat/advantage.go:104-134` (no `dodge` case in target conditions loop)
- **Spec/Phase ref:** Phase 54 "Dodge"; spec §1138 ("attacks against the character have disadvantage").
- **D&D rule:** Dodge gives attackers disadvantage on attack rolls against you until your next turn.
- **Problem:** The "dodge" condition is applied to the dodging combatant and is consulted by `CheckSaveConditionEffects` for DEX-save advantage, but `DetectAdvantage` never checks for it. Attacks against a Dodging target therefore proceed at normal advantage. Half the Dodge benefit is missing.
- **Suggested fix:** Add `case "dodge":` to the target-conditions switch in `DetectAdvantage`, emitting `disadvReasons = append(disadvReasons, "target dodging")`. Per RAW, the rule requires the dodger to see the attacker and not be incapacitated/speed-0 — gate the disadv on those preconditions if available.

## [High] Help action grants advantage only on attacks, not on ability checks
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:254-261`; `advantage.go:78-85` (only consumed via attack flow)
- **Spec/Phase ref:** Phase 54 "Help"; spec §1140 ("advantage on next attack roll … or advantage on their next ability check").
- **D&D rule:** Help grants advantage on the next attack roll OR the next ability check (helper's choice / situational).
- **Problem:** The implementation always sets `help_advantage` scoped to a `TargetCombatantID` (an enemy combatant). Ability-check Help (e.g., helping a teammate pick a lock) cannot be modelled because the condition assumes an attack target. Helped checks proceed at normal odds.
- **Suggested fix:** Make `target` optional. When omitted, set `TargetCombatantID = ""` and a flag (e.g., `Condition: "help_check_advantage"`) consumed by the next non-attack d20 roll for that ally. Plumb through `internal/check` / `internal/save` ability-check entry points.

## [High] AoE pending save DC subtraction loses cover information
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:592` (`Dc: int32(ps.DC - ps.CoverBonus)`)
- **Spec/Phase ref:** Phase 59 "AoE & Saves"; spec §891 "AoE + cover interaction (DEX save bonus)".
- **D&D rule:** Half cover = +2 DEX save vs AoE; three-quarters cover = +5. The bonus applies to the **saver's roll**, not the DC.
- **Problem:** Storing `DC - CoverBonus` while the player's `/save` adds only their DEX mod is mathematically equivalent for the success/fail outcome — but the saver never sees the bonus in their roll log, the DC displayed to them is artificially lowered, and the `PendingSave.CoverBonus` is discarded after row creation. Worse, the resolution path reads back `r.Dc` and re-compares against `result.Total` (which lacks the cover bonus by design), so any future feature that surfaces "rolled X vs DC Y" will print misleading values.
- **Suggested fix:** Keep `Dc` = the original spell DC and add a `cover_bonus` column to `pending_saves` (or encode it in the `source` tag). At resolution time add the cover bonus to the player's d20 total before comparing to DC.

## [High] Pact-magic upcast respects pact level but silently ignores `--slot` requests
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:446-457`
- **Spec/Phase ref:** Phase 64 "Pact Magic (Warlock)"; spec §891-1072 ("Upcast must be <= pact slot level").
- **D&D rule:** Warlocks always cast at their highest pact slot level; multiclass warlocks pick pool explicitly.
- **Problem:** If a multiclass warlock passes `--slot 2` but their pact slot is level 3 and `UseSpellSlot=false`, the code uses the pact slot at level 3 regardless of the requested level. The spec requires rejecting `--slot N` above the pact slot level; the reverse case (slot below pact level) is also silently overridden. Players cannot intentionally downcast below pact level.
- **Suggested fix:** When `cmd.SlotLevel > 0` and falling into the pact path, reject with "Upcast slot level X exceeds pact slot level Y" if `cmd.SlotLevel > pactSlots.SlotLevel`. If `cmd.SlotLevel < pactSlots.SlotLevel`, reject as well or fall back to regular slots — the spec is ambiguous but silent override is wrong.

## [High] Multiclass spellcasting ability picks highest score, not class-of-spell
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:1542-1557` (`resolveSpellcastingAbilityScore`)
- **Spec/Phase ref:** Phase 58; spec §988-989 ("The spellcasting ability varies by class").
- **D&D rule:** Each spell uses its **casting class**'s ability — a Wizard/Cleric uses INT for Wizard spells and WIS for Cleric spells.
- **Problem:** `resolveSpellcastingAbilityScore` iterates classes and returns the maximum score among all spellcasting classes. A Wizard 5 / Cleric 1 with INT 16 and WIS 14 casts Cure Wounds with INT bonus instead of WIS.
- **Suggested fix:** Pass the spell row through to ability resolution, look up the originating class from `spell.classes`, and use that class's ability. Fall back to "highest of available casting classes" only when the spell could plausibly belong to multiple of the character's classes (rare).

## [High] Spell attack rolls never apply advantage/disadvantage
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:638` (`roller.RollD20(attackMod, dice.Normal)`)
- **Spec/Phase ref:** Phase 58; spec §989 ("Melee spell attacks within 5ft of a prone target get advantage; ranged spell attacks against a prone target get disadvantage").
- **D&D rule:** Spell attack rolls receive the same advantage/disadvantage from conditions, cover, hidden/invisible, prone-vs-melee/ranged, etc. as weapon attacks.
- **Problem:** Cast hard-codes `dice.Normal` for the d20 roll. Hidden caster, invisible target, target prone within 5ft of melee touch spell, attacker restrained/poisoned — none of these adjust the spell attack roll. The spec's prone interaction is called out explicitly; the implementation ignores it.
- **Suggested fix:** Call `DetectAdvantage` with `AttackerHidden`, target conditions, distance, and weapon=zero (or a synthesized "spell" attack type) to derive the roll mode, then pass that mode to `RollD20`.

## [High] Concentration check DC always fires "max(10, dmg/2)" but DC=10 isn't max with damage 19
- **Location:** `/home/ab/projects/DnDnD/internal/combat/concentration.go:18-24`
- **Spec/Phase ref:** Phase 61; spec §893 ("DC = max(10, half damage)").
- **D&D rule:** `DC = max(10, floor(damage/2))`.
- **Problem:** `ConcentrationCheckDC` returns `half` only if `half > 10`. With damage = 20, half = 10 → falls through to `return 10`. With damage = 21, half = 10 (integer division) → again 10. Off by one: at damage = 22, half = 11. The spec wants `>=` not `>`. Behavior is correct **for ties** (DC=10 either way) but the wording is misleading; the real bug is that `half = damage/2` truncates, so damage 21 yields DC 10 not DC 10 anyway (`max(10, 10)`). Functional impact: none for the typical case, but worth tightening.
- **Suggested fix:** Reformulate as `dc := damage/2; if dc < 10 { dc = 10 }; return dc`. Behaviorally identical, but matches the spec phrasing.

## [High] AoE damage applies `int(float64(baseDamage)*0.5)` truncates instead of rounding
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:1024`
- **Spec/Phase ref:** Phase 59; spec §891 ("Half damage on save").
- **D&D rule:** "Half" damage means **round down** per RAW (PHB §202), which Go's `int(x)` truncation does correctly for positive values — but only because all baseDamage values are non-negative integers. The bigger concern: `damage = int(float64(baseDamage) * 0.5)` cannot represent fractional damage from other multipliers (e.g., vulnerability ×2 is fine, resistance ×0.5 truncates the same way).
- **Problem:** Minor: code is correct but `float64 * multiplier` is fragile. Cleaner is `baseDamage / 2` for half damage. Also, save effect "special" returns multiplier -1.0 and is bounded to 0 — silently swallows DM-required handling.
- **Suggested fix:** Replace float multiplier with integer math (`baseDamage / 2`); for "special" save effects, return early and route to `#dm-queue` rather than zero-damaging the targets.

## [Medium] Twinned Spell does not enforce "single creature target" beyond AoE/self check
- **Location:** `/home/ab/projects/DnDnD/internal/combat/metamagic.go:108-116`
- **Spec/Phase ref:** Phase 66b; spec §948 ("Spell must target only one creature").
- **D&D rule:** Twinned requires single-target. Spells like Chain Lightning (4 targets), Magic Missile (multiple darts), Scorching Ray (multiple rays), and any "you can affect up to N creatures" spell are NOT twinnable.
- **Problem:** Twin only rejects self-range and AoE. A non-AoE multi-target spell (Magic Missile, Scorching Ray) has no `area_of_effect` JSON, so Twin happily accepts it. The spec line 949 doesn't drill into this but RAW Sage Advice is explicit.
- **Suggested fix:** Add a `single_target = true` boolean to the spell schema (or check `spell.targets` count); reject Twin when the spell can target >1 creature inherently.

## [Medium] Pact slot deduction does not refuse when `effectiveSlotLevel == 0` and damage path requires upcast
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:444-457`
- **Spec/Phase ref:** Phase 64.
- **D&D rule:** Warlock pact slots always cast at pact level — if you want to cast a 1st-level spell with no upcast benefit, it still consumes a slot at pact level (a "5th level Burning Hands" via pact slot 5 is the only way).
- **Problem:** The current pact code sets `effectiveSlotLevel = pactSlots.SlotLevel` (good), but `ScaleSpellDice` is called with `slotLevel = pactSlots.SlotLevel`, so damage IS scaled to pact level — that's RAW for the **single-target** path. **However** in CastAoE the pact-slot branch is missing entirely. A Warlock casting Fireball through an AoE spell ID would fall into the regular-slot path or fail.
- **Suggested fix:** Mirror the pact-slot detection logic from `Cast` into `CastAoE`. Without this, multiclass Warlocks who can take Fireball via Eldritch Lore feat can't cast AoEs through pact slots.

## [Medium] Hide: success comparison ties go to perceiver, but spec says "meets or exceeds"
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:361`
- **Spec/Phase ref:** Phase 57; spec §1474 ("If any hostile's passive Perception meets or exceeds the roll: hide fails").
- **D&D rule:** Stealth check vs passive Perception is contested; ties typically go to the defender. Spec phrasing matches code (`>`).
- **Problem:** Not actually a bug — code (`stealthTotal > highestPP`) matches the spec ("meets or exceeds" = PP wins on ties, requires Stealth > PP). Flagged for awareness; no fix needed.
- **Suggested fix:** None; verify intent in a comment near line 361.

## [Medium] Help action duration tied to ally's turn, not helper's turn
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:254-261` (`SourceCombatantID: cmd.Helper.ID.String()`, `DurationRounds: 1`, `ExpiresOn: "start_of_turn"`)
- **Spec/Phase ref:** Phase 54 "Help"; spec §1140 ("next attack roll … within 1 round").
- **D&D rule:** Help advantage lasts until "the start of your [helper's] next turn".
- **Problem:** With `SourceCombatantID = helper.ID`, the `isExpired` logic only triggers when the helper's start_of_turn occurs — that part is correct. However, the **condition is placed on the ally** with `DurationRounds=1, StartedRound=currentRound`. If the helper is later in initiative than the ally, the ally already had their turn before the condition expires; if the ally's next turn comes before the helper's next turn, the help advantage is correctly available. The interaction with `StartedRound` may yield off-by-one issues — needs explicit tests for "helper at init 20, ally at init 10" scenarios.
- **Suggested fix:** Add a "consumed_by_attack" flag in addition to the round-based expiry; clear the condition on first attack (already done via `consumeHelpAdvantage`) AND on `helper start_of_turn`. Verify both paths in an integration test.

## [Medium] Material component check treats `material_cost_gp = 0` as costly when `Valid = true`
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:470` (`if spell.MaterialCostGp.Valid`)
- **Spec/Phase ref:** Phase 63; spec §891 ("ordinary material components (no gold cost) are automatically satisfied").
- **D&D rule:** Only "costly" components (gold value listed) consume from inventory or require gold fallback.
- **Problem:** `MaterialCostGp.Valid` is true whenever the column is non-NULL — even when it's 0. Seed data that explicitly stores 0 (rather than NULL) for ordinary components will route through the gold-fallback / inventory-check path, prompting "Buy a feather for 0gp?" or failing if inventory lacks it.
- **Suggested fix:** Change the guard to `spell.MaterialCostGp.Valid && spell.MaterialCostGp.Float64 > 0`.

## [Medium] Stand from prone uses integer half (12 for speed 25) — matches Sage Advice but no rounding-direction test
- **Location:** `/home/ab/projects/DnDnD/internal/combat/condition_effects.go:254-257`
- **Spec/Phase ref:** Phase 54; spec §1147 ("Costs half the character's maximum movement speed").
- **D&D rule:** "Half your speed", rounded down per movement-cost convention (Sage Advice Compendium).
- **Problem:** `StandFromProneCost(25) = 12` (Go integer truncation). Speed 25 is unusual but possible (e.g., dwarf with heavy armor strength penalty applied incorrectly). Test asserts 12; documentation doesn't explain rounding direction.
- **Suggested fix:** Add a doc-comment noting rounding-down per Sage Advice; no behavior change needed.

## [Medium] Spell range validation accepts unrecognized `range_type` values silently
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:77-94` (`ValidateSpellRange`, `default: return nil`)
- **Spec/Phase ref:** Phase 58; spec §996 ("Spell range: enforced by backend").
- **D&D rule:** Range MUST be enforced; unknown range types should be rejected.
- **Problem:** Default branch returns `nil`, so any `range_type` not in {self, self (radius), sight, unlimited, touch, ranged} bypasses range enforcement entirely. A typo in seed data ("ranged " with trailing space) silently disables range checks.
- **Suggested fix:** Default case should `return fmt.Errorf("unknown spell range type: %q", spell.RangeType)`. Adds defensive logging.

## [Medium] Ritual casting only validates primary class, ignoring multiclass
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:432-438`
- **Spec/Phase ref:** Phase 60; spec §1018 ("Only classes with the Ritual Casting feature ... can use this option").
- **D&D rule:** Ritual Casting is a class feature. A Cleric 1/Wizard 5 has access via both classes. Conversely, an Eldritch Knight (Fighter subclass) has Wizard spells but no Ritual Casting feature.
- **Problem:** `primaryClass = classes[0].Class` only checks the first class. A Wizard-dipped Fighter (Fighter primary) cannot ritual-cast even if they could via their Wizard side, and a Bard without the Magical Secrets subclass that grants rituals would always be allowed.
- **Suggested fix:** Iterate all classes and accept if any has Ritual Casting. For Bards specifically, check the Magical Secrets / Ritual Casting subclass feature level (typically 6+ for College of Lore).

## [Medium] AoE save DC pre-subtracts cover bonus but stores no roll-side trace for full cover exclusion
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:545-562`
- **Spec/Phase ref:** Phase 59.
- **D&D rule:** Full cover excludes target from AoE entirely.
- **Problem:** Full-cover targets are skipped (`continue`) before adding to `pendingSaves` and never appear in `affectedNames`. So combat log says "Affected: <none>" if all targets had full cover — but log doesn't explain why. DMs may not realize cover is what protected them.
- **Suggested fix:** Add a `FullyCovered []string` slice to `AoECastResult` and surface in `FormatAoECastLog`: "🛡️ Excluded (full cover): X, Y".

## [Medium] OA detection uses `IsNpc` faction check, breaks for PC-vs-PC combat
- **Location:** `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:106-108`
- **Spec/Phase ref:** Phase 55.
- **D&D rule:** OA triggers when leaving any **hostile** creature's reach. Hostility is independent of NPC-vs-PC.
- **Problem:** `if hostile.IsNpc == mover.IsNpc { continue }` filters by faction (NPC vs PC). PC-vs-PC duels, charmed PCs, dominated NPCs siding with party, etc., are not handled. Two NPCs of opposing factions also never trigger OAs against each other.
- **Suggested fix:** Track factions/allegiance explicitly (e.g., `combatant.Allegiance` enum) and use it for hostility checks. Short-term: allow DM override via a per-encounter "hostility matrix".

## [Medium] Hide's auto-reveal-on-attack does not strip prior hide condition records
- **Location:** `/home/ab/projects/DnDnD/internal/combat/attack.go:830-839`
- **Spec/Phase ref:** Phase 57.
- **D&D rule:** Attacking reveals the hider; subsequent attacks should not still apply "first-attack-from-hidden advantage".
- **Problem:** Auto-reveal flips `IsVisible = true` but `DetectAdvantage` keys off `!IsVisible` for hidden status (`AttackerHidden = !attacker.IsVisible` at attack.go:1481). On the next attack the flag will be false, so behavior is correct **by chance**. However, no tracking of "first attack since hiding" exists — if hidden state could be reset by movement away or other obscurement entry, the "advantage on first attack from hiding" semantics are unprotected. Edge case but worth a flag.
- **Suggested fix:** Introduce a `first_attack_from_hidden = true` turn flag set on Hide success and cleared after the first attack roll; gate the advantage on this flag rather than `!IsVisible` alone.

## [Medium] Grapple/shove adjacency uses Chebyshev distance only — no altitude check
- **Location:** `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:73-80, 201-208`
- **Spec/Phase ref:** Phase 56; spec § Altitude system.
- **D&D rule:** Grappling requires the grappler to reach the target (5ft). Flying creatures above another can't grapple them without closing altitude.
- **Problem:** `GridDistanceFt` is 2D Chebyshev; if grappler is at altitude 30 and target at altitude 0 same tile, distance returns 0 (adjacent) — but they're 30ft apart vertically. The spec's altitude system uses 3D Euclidean (`Distance3D`).
- **Suggested fix:** Use `Distance3D(grappler.col, grappler.row, grappler.alt, target.col, target.row, target.alt)` (already exists for Cast/teleport).

## [Medium] Push destination unoccupied-check ignores dead bodies and altitude
- **Location:** `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:221-233`
- **Spec/Phase ref:** Phase 56; spec § Shove.
- **D&D rule:** Shove pushes the target 5 feet away; destination must be available (not blocked by terrain or other creatures).
- **Problem:** The occupancy loop skips `!c.IsAlive`, so a corpse on the destination tile is treated as empty. Per 5e creature spaces remain "difficult terrain" but not blocked. Acceptable per RAW. However, push doesn't validate against walls or out-of-bounds map tiles either (`pushCol` could be 0 or negative).
- **Suggested fix:** Add bounds check (`pushCol >= 1 && pushRow >= 1 && pushCol <= map.Cols`); optionally check `map_tiles` for wall/impassable terrain at the push destination.

## [Medium] `applyConcentrationOnCast` clears prior concentration even on cast failure later (no rollback)
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:620-627`
- **Spec/Phase ref:** Phase 61.
- **D&D rule:** Casting a new concentration spell drops the old; if the new cast fails (e.g., counterspelled), the old concentration should arguably be back.
- **Problem:** `Cast()` calls `applyConcentrationOnCast` at step 10, but subsequent steps (teleport, spell attack, deferred material deduction) can still error out. If they do, the old concentration is already dropped and the new one is already persisted with no rollback. A failed teleport (e.g., destination occupied) leaves the player without their previous concentration spell AND without the new one.
- **Suggested fix:** Defer the concentration switch until after all "may-error" validations have passed (i.e., after step 12 teleport handling). Wrap in a transaction with the slot deduction.

## [Medium] Cone shape projects from caster center, not tile edge
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:113-117` (`ConeAffectedTiles`)
- **Spec/Phase ref:** Phase 59; spec §891 ("Cones originate from the caster toward the target").
- **D&D rule:** "A cone's point of origin is not included in the cone's area of effect, unless you decide otherwise" (PHB). Caster tile is excluded — code does this (skips `dc==0 && dr==0`).
- **Problem:** Geometry quirk: cones grow from a point. The code projects from caster's tile **center**, which for an immediately-adjacent target results in a half-tile-narrow base. Adequate approximation; mostly affects very short cones (e.g., Burning Hands 15ft).
- **Suggested fix:** Document the approximation; consider tile-edge origin if narrow cones miss obvious targets in playtests.

## [Medium] Concentration save uses `currentConcentration` name-string only (no spell ID)
- **Location:** `/home/ab/projects/DnDnD/internal/combat/concentration.go:36-45` (`CheckConcentrationOnDamage`)
- **Spec/Phase ref:** Phase 61; spec §893.
- **D&D rule:** Concentration is on a specific spell; cleanup should remove that spell's effects.
- **Problem:** Damage-driven concentration save fires before resolution looks up the spell ID (via `GetCombatantConcentration` later). If `concentration_spell_id` is unset (legacy data) but `concentration_spell_name` is set, the cleanup path can't strip spell-sourced conditions. Mitigated by `applyConcentrationOnCast` always setting both columns when concentration is acquired post-Phase-118.
- **Suggested fix:** Run a one-shot migration backfilling `concentration_spell_id` from a fuzzy name match for legacy rows; reject CheckConcentrationOnDamage with a warning if name is set but ID isn't.

## [Medium] Passive Perception for creatures lacks proficiency when Skills JSONB is empty
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:516-530`
- **Spec/Phase ref:** Phase 57; spec §1477 ("Passive Perception: 10 + Perception modifier (including proficiency if proficient)").
- **D&D rule:** Many monsters have proficient Perception.
- **Problem:** For creatures, `creatureSkillMod(...)` returns the precomputed mod if present; else falls back to `10 + WIS mod` (no proficiency). Open5e / SRD seed data may omit the `skills` map for creatures with proficient Perception; their passive becomes effectively 10 + WIS instead of 10 + WIS + proficiency. Hide rolls succeed too easily against them.
- **Suggested fix:** Read `creature.ProficiencyBonus` (or derive from CR) and add to WIS mod when `skills` is missing AND the creature has a Perception entry in its stat block. Long-term: seeding pipeline should always populate `skills` map.

## [Medium] Hide's "spotted by" picks highest-PP enemy, but losing tied roll vs second-highest is hidden
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:347-360`
- **Spec/Phase ref:** Phase 57.
- **D&D rule:** Hide fails if any hostile's passive Perception meets/exceeds the roll.
- **Problem:** Code tracks only the highest PP hostile in `spottedBy`. If multiple hostiles have equal high PPs and the roll fails against several, only one is named in the log. Minor UX gap.
- **Suggested fix:** Collect all hostiles whose PP >= stealth and list them in the failure message.

## [Medium] Subtle Spell does not actually suppress concentration-break-in-silence
- **Location:** `/home/ab/projects/DnDnD/internal/combat/concentration.go:126-135` (`CheckConcentrationInSilence` checks V/S regardless of metamagic)
- **Spec/Phase ref:** Phase 66b "Subtle Spell".
- **D&D rule:** Subtle Spell removes V/S for that casting — but the *spell* still has V/S components for ongoing concentration purposes. RAW: Subtle bypasses Silence at casting time only.
- **Problem:** `ValidateSilenceZone` correctly blocks casts of V/S spells in Silence, but a Subtle-cast spell isn't excepted (because Subtle is a flag on cast, not a property of the spell). Possible: a Sorcerer Subtle-casts Hold Person in Silence, then exits Silence. Per RAW that's fine. But if they then re-enter Silence, current logic auto-breaks concentration because the spell **as defined** still has V/S. Per RAW this is correct (Subtle only suppresses components at the moment of casting). Tag for clarity.
- **Suggested fix:** No code change needed; add an integration test confirming "Subtle-cast spell + later Silence entry = concentration breaks" matches RAW intent.

## [Medium] Cunning Action passes stale `cmd.Turn` to `resolveHide` after consuming bonus action
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:856-872`
- **Spec/Phase ref:** Phase 54 "Rogue Cunning Action".
- **D&D rule:** Cost a bonus action, then resolve Hide.
- **Problem:** Bonus action is consumed via `UseResource(cmd.Turn, ResourceBonusAction)`. `resolveHide` is then called with `Turn: cmd.Turn` (the pre-consumption value, not `updatedTurn`). The resolveHide function calls `UpdateTurnActions(TurnToUpdateParams(updatedTurn))` at line 373 with the passed `updatedTurn` (which IS the post-consumption value passed in). So `cmd.Turn` is unused inside resolveHide — confusing but not buggy. Worth a rename or removing the field.
- **Suggested fix:** Rename the parameter or pass `updatedTurn` to make ownership clear.

## [Low] OA detection's faction check fails for charm/dominate scenarios
- **Location:** `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:106-108`
- **Spec/Phase ref:** Phase 55.
- **D&D rule:** A charmed PC is friendly to the charmer; moving past former allies provokes OAs from them.
- **Problem:** Same root cause as the medium finding above; called out separately for the charm/dominate use case.
- **Suggested fix:** See faction-tracking suggestion above.

## [Low] Stand from prone does not require movement to be available beyond cost
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:592-595`
- **Spec/Phase ref:** Phase 54.
- **D&D rule:** "If you have no movement left, you can't stand up."
- **Problem:** Code checks `int32(cost) > cmd.Turn.MovementRemainingFt`, which handles the "not enough movement" case. But it doesn't preclude the speed being 0 from grappled/restrained (separately, those conditions don't auto-fail because `cost` would still be `maxSpeed/2`, which is non-zero — meaning a grappled+prone creature could "stand" while still grappled, just with 0 movement remaining). Per RAW grappled doesn't prevent standing but does prevent moving.
- **Suggested fix:** When EffectiveSpeed (after conditions) is 0, reject Stand even if MovementRemainingFt > 0. Or document the corner case.

## [Low] Drop Prone ApplyCondition runs immunity check unnecessarily
- **Location:** `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:651-674`
- **Spec/Phase ref:** Phase 54.
- **D&D rule:** Prone-immune creatures (e.g., Beholder, Treant) shouldn't go prone, even voluntarily.
- **Problem:** Code uses `ApplyCondition` which respects `CheckConditionImmunity`. So a beholder PC (theoretical) wouldn't drop prone — that's RAW-correct. But the log message says "X drops prone" while the condition was actually blocked, only showing the immunity message after. Minor cosmetic.
- **Suggested fix:** Check the returned msgs for an immunity hit and avoid the "drops prone" line if blocked.

## [Low] FormatOAPrompt uses target's display name as the slash arg, which can contain spaces/diacritics
- **Location:** `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:218-221`
- **Spec/Phase ref:** Phase 55.
- **D&D rule:** N/A.
- **Problem:** `/reaction oa %s` interpolates the display name. Names with spaces ("Captain Greybeard") would yield `/reaction oa Captain Greybeard` which Discord may misparse the trailing args. Target IDs are stable; display names are not.
- **Suggested fix:** Use the short combatant ID (`renderer.ShortID(targetID)` or similar) in the slash hint.

## [Low] `IsBonusActionSpell` uses substring match — fragile against locale/whitespace
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:23-25`
- **Spec/Phase ref:** Phase 58.
- **D&D rule:** Spell casting time enum: action, bonus action, reaction, 1 minute, 10 minutes, etc.
- **Problem:** `strings.Contains(strings.ToLower(spell.CastingTime), "bonus action")` matches "1 bonus action" but also any garbage with "bonus action" embedded. Better: parse a canonical enum.
- **Suggested fix:** Normalize CastingTime to an enum during seeding; compare against the enum.

## [Low] `applyMetamagicEffects` swallows unknown metamagic option silently
- **Location:** `/home/ab/projects/DnDnD/internal/combat/metamagic.go:196-214`
- **Spec/Phase ref:** Phase 66b.
- **D&D rule:** N/A.
- **Problem:** The switch has no `default` arm. An unknown option in `cmd.Metamagic` is validated by `ValidateMetamagicOptions` first, but if a future caller bypasses validation, the unknown option produces zero effect with no warning.
- **Suggested fix:** Add `default: log.Warn(...)` or assert valid input.

## [Low] CarefulSpellCreatureCount minimum-1 ignores negative CHA mod gracefully
- **Location:** `/home/ab/projects/DnDnD/internal/combat/metamagic.go:217-228`
- **Spec/Phase ref:** Phase 66b ("up to CHA mod").
- **D&D rule:** RAW says CHA modifier (no minimum of 1; theoretically 0). Spec line 932 says "up to CHA mod creatures".
- **Problem:** A Sorcerer with CHA 10 (mod 0) — by RAW Careful Spell would protect 0 creatures (useless). Code forces minimum 1, granting a free protected ally. Off-RAW.
- **Suggested fix:** Drop the minimum-1: `return AbilityModifier(chaScore)`. Reject the cast if the result is 0 with "Careful Spell requires CHA mod ≥ 1".

## [Low] Distant Spell touch -> 30ft does not propagate into `ValidateSpellRange`
- **Location:** `/home/ab/projects/DnDnD/internal/combat/metamagic.go:136-144`; spellcasting.go does not consult `DistantRange` for range validation.
- **Spec/Phase ref:** Phase 66b "Distant Spell".
- **D&D rule:** Distant doubles range or turns touch into 30ft.
- **Problem:** `applyMetamagicEffects` sets `result.DistantRange` as a display string, but `ValidateSpellRange` is called BEFORE metamagic effects are applied (step 7 vs step 9c). So a Distant-cast touch spell at 20ft distance is rejected as "out of range" before Distant ever kicks in.
- **Suggested fix:** Apply metamagic range adjustments before `ValidateSpellRange` in the Cast pipeline (or pass a "distant" flag into the validator that doubles the limit / overrides touch).

## [Low] Empowered Spell reroll surfaces only the lowest dice (no player choice)
- **Location:** `/home/ab/projects/DnDnD/internal/combat/metamagic.go:248-277`
- **Spec/Phase ref:** Phase 66b.
- **D&D rule:** Sorcerer chooses which damage dice to reroll.
- **Problem:** Code documents this is by design (forfeit-friendly canonical "always reroll worst"), but never offers an interactive prompt for the player to pick. Acceptable as default; verify SR-025 covers the optional prompt.
- **Suggested fix:** SR-025 already tracks this. No change required.

## [Low] Heightened Spell + AoE picks "first affected" without exposing target choice in non-Discord paths
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:541-562`
- **Spec/Phase ref:** Phase 66b "Heightened Spell".
- **D&D rule:** Sorcerer picks one target.
- **Problem:** Without an explicit `HeightenedTargetID`, the first affected combatant in `affected` order receives disadvantage. Order is dictated by `ListCombatantsByEncounterID` — non-deterministic if not sorted. Test-flakiness risk and bad UX for non-Discord (DM dashboard, replay) flows.
- **Suggested fix:** Sort `affected` deterministically (e.g., by initiative or by combatant ID) before picking the default heightened target; surface a warning when no explicit target was supplied.

## [Low] Twin Spell consumes (spell_level) sorcery points — but spec says "1 for cantrips"
- **Location:** `/home/ab/projects/DnDnD/internal/combat/sorcery.go:38-50`
- **Spec/Phase ref:** Phase 66b.
- **D&D rule:** Twin cost = spell level, minimum 1 (cantrips cost 1 SP).
- **Problem:** `SorceryPointCost("twinned", 0) = 1` — correct. For level 1 spell, cost = 1. For level 0 spell (cantrip), cost = 1. Matches spec. **However** Twin per RAW errata changed cantrips to cost 1 SP only if the cantrip can target one creature — already validated by `validateTwinnedSpell`. OK.
- **Suggested fix:** None; flagged for completeness.

## [Low] FontOfMagic conversion cap check uses `sorcLevel` as max, not feature_uses.max
- **Location:** `/home/ab/projects/DnDnD/internal/combat/sorcery.go:208-211`
- **Spec/Phase ref:** Phase 66a.
- **D&D rule:** Sorcery point max = sorcerer level (per Sorcerer table).
- **Problem:** Code checks `fom.currentPoints + pointsGained > fom.sorcLevel`. If `feature_uses["sorcery-points"].max` was manually overridden by the DM (e.g., via a magic item or homebrew), the conversion still uses raw sorcerer level. Edge case.
- **Suggested fix:** Read max from `feature_uses["sorcery-points"].Max` rather than `sorcLevel`.

## [Low] Ritual casting allows any class with feature; Bard requires "Ritual Caster" (Lore subclass)
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:1235-1242` (`HasRitualCasting`)
- **Spec/Phase ref:** Phase 60.
- **D&D rule:** Only Bards of College of Lore (or with Magic Initiate feat) have Ritual Casting. Base Bard does NOT (Wizard, Cleric, Druid, Bard-with-feature do).
- **Problem:** Function returns true for all Bards. A non-Lore Bard 5 can ritual-cast their spells, which is incorrect per RAW.
- **Suggested fix:** Check character features for `ritual_casting = true` instead of class name; or restrict Bards to subclass=Lore (`if className == "bard" && subclass == "College of Lore"`).

## [Low] Spell preparation max calculation uses character.level instead of class.level
- **Location:** `/home/ab/projects/DnDnD/internal/combat/preparation.go:236-240`
- **Spec/Phase ref:** Phase 65.
- **D&D rule:** Prepared spells = casting ability mod + caster class level (not total character level).
- **Problem:** Code uses `pc.classLevel` (the relevant class's level — Cleric level for Cleric prep) — correct. **However** `MaxPreparedSpells(abilityMod, classLevel)` uses Cleric/Druid/Paladin class level only. For multiclass Cleric 3/Wizard 5, the prep limit is `WIS mod + 3` (Cleric level), which matches RAW. OK.
- **Suggested fix:** None — flagged for confirmation in a multiclass test.

## [Low] AlwaysPreparedSpells subclass list omits Cleric Light, Tempest, etc. and Druid Moon/Stars
- **Location:** `/home/ab/projects/DnDnD/internal/combat/preparation.go:69-93`
- **Spec/Phase ref:** Phase 65.
- **D&D rule:** Each Cleric Domain and Druid Circle has its own always-prepared spell list.
- **Problem:** Only Cleric Life, Paladin Devotion, and Druid Land are seeded. Other subclasses get nothing.
- **Suggested fix:** Expand `alwaysPreparedBySubclass` to cover SRD subclasses (Light, Knowledge, Nature, Tempest, Trickery, War for Cleric; Vengeance, Ancients for Paladin; Moon for Druid).

## [Low] OA prompt does not display the OA target's tile or the reach distance
- **Location:** `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:218-221`
- **Spec/Phase ref:** Phase 55.
- **D&D rule:** UX nicety.
- **Problem:** Prompt says "moved out of your reach (left C2)" but doesn't say where the mover ended up or that the hostile uses reach 10ft. For a glaive wielder, the player may want to know "they're now 15ft away, I can still reach with my reach weapon".
- **Suggested fix:** Add target's final tile and the reach used in the prompt.

## [Low] Empowered + Twinned combo applies Empowered to both targets but only one reroll budget
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:980-1010`
- **Spec/Phase ref:** Phase 66b.
- **D&D rule:** "Empowered Spell ... reroll up to CHA mod damage dice" — applies to the spell's damage, which for Twinned single-target spells is rolled once per target (separate rolls).
- **Problem:** AoE branch only; Cast (single-target) + Twin doesn't run through this code. The Twin second-target damage path is not in the reviewed code — likely missing entirely.
- **Suggested fix:** Verify Twinned single-target spell damage is actually rolled and applied for the second target (likely impacted by the Critical "single-target Cast doesn't apply damage" finding).

## [Low] Pact Magic check does not respect Sorcerer's slot pool (no cross-pool override)
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:446-456`
- **Spec/Phase ref:** Phase 64 multiclass note.
- **D&D rule:** Multiclass Warlock/Sorcerer keeps two distinct slot pools. `--spell-slot` flag forces regular slot.
- **Problem:** Verified: `UseSpellSlot` flag works as documented. Flagged here because the `--slot N` flag for forcing a specific level is separate from `--spell-slot` (force-pool). Confusing naming; no functional bug.
- **Suggested fix:** Rename `UseSpellSlot` → `ForceRegularSlot` for clarity.

## [Low] DragMovementCost is always ×2 regardless of number of grappled targets
- **Location:** `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:373-375`
- **Spec/Phase ref:** Phase 56; spec §1452 ("Multiple grappled creatures do not further multiply cost — dragging always costs ×2").
- **D&D rule:** Matches PHB ruling.
- **Problem:** Correct per spec and RAW.
- **Suggested fix:** None — confirming intent.

## [Low] Concentration save not enqueued for self-damage from spells like Wrathful Smite
- **Location:** `/home/ab/projects/DnDnD/internal/combat/concentration.go:422-448` (`MaybeCreateConcentrationSaveOnDamage`)
- **Spec/Phase ref:** Phase 61.
- **D&D rule:** Any damage (including self-inflicted) triggers a concentration save.
- **Problem:** Function fires whenever a concentrating combatant takes damage. Should work for self-damage; not verified.
- **Suggested fix:** Add integration test for "caster concentrating on Bless, takes self-damage via a fire shield reflection, saves to maintain".

## [Low] Spell DC calculation uses `ProficiencyBonus` from character row without class-progression check
- **Location:** `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:631`
- **Spec/Phase ref:** Phase 58.
- **D&D rule:** Proficiency bonus by character level (RAW): +2 (1-4), +3 (5-8), +4 (9-12), +5 (13-16), +6 (17-20).
- **Problem:** `char.ProficiencyBonus` is stored; if seeding doesn't update on level-up, DC stays stale. No functional bug here, but a dependency on correct level-up wiring.
- **Suggested fix:** None for E review; verify in Group covering level-up.

## [Low] CastAoE never enforces ValidateSeeTarget on AoE single-creature anchors
- **Location:** `/home/ab/projects/DnDnD/internal/combat/aoe.go:466-468`
- **Spec/Phase ref:** Phase 58 see-target.
- **D&D rule:** Most AoE spells target a point, not a creature, so see-target doesn't apply. Spells like Sleep target a point and area though.
- **Problem:** AoE casts only validate range to the destination tile; sight-to-destination isn't checked unless the spell has a `teleport.requires_sight` flag (which AoEs don't). RAW: "you must have an unobstructed path to the target square" for most AoEs. Cover/walls between caster and AoE origin aren't enforced.
- **Suggested fix:** Add cover/line-of-effect check from caster to AoE origin point; reject if full cover.

---

## Phase Coverage

- Phase 54 (Standard Actions): findings exist (Dodge attack disadvantage, Help check, Stand-speed, DropProne immunity message) — overall OK
- Phase 55 (Opportunity Attacks): findings exist (faction check, prompt UX) — overall OK
- Phase 56 (Grapple/Shove/Drag): findings exist (3D distance, push bounds) — overall OK
- Phase 57 (Stealth & Hiding): findings exist (passive perception proficiency, hide-first-attack flag) — overall OK
- Phase 58 (Spell Casting Basic): **critical findings** (no damage/healing applied; multiclass ability; spell attack adv) — NEEDS FIX
- Phase 59 (AoE & Saves): **critical findings** (no upcast/cantrip scaling in AoE; cover DC encoding; full-cover logging) — NEEDS FIX
- Phase 60 (Upcasting/Ritual/Cantrip): findings exist (ritual class check, see also Phase 59 critical) — partial OK
- Phase 61 (Concentration): findings exist (rollback on cast failure; legacy concentration column) — overall OK
- Phase 62 (Teleportation): OK
- Phase 63 (Material Components): finding exists (MaterialCostGp = 0 edge case) — overall OK
- Phase 64 (Pact Magic): findings exist (silent slot-level override; CastAoE missing pact branch) — partial OK
- Phase 65 (Prepare): findings exist (always-prepared subclass coverage; ritual check) — overall OK
- Phase 66a (Sorcery Points/Framework): finding (cap check uses class level not feature max) — overall OK
- Phase 66b (Metamagic Options): findings (Twin single-creature target check; Distant-touch range; Heightened sort order) — overall OK
- Phase 67 (Spell Effect Zones): OK
