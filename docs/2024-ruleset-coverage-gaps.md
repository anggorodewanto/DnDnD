# 2024 Ruleset Coverage Gaps — Handover / Pickup Backlog

**Purpose.** A self-contained backlog of D&D 2024-ruleset mechanics that are
*partially* wired, *data-only*, or *stale* in DnDnD. Written so a fresh agent can
pick any single `COV-##` item and execute it **without re-investigating the
codebase**. Every item names the exact hole, the file:line evidence, the
already-wired machinery to mirror, an implementation sketch, and an acceptance bar.

**How to use this doc.**
1. Pick a `COV-##`. Prefer Tier 1 (systemic) → Tier 2 (cheap mirror wins) → Tier 3/4.
2. Read its "Mirror" pointer first — an equivalent mechanic is almost always already
   wired; copy its shape rather than inventing one.
3. Red/green TDD per [CLAUDE.md](../CLAUDE.md): failing test first, minimal green, then
   `make cover-check` (90% overall / 85% per-pkg) and `/simplify`.
4. Flip the item's **Status** to `IN PROGRESS` / `DONE (commit)` inline as you go.
5. If a migration is added, update the testdb table lists + `MigrateDown` test (see
   memory `project_new_migration_test_hooks`).

**Relationship to `docs/live-play/issues.md`.** That file is the DM-side live-play
field journal (ISSUE-0xx). This file is the *engineering* feature-gap backlog. When you
start a `COV-##`, you may promote it to the next free `ISSUE-###` if you want it in the
shared ledger — but do **not** rewrite live-play issue history to do so.

**Ground truth this doc was built from** (verified 2026-07-04, commit `d491ce1`):
three parallel code-survey passes over the feat/mastery engine, spell resolution, and
class-feature coverage. Two Tier-1 claims were re-verified by hand (see COV-1/COV-2).

---

## Orientation — the effect engine (read once, reused by most items)

The combat effect engine is the spine most of these items plug into.

- **Core model:** `internal/combat/effect.go`
  - `Effect` (`:99`), `EffectType` (20 consts `:13`), `TriggerPoint` (8 consts `:53`),
    `EffectConditions` (`:77`), `EffectContext` (`:158`), `FeatureDefinition` (`:117`).
  - Pipeline: `CollectEffects` (`:293`) → `EvaluateConditions` (`:191`) →
    `SortByPriority` (`:315`) → `ProcessEffects` (`:323`, returns `ProcessorResult`).
  - Trigger points wired: `on_attack_roll`, `on_damage_roll`, `on_take_damage`,
    `on_save`, `on_check`, `on_turn_start`, `on_turn_end`, `on_rest`.
  - **Reserved, no consumer:** `EffectAura`, `EffectDMResolution` (`:395-404`).
- **Slug → mechanics registration:** `internal/combat/feature_integration.go`
  - `BuildFeatureDefinitions` (`:379`): `switch` mapping a `mechanical_effect` slug string
    to a `FeatureDefinition`. Only these slugs are mapped today: `rage, sneak_attack,
    evasion, uncanny_dodge, archery, defense, dueling, great_weapon_fighting,
    pack_tactics, aura_of_protection, wild_shape, martial_arts_d4,
    bonus_action_unarmed_strike, speed_plus_10`.
  - `BuildAttackEffectContext` (`:481`), `collectMagicItemFeatures` (`:528`).
- **Attack resolution + trigger firing:** `internal/combat/attack.go`
  - `ResolveAttack` (`:633`) fires `ProcessEffects(..., TriggerOnAttackRoll)` (`:763`) and
    `TriggerOnDamageRoll` (`:766`).
  - `populateAttackFES` (`:2406`) assembles `input.Features` + injects riders
    (`GreatWeaponMasterFeature` `:2459`, `SacredWeaponFeature`, `HexFeature`).
  - `Service.Attack` (`:1198`) orchestrates → `applyHitDamage` (`:1362`) →
    `applyMasteryEffects` (`:1375`) → `applyCleaveAttack` (`:1383`) →
    `populatePostHitPrompts`.
  - Feat lookup: by name `HasFeatureByName`, by slug `HasFeat`→`hasFeatureEffect` (`:30/322`).
- **On-take-damage trigger:** `internal/combat/damage.go` — `ApplyDamage` (`:186`),
  `collectFESResistances` fires `TriggerOnTakeDamage` (`:444-464`).
- **Post-hit reaction prompt pattern** (offer a choice after a hit lands):
  `internal/combat/class_feature_prompt.go` `populatePostHitPrompts` (`:29`) sets
  eligibility flags on `AttackResult`; Discord posts/dispatches via
  `internal/discord/class_feature_prompt.go` + `attack_handler.go:444-537`.
- **Pre-roll reaction window** (declare before the roll): `internal/combat/reactions.go`
  `AvailableReactions` (`:48`), `applyReactionToRoll` (`:93`); consumed in Turn Builder
  `turn_builder_handler.go:308-320`.
- **Limited-use resource pools:** `internal/portal/init_feature_uses.go` seeds
  rage, ki, channel-divinity, lay-on-hands, bardic, action-surge, second-wind,
  wild-shape, sorcery-points.
- **Spell resolution:** `Service.Cast` single-target (`internal/combat/spellcasting.go:361`),
  `Service.CastAoE` (`internal/combat/aoe.go:377`). AoE saves: `ResolveAoESaves` /
  `ApplySaveResult` (`aoe.go:230`).
- **Condition application:** `ApplyCondition` (condition system in
  `internal/combat/condition*.go`).

### Already solid — DO NOT redo

- **All 8 weapon masteries** (Cleave, Graze, Nick, Push, Sap, Slow, Topple, Vex) wired on
  `Attack`, `OffhandAttack`, `GWMBonusAttack`. `mastery.go` + `attack.go:590-826`.
- **Concentration** fully enforced: one-at-a-time, on-damage CON save (DC max(10,dmg/2)),
  incapacitation auto-break, silence break, rage/end-combat cleanup. `concentration.go`.
- **Cantrip scaling** generic via `damage.cantrip_scaling` flag + character level.
  `spellcasting.go:1276-1375`.
- **GWM 2024**: −5/+10 + prof rider (once/turn) + bonus-action swing on crit/kill. `c8cea2b`.
- **Warlock builder**: pact boon + invocation + expertise picker (ISSUE-060, `baaf206`).
- **Pact of the Blade** (COV-7): a warlock bonded to a pact weapon uses CHA (if higher) for its
  attack + damage rolls, via `effectiveAbilityMod` over the `abilityModForWeapon` chokepoint.
  Its riders **Lifedrinker** (+CHA necrotic on hit) and **Thirsting Blade** (2 attacks) are also
  wired (COV-6).
- **Evasion** (Rogue 7+) wired on the DEX save-for-half chokepoint (`ResolveAoESaves` →
  `ApplyEvasion`): made save = no damage, failed = half. Applies to single-target (COV-1)
  and AoE casts. `evasion` seeded at Rogue L7 (COV-3).
- Wired spell effects: spell-attack damage, single-target + AoE save damage (COV-1),
  **save-or-suck conditions via the generic `conditions_applied` array (COV-2)**, healing,
  teleport (self / self+creature), agonizing-blast EB, Invisibility, Hex, Hunter's Mark
  (on-hit 1d6-force rider, COV-5), Fly, Spare the
  Dying, zone spells (Spirit Guardians, Wall of Fire, Fog Cloud, Darkness, Silence,
  Moonbeam…), Counterspell, Divine Smite.

---

## Tier 1 — Systemic holes (one fix unlocks many spells)

### COV-1 — Single-target save spells resolve to nothing
**Status:** DONE (save+damage slice) 2026-07-04 · **Severity:** high · **Pkg:** `internal/combat`

**Shipped.** `Service.Cast` now enqueues one pending save for single-target
save+damage spells (`spellcasting.go` step "12γ"), reusing the AoE-tagged source
(`AoEPendingSaveSourceFull`) so the existing `/save` handler, DM dashboard
(`ListPendingSaves`), and `ResolveAoEPendingSaves` resolve it and apply
save-for-half/none damage — with zero new Discord/DB plumbing. `CastResult.SavePending`
signals it; `FormatCastLog` prompts the target to roll. Verified end-to-end: the
monster-resolve path (`pending_save_resolve.go:168`) drives the apply step.
Gate: `hasSavingThrow && hasDamage && !IsAttack && ResolutionMode=="auto" && !area`.
Tests: `TestCast_SingleTargetSaveSpell_CreatesPendingSave`, `TestCast_AttackSpell_NoPendingSave`,
`FormatCastLog` pending-save subtest.

**Deferred follow-ups (new COV items when picked up):**
- **Condition-on-fail** for save+damage spells with a rider (Ray of Sickness→poisoned) —
  belongs to **COV-2** (generic `conditions_applied`).
- **Single-target cover** vs DEX saves — `CoverBonus` passed as 0 (AoE computes per-tile;
  single-target does not yet).
- **PC-target auto-prompt** — a PC target isn't yet actively pinged to `/save` (the log
  line tells them; the DM-dashboard path for monster targets is fully automatic).
- **Multi-cast collision** — two *simultaneously-pending* single-target casts of the *same*
  spell share the spell-id source tag, so the resolver waits for both (pre-existing AoE
  behavior). Narrow window; independent-per-target source tags would fix it.

---

**Original problem (for reference):**

**Problem.** `Service.Cast` computes and *prints* the save DC line
(`spellcasting.go:260` → `🛡️ DC %d %s save`; `result.SaveAbility` set `:651`) but creates
**no pending save and applies no effect** for single-target save spells. Only the **AoE**
path (`CastAoE`/`ResolveAoESaves`) actually rolls a save and applies an outcome. So Sacred
Flame, Poison Spray, Hold Person, Blindness/Deafness, Phantasmal Killer, Ray of Sickness,
Command, etc. produce a DC line and zero mechanics.

**Verified.** `grep "PendingSave\|SaveEffect" internal/combat/spellcasting.go` → only the
`:260` print + `:651` display assignment; no save creation.

**Mirror.** The AoE save path: `aoe.go:230 ApplySaveResult`, `aoe.go:937-1231`
per-target pending-save creation + half/no multiplier. Reuse the same pending-save
enqueue + resolution for the single-target case (target = the one creature, not a tile set).

**Sketch.**
1. In `Cast`, when `spell.SaveAbility != ""` and there is a single target, enqueue a
   pending save against that target (same struct AoE uses) instead of only printing the DC.
2. On save resolution, apply the damage multiplier (`save_effect` = half/none) for damage
   spells, and — coupled with **COV-2** — apply the condition on fail for save-or-suck spells.
3. Keep DM-routing (`resolution_mode = dm_required`) untouched.

**Acceptance.** Casting Hold Person on a target enqueues a WIS save; on fail the target is
paralyzed (needs COV-2); on success nothing. Sacred Flame enqueues a DEX save; fail =
full radiant, success = none. Red test first per spell family (save-for-half damage;
save-or-condition).

**Risk.** Save resolution is async (pending-save queue) — verify the condition/damage is
applied at *resolution* time, not cast time. Check how AoE defers it.

---

### COV-2 — `conditions_applied` is dead data; the condition never lands
**Status:** DONE (save-or-suck slice) 2026-07-04 · **Severity:** high · **Pkg:** `internal/combat`

**Shipped.** `conditions_applied` is now read at **save-resolution** time. The shared
resolver `ResolveAoEPendingSaves` (`aoe.go`) applies each `spell.ConditionsApplied` entry
to every target that **failed** its save, via a new `applyOnFailConditions` helper —
covering both single-target casts (COV-1 enqueue) and real multi-target AoE casts in one
chokepoint. Each condition is scoped to the spell (`SourceSpell`) and, for concentration
spells, to its caster (`SourceCombatantID`, found via `casterConcentratingOn` reading the
encounter's concentration columns) so `RemoveSpellSourcedConditions` /
`BreakConcentrationFully` strip it on concentration drop. The COV-1 enqueue gate widened
from `hasDamage` to `hasDamage || hasConditions` (`hasConditions` in `metamagic.go`) so
condition-only save spells (Hold Person, Sleep, Web…) now enqueue a save instead of
printing a DC and doing nothing. Immune targets are skipped inside `ApplyCondition`
(🛡️ line). Duration is indefinite (`DurationRounds=0`): concentration spells clear via
teardown, non-concentration ones via combat-end cleanup / the DM editor. The hardcoded
`invisibility`/`hex`/`fly` cast-time paths are left as-is — they are no-save self-buffs
that never enqueue a save, so no double-apply. `AoEDamageResult.ConditionMessages` carries
the log lines. Tests: `TestCast_SingleTargetConditionSaveSpell_CreatesPendingSave`,
`TestResolveAoEPendingSaves_AppliesConditionOnFailedSave`,
`TestResolveAoEPendingSaves_AppliesDamageAndConditionOnFail`. Coverage: combat 91.5%.

**Deferred follow-ups (new COV items when picked up):**
- **Per-turn re-saves & timed expiry** — save-or-suck conditions apply indefinitely; the
  2024 end-of-turn repeat save (paralyzed/frightened/etc.) and non-concentration duration
  expiry (Blindness/Deafness = 1 min) are not modeled. Cleared only by concentration
  teardown, combat end, or the DM editor.
- **Condition riders** — landing the condition is step one; whether the engine enforces
  each rider (paralyzed = auto-crit in melee ≤5ft; frightened = disadvantage + no-approach)
  is a separate audit.
- **PC-target auto-prompt & multi-cast collision** — inherited from COV-1 (a PC target is
  told to `/save` via the log line, not actively pinged; two simultaneous casts of the same
  concentration spell resolve to the first concentrating caster found).

---

**Original problem (for reference):**

**Problem.** ~20 save spells carry a `conditions_applied` array in seed data and classify
as `auto` off it, but **combat never reads the field**. The only conditions that land
(`invisible`, `hexed`, `fly_speed`) come from **hardcoded per-spell-ID branches**, not the
generic array. So Invisibility works; its field-identical siblings (Sleep→unconscious,
Hold Person→paralyzed, Web/Entangle/Evard's→restrained, Fear/Phantasmal Killer→frightened,
Blindness/Deafness→blinded, Power Word Stun→stunned, Grease/Earth Tremor→prone,
Sickening Radiance→exhaustion) apply nothing.

**Verified.** `grep -rn "ConditionsApplied" internal/combat --include=*.go | grep -v _test`
→ single hit `invisibility.go:95`, and that line *writes* a literal `[]string{"invisible"}`
to build a condition — it does **not** read `spell.ConditionsApplied`. Field is genuinely
unconsumed.

**Mirror.** `invisibility.go` shows the `ApplyCondition` call shape; generalize it to read
`spell.ConditionsApplied` and apply each on the appropriate trigger (on cast for
self/buff, on failed save for save-or-suck — couple with COV-1).

**Sketch.**
1. After a save resolves as "affected" (COV-1) OR for no-save conditions on cast, loop
   `spell.ConditionsApplied` and call `ApplyCondition` on the target, sourced/scoped to the
   spell so concentration teardown (`BreakConcentrationFully`, `concentration.go:527`) clears
   them.
2. Duration: tie to `spell.Concentration` (concentration-tracked) or `spell.Duration`.
3. Retire the hardcoded `invisibility.go` literal path *or* leave it and let the generic
   path own everything except its special break-on-attack rule — decide during impl; don't
   double-apply.

**Acceptance.** Hold Person on a failed WIS save applies `paralyzed`, cleared when the
caster drops concentration. Sleep applies `unconscious`. No condition double-applies.
Concentration teardown removes spell-sourced conditions.

**Risk.** Some conditions have riders (paralyzed = auto-crit in melee within 5ft;
frightened = disadvantage + no-approach). Applying the *condition* is step one; whether the
condition engine already enforces its riders is a separate check — note gaps as COV
follow-ups rather than expanding scope here.

---

## Tier 2 — Cheap wins (machinery already wired, small surface)

### COV-3 — Evasion / Uncanny Dodge never emitted
**Status:** DONE (Evasion slice) 2026-07-04 · Uncanny Dodge split to **COV-16** · **Severity:** low · **Pkg:** `internal/combat` + `internal/refdata`

**Reality check (the doc's original premise was wrong).** The `FeatureDefinition`s for
both are coded (`feature_integration.go:110`/`:139`), but "engine side already works" was
**false** — neither had an end-to-end consumer:
- `EvasionFeature()` emitted `EffectModifySave{On:"evasion", Modifier:0}`, which
  `ProcessEffects` collected but never acted on (adds a zero modifier). The AoE DEX-save
  damage path computed its half/none multiplier purely from the spell, ignoring the
  target's Evasion. `ApplyEvasion` had zero production callers.
- `UncannyDodgeFeature()` emitted `EffectReactionTrigger{On:"uncanny_dodge"}` into
  `ReactionTriggers`, a slice with **no production reader**. `ApplyUncannyDodge` was
  test-only.

So COV-3 was two unequal halves, not a seed edit.

**Shipped (Evasion).** Evasion is now wired end-to-end. `ResolveAoESaves` (`aoe.go`) — the
single chokepoint reached by both single-target save casts (COV-1 enqueue) and real AoE
casts — now overrides the per-target multiplier with `ApplyEvasion(baseDamage, success)`
when the save is a DEX **save-for-half** (`SaveEffect=="half_damage"`, new
`AoEDamageInput.SaveAbility=="dex"`) **and** the target is a PC with the `evasion` feature
(new best-effort `combatantHasEvasion` helper, mirrors `collectFESResistances`). Result:
made DEX save → no damage, failed → half. Seed: Rogue `features_by_level["7"]` now carries
`{mechanical_effect:"evasion"}` (2024 L7); level-gating (`derive_stats.go:223`,
`lvl<=c.Level`) keeps it off under-level rogues. Tests:
`TestResolveAoESaves_EvasionUpgradesDexSaveForHalf`,
`TestResolveAoESaves_EvasionOnlyAppliesToDexSaves` (ability gate),
`TestIntegration_SeedRogueEvasionFeature` (seed→present link, guards a future reshuffle
from silently making it dead again).

**Known dead scaffolding (follow-up).** `EvasionFeature()` + its `case "evasion"` in
`BuildFeatureDefinitions` emit an inert `EffectModifySave{On:"evasion", Modifier:0}` on
`TriggerOnSave` — the real Evasion mechanic now lives in `ResolveAoESaves`, so that FES def
has **no functional consumer** (nothing reads `On:"evasion"` to reduce damage; the `/save`
path only rolls). It's left in place because 5 tests across `combat`+`discord` assert it,
and it's a plausible anchor if the effect engine ever generalizes save-damage transforms
(the correct trigger to generalize is the *second* such feature, e.g. Improved Evasion).
Its only live side effect — a cosmetic `Evasion: +0` line on the `/save` breakdown — is
suppressed at the render site (`internal/save/save.go` skips zero-modifier
`EffectModifySave` reasons; `TestSave_ZeroModifierSaveEffectSuppressed`).

**Split out.** Uncanny Dodge is a **post-hit damage-halving reaction**, a different shape
from the existing pre-roll **+AC** reaction model in `reactions.go` (which only recomputes
hit→miss). It needs a new reaction flavor across combat + Turn Builder + Discord and must
respect the pre-declare / no-heal-back rule. Promoted to **COV-16** with its own plan;
`uncanny_dodge` is intentionally **not** seeded until that consumer lands (seeding it now
would re-create the dead-data anti-pattern this item exposed).

---

### COV-4 — Second Wind: pool seeded, no spend command
**Status:** DONE 2026-07-04 · **Severity:** medium · **Pkg:** `internal/combat` (+ `discord`/`refdata`/`portal`)

**Shipped.** New `Service.SecondWind` (`internal/combat/second_wind.go`) mirrors Lay on
Hands / Action Surge / Martial Arts: as a **bonus action** the fighter self-heals
`1d10 + fighter level` (`SecondWindHealDice`), spending one use from the `second-wind`
pool via the shared `ParseFeatureUses`/`DeductFeatureUse` machinery. Gate order: bonus
action available (`ValidateResource(ResourceBonusAction)`) → is a character (not NPC) →
Fighter L1+ → pool `Current > 0`. HP write reuses the inline `min()`-cap +
`UpdateCombatantHP` + `MaybeResetDeathSavesOnHeal` pattern (no shared heal helper exists).
Recharge needs no new plumbing — the existing rest path already refills any `Recharge:
"short"` key.

Wired through all four registries a bonus action needs: `/bonus second-wind` dispatch +
`BonusCombatService` interface (`bonus_handler.go`), `bonusSubcommandKeys`
(`action_keys.go`), the action catalog row (fighter L1, `action_catalog.go` — pinned by
`TestActionCatalog_MatchesDiscordDispatch`), and `/help bonus` (`help_content.go`). New
`FeatureKeySecondWind` constant replaces the bare literal in `init_feature_uses.go`.

**Seed fix (found during `/simplify` altitude review).** Second Wind is a Fighter **L1**
feature but was seeded only inside the Action Surge `ce.Level >= 2` gate, so a L1 fighter
would see the command + catalog entry but hit "no uses remaining". `init_feature_uses.go`
now gives Second Wind its own `ce.Level >= 1` gate (mirrors the Paladin case's nested Lay
on Hands gate); the L2 builder test still passes.

**Tests.** `second_wind_test.go` (happy path, HP cap, empty pool, bonus-action-used,
not-a-fighter, NPC, `SecondWindHealDice`); `TestBonusHandler_SecondWind` (dispatch routes
to the service, public response). Coverage: combat 91.47%.

**Deferred follow-ups (new COV items when picked up):**
- **2024 use scaling** — 2024 Second Wind is 2 uses at L1, 3 at L4, 4 at L10 (regain 1 on
  short rest, all on long). The pool is still seeded at a flat `{1,1}`; scale
  `Current/Max` by level and split short-rest partial recharge from long-rest full.
- **Turn Builder entry** — offered only via `/bonus second-wind`, not yet a Turn Builder
  bonus-action button (mirror the existing bonus-action UX).

**Original problem (for reference).** The Second Wind use-pool was seeded and rest-recharged
(`init_feature_uses.go`) but had **no combat command to spend it** — no `second-wind`
consumer anywhere in `internal/combat/`. Mirror was `lay_on_hands.go` / `action_surge.go`.

---

### COV-5 — Ranger free Hunter's Mark (2024 Favored Enemy)
**Status:** DONE (on-hit rider slice) 2026-07-04 · **Severity:** medium · **Pkg:** `internal/combat` (+ seed)

**Shipped.** Hunter's Mark's on-hit rider is now wired end-to-end as a direct Hex mirror.
Casting it (`spell.ID == huntersMarkSpellID`, `spellcasting.go`) marks the target with a
source-tagged `hunters_mark` condition (`applyHuntersMarkConditionFromCast`); every weapon hit
the caster lands on that marked target then adds an extra **1d6 force** (2024 damage type) via
`HuntersMarkFeature` (`feature_integration.go`), injected in `populateAttackFES` (`attack.go`)
only when `targetHuntersMarkedBy` matches this attacker — so only the ranger who cast it gets
the rider, and only against that target. The marker is torn down for free on concentration end
through the generic `RemoveSpellSourcedConditions` (matched on caster ID + `spell.ID`) — zero
new cleanup code. During `/simplify` the byte-identical Hex/Hunter's-Mark leaf helpers (marker
match + condition apply) were collapsed into shared `targetMarkedBySpell` /
`applySpellMarkerCondition` (`spell_marker.go`); each spell keeps its own constants + rider
`FeatureDefinition` and forwards the drift-prone logic there. Seed: Ranger Favored Enemy text
updated 2014→2024 (always-prepared Hunter's Mark), and the spell's damage type `weapon`→`force`.
Tests: `hunters_mark_test.go` (`HuntersMarkFeature` shape; marked-target +1d6 force; not-marked
/ marked-by-someone-else no-bonus; cast marks target; no-target no-op). Coverage: gates met.

**Deferred follow-up (new COV item when picked up):**
- **Free-cast pool (Favored Enemy N/day).** 2024 Favored Enemy grants a number of slot-free
  Hunter's Mark casts (regained on a Long Rest). Not wired — casting still spends a spell slot.
  Needs a new limited-use pool seeded for rangers (mirror `init_feature_uses.go`) **plus
  slot-vs-pool substitution in `Service.Cast`'s deduction step**, a genuinely separate (and
  riskier) surface than the on-hit rider; the seed text describes the feature but the free-cast
  machinery is intentionally not yet consumed.

**Original problem (for reference).** 2024 Favored Enemy = a number of free Hunter's Mark casts.
Seed carried the 2014 text (`seed_classes.go:271` "advantage on tracking"). Hunter's Mark had a
spell seed but **no on-hit rider and no free-cast**, unlike Hex which is fully wired. Mirror was
`internal/combat/hex.go` (on-hit +1d6 rider, source-scoped `hexed` tag, cleared on concentration
end) + `HexFeature` + cast branch.

---

### COV-6 — Warlock invocations beyond Agonizing Blast are inert
**Status:** DONE (EB-rider + pact-weapon-rider slices) 2026-07-04 · **Severity:** medium · **Pkg:** `internal/combat`

**Shipped (pact-weapon riders, once COV-7 landed the pact-weapon attack).** The two
Pact-of-the-Blade-gated invocations are now wired:
- **Lifedrinker** (Warlock L12) — every pact-weapon **hit** adds a flat **CHA modifier (min 1)
  necrotic**. New `LifedrinkerFeature(chaMod)` (`feature_integration.go`) is a flat
  `EffectModifyDamageRoll` (not dice, so never crit-doubled — RAW), injected in
  `populateAttackFES` alongside the Hex/Hunter's Mark riders and gated on the same COV-7
  pact-weapon eligibility (`input.PactBladeCHA && HasInvocation(...,"lifedrinker")`). It rides
  the existing `ProcessEffects(TriggerOnDamageRoll)` pipeline and shows as a typed necrotic
  `DamageComponent` in the log.
- **Thirsting Blade** (Warlock L5) — grants a **second attack** with the Attack action. One
  branch in the shared `resolveAttacksPerAction` (`turnresources.go`):
  `HasInvocation(pact_of_the_blade) && HasInvocation(thirsting_blade) → bestAttacks = max(_, 2)`.
  `max` never stacks with a real Extra Attack; the branch (not the class `attacks_per_action`
  seed map) is the only layer that can see an invocation. Weapon-agnostic, matching COV-7's
  "any non-improvised weapon is the pact weapon" scope. Slugs `lifedrinkerEffectID` /
  `thirstingBladeEffectID` added to `pact_blade.go`. Tests: `lifedrinker_test.go`
  (feature shape; +CHA-necrotic on pact-weapon hit with breakdown component; no-pact-blade →
  no rider) + `thirsting_blade_test.go` (both invocations → 2 attacks; either alone → 1).

**Shipped (Eldritch-Blast-rider slice).** The two Eldritch-Blast-cantrip riders that ride
already-live machinery are wired end-to-end, mirroring `agonizing_blast.go` (new
`combat/eldritch_blast_invocations.go`):
- **Repelling Blast** — on an EB **hit** by a warlock carrying the `repelling_blast`
  invocation, the target is pushed 10 ft straight away via the shared `applyPushEffect`
  (`mastery.go`, the same forced-movement machinery the Push mastery and `/shove` use).
  Auto-applied on hit (mirrors the auto-resolved Push mastery), gated by
  `castTriggersRepellingBlast` (spell is EB **and** caster has the invocation).
  `CastResult.RepellingBlastPushed` signals it; `FormatCastLog` prints a 💨 push line.
- **Eldritch Spear** — `applyEldritchSpearRange` extends EB's effective range to 300 ft (from
  120) at the **live** `ValidateSpellRange` chokepoint, so a caster with `eldritch_spear` can
  reach a target the base range would reject. For every other spell/caster it returns the
  spell unchanged.

Both gate on the clean-slug `Feature{MechanicalEffect:<id>}` via `HasInvocation`, exactly like
Agonizing Blast. Tests: `eldritch_blast_invocations_test.go` (gate helpers; EB hit pushes
target to E8 / no-invocation no push; 150 ft cast rejected without Eldritch Spear, accepted
with it; `FormatCastLog` push line). Coverage: combat 91.5%.

**Deferred follow-ups (blocked / new COV items when picked up):**
- ~~**`lifedrinker` + `thirsting_blade`**~~ **DONE 2026-07-04** (see the pact-weapon-rider
  slice above) — unblocked once COV-7 landed the pact-weapon attack. If a future EB/spell
  on-hit rider lands, extract an `applySpellOnHitRiders` switch (analogous to
  `applyMasteryEffects`) so `Cast` stops accreting numbered inline rider blocks — premature at
  n=2 (Agonizing Blast is also inline today).
- **Per-beam Repelling Blast** — the current push is one shove per EB **cast-hit**. True
  per-beam push (shove on each of the 2/3/4 beams independently, potentially different
  targets) needs multi-beam EB (**COV-14**). The single-beam behavior shipped here is the
  correct degradation until then.
- **Eldritch Spear vs Distant Spell inconsistency (note only)** — Eldritch Spear is the first
  *functional* range modifier at `ValidateSpellRange`; the Distant Spell metamagic's range
  extension is still **display-only** (`ApplyDistantSpell` returns a string). Generalizing to
  one "effective spell range (invocation + metamagic)" seam is a Tier-4 cleanup, not wired here.

**Original problem (for reference).** 29 invocations are catalog-defined
(`refdata/invocation_catalog.go`) and builder-pickable (ISSUE-060), but **only
`agonizing_blast` was combat-wired** (`combat/agonizing_blast.go`). `repelling_blast`,
`lifedrinker`, `eldritch_spear`, `thirsting_blade` were inert. Mirror: `agonizing_blast.go`
reads the invocation off the character and modifies EB resolution.

---

### COV-7 — Pact Boons have no combat consumer
**Status:** DONE (Pact of the Blade slice) 2026-07-04 · **Severity:** low · **Pkg:** `internal/combat`

**Shipped.** Pact of the Blade is combat-wired: a warlock carrying the `pact_of_the_blade`
boon uses **Charisma for a pact weapon's attack AND damage rolls**, taken player-optimally
as `max(weapon's normal ability, CHA)` (2024 "can use Charisma"). The boon already persisted
as `Feature{MechanicalEffect:"pact_of_the_blade"}` (builder, `portal/invocations.go`), so
**no seed/data change was needed** — only the missing consumer. New `combat/pact_blade.go`:
`effectiveAbilityMod(input)` centralizes the substitution over the single ability chokepoint
`abilityModForWeapon`. `Service.Attack`→`populateAttackFES` decides eligibility once
(`HasInvocation(char.Features, "pact_of_the_blade") && !IsImprovised && weapon != unarmed`),
setting a new `AttackInput.PactBladeCHA bool`; `ResolveAttack` then swaps CHA into `atkMod`,
`dmgMod`, and both mastery-DC sites via `effectiveAbilityMod`. The `attackAbilityUsed` label
authority gained a `pactBladeCHA` param (mirrors its existing `isRaging` bool) so a CHA swing
reports `ability_used:"cha"` — Rage's melee-STR filter correctly won't misfire on it. The
`PactBladeCHA` flag rides `AttackInput`, so it propagates through the struct-copy in Cleave's
secondary attack for free. Tests: `pact_blade_test.go` (`effectiveAbilityMod` CHA-higher /
weapon-higher / no-boon; `ResolveAttack` end-to-end both directions; `Service.Attack` Blade→CHA
positive + Tome→STR negative control). Coverage: combat 91.5%, gates met.

**Deferred follow-ups (new COV items when picked up):**
- ~~**`thirsting_blade` + `lifedrinker`**~~ **DONE 2026-07-04** (COV-6 pact-weapon-rider slice)
  — the pact-weapon attack from this item unblocked both; `lifedrinker` is a flat
  `EffectModifyDamageRoll` rider in `populateAttackFES`, `thirsting_blade` a `max(_,2)` branch
  in `resolveAttacksPerAction`.
- **Off-hand / thrown-off-hand pact-weapon attacks** — `PactBladeCHA` is set only in
  `populateAttackFES` (main `Attack` + GWM bonus, incl. main-hand thrown). `OffhandAttack`
  (`attack.go`) builds its input via `populateAttackContext` and never sets the flag, so a TWF
  pact-weapon build's off-hand swing still uses STR/DEX. One-liner to close (set the flag in the
  off-hand builder) — niche 2024 corner, left for when TWF-warlock comes up.
- **Pact of the Chain (familiar) / Pact of the Tome (extra cantrips)** — still builder-only,
  no combat consumer. Chain needs a familiar/summon model; Tome's extra cantrips ≈ the
  invocation grant-spell path already done (ISSUE-060) but not yet materialized for boons.

**Original problem (for reference).** Pact boons were builder-pickable but inert —
`invocation_catalog.go:45`: "Pact boons have no mechanical combat consumer yet." Pact of the
Blade (attack with pact weapon, use CHA), Pact of the Chain (familiar), Pact of the Tome
(extra cantrips). Mirror: existing attack path + Monk's `MonkLevel` ability-override precedent.

---

### COV-8 — Cunning Strike / Brutal Strike / Tactical Master / Steady Aim
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/combat` (+ seed for the levels)

Four 2024 martial riders that each sit on already-wired machinery. Each is its own small
item; split if picked up separately.

- **Cunning Strike (Rogue L5):** spend sneak-attack dice for a rider (poison/trip/withdraw).
  Rides the once/turn `SneakAttackFeature` (`feature_integration.go:89`).
- **Brutal Strike (Barb L9):** forgo advantage → on-hit extra damage + effect. Mirrors the
  GWM on-hit rider (`GreatWeaponMasterFeature` `feature_integration.go:317`) and the mastery
  on-hit pipeline (`mastery.go`).
- **Tactical Master (Fighter L9):** swap in push/sap/slow on any mastery weapon. Sits
  directly on `onHitMastery` (`attack.go:602`) / `mastery.go`.
- **Steady Aim (Rogue):** grant advantage this turn (speed 0). Mirrors the reckless /
  `vex_advantage` single-shot grant (`applyRecklessMarker`, `mastery.go:302`, `attack.go:1407`).

**Blocker for all four:** the level's feature must exist in seed data — see COV-10
(`features_by_level` only 1–3 today).

---

## Tier 3 — Feats (only 6 of 41 wired)

### COV-9 — Unwired feats (description-only)
**Status:** IN PROGRESS (Savage Attacker slice DONE 2026-07-04; Alert + Sharpshooter-passives + Polearm-Master-butt-strike + Crossbow-Expert-bonus-attack + Shield-Master-bonus-shove + Shield-Master-Interpose-Shield + Shield-Master-shield-AC-save slices DONE 2026-07-05, Shield Master COMPLETE; Tough +2-HP/level slice DONE 2026-07-05; CON-feat→HP resync slice DONE 2026-07-05) · **Severity:** medium · **Pkg:** `internal/combat` + `internal/refdata` + `internal/discord` + `internal/levelup`

**Shipped (Savage Attacker slice).** Savage Attacker is combat-wired: a character with the
feat rerolls a **melee weapon's damage dice once per turn and keeps the higher total**
(the seeded 2014 shape — melee-only). New `combat/savage_attacker.go`: `rollWeaponDamageSavage`
wraps the existing `resolveWeaponDamage` (identical return signature), rolling it a second time
and keeping the higher total when eligible; `savageAttackerEligible` gates on the feat +
melee + not-yet-used-this-turn. `populateAttackFES` sets the new `AttackInput.SavageAttacker`
flag via a name scan of the already-parsed `feats` slice (`featsHaveName`, no extra
`json.Unmarshal`) — name-based detection mirrors the GWM 2024 precedent, dodging the
`mechanical_effect` JSON-array shape slug matching misses. Both `ResolveAttack` damage sites
(auto-crit + normal hit) call the wrapper and, on a reroll, set `AttackResult.SavageAttackerUsed`
and append `savageAttackerUsedEffect` to `OncePerTurnEffectsFired` — so the existing
`markUsedEffects` machinery (Attack / OffhandAttack / GWMBonusAttack) spends the once-per-turn
lock with **zero new service plumbing**, exactly like Sneak Attack / Cleave / Nick. Because all
three attack paths funnel through `populateAttackFES`, off-hand (TWF) and GWM-bonus melee swings
share the flag and the once-per-turn snapshot for free. `savageAttackerTag` adds a 🎲 combat-log
tag (mirrors `sneakAttackTag`). No seed/data change (the feat was already seeded + builder-pickable).
Tests: `savage_attacker_test.go` (eligibility gate; reroll keeps higher / keeps original when worse;
`ResolveAttack` melee reroll + ranged-no-reroll + already-used-no-reroll; `Service.Attack`
end-to-end feat→reroll; log tag). Coverage: gates met (90% overall / 85% per-pkg).

**Shipped (Alert slice).** Alert is initiative-wired: a character with the feat gets **+5 to their
initiative roll** (the seeded 2014 shape). New `combat/alert.go`: `alertInitiativeBonus(featuresJSON)`
returns 5 when `HasFeatureByName(..., "Alert")` (name-based, GWM/Savage precedent). `getDexModifier`
was renamed `getInitiativeModifiers` returning `(dexMod, featBonus, err)` from the **same single
`GetCharacter` fetch** (it previously read DEX and discarded the character); `RollInitiative` and
`InsertSummonIntoInitiative` add `featBonus` to the `RollD20` modifier. The +5 lands in the roll
**total** only — `InitiativeEntry.DexMod` stays pure so `SortByInitiative`'s DEX tie-break is
unaffected (RAW: Alert adds to the result, not to DEX). Creatures carry no features → `featBonus=0`,
so monster/summon initiative is unchanged. No seed/data change. Tests: `alert_test.go`
(`alertInitiativeBonus` present/absent/case-insensitive/nil; `RollInitiative` +5 in the recorded roll
and beating a higher-DEX rogue; `getInitiativeModifiers` creature→0 bonus). Coverage: gates met.
**Altitude (why no FES):** the Feature Effect System has **no initiative trigger point, no
`EffectModifyInitiative` type, and no `ProcessEffects` consumer anywhere in the initiative path** —
Alert is the first feature ever to touch an initiative roll. A call-site helper is the right depth
(mirrors Savage Attacker); generalize to a `TriggerOnInitiative` lane when a *second* initiative
modifier is built (the seeded-but-unwired Ranger "Natural Explorer" `advantage_initiative` is the
next candidate). DEFERRED: Alert's "can't be surprised" + "no advantage from unseen attackers"
(needs surprise / attacker-visibility tracking); 2024 shape (bonus = proficiency bonus + init-swap).

**Shipped (Sharpshooter passive-riders slice).** Sharpshooter's two **passive** riders are now
combat-wired (the −5/+10 power-attack toggle already existed): a character with the feat makes
ranged weapon attacks that **ignore half & three-quarters cover** and take **no long-range
disadvantage**. A new `AttackInput.HasSharpshooter` (the always-on "has the feat" flag, distinct
from the per-attack `Sharpshooter` −5/+10 toggle) is set in `populateAttackFES` via
`featsHaveName(feats, "Sharpshooter")` (name-based, mirrors Savage Attacker / GWM — no extra
`json.Unmarshal`). Long-range rider: a new `AdvantageInput.HasSharpshooter` guards the
`IsInLongRange` disadvantage branch in `DetectAdvantage`, an exact mirror of the existing
`HasCrossbowExpert` flag one line above. Cover rider: `ResolveAttack` zeroes half/¾ cover before
`EffectiveAC` when `HasSharpshooter && !isMelee`; **full cover still blocks** (handled upstream by
`resolveAttackCover`→`ErrTargetFullyCovered`, and `CoverFull.ACBonus()` is 0 here regardless), and
`result.Cover` keeps the geometric value so the combat log stays truthful. Both riders are always-on
for the feat-haver, independent of whether they take the −5/+10 that swing (RAW). No seed/data change.
Tests: `sharpshooter_test.go` (DetectAdvantage long-range negated with/without feat; ResolveAttack
half + ¾ cover ignored via `EffectiveAC`; melee-weapon cover NOT ignored). Coverage: gates met (combat
91.5%). **Altitude (why call-site, not FES):** FES has no trigger/effect vocabulary to *remove* a
disadvantage source or *ignore cover* — `EffectConditionalAdvantage` only appends reasons, and
`EffectModifyAC`/`ProcessorResult.ACModifier` is never consumed in `ResolveAttack`. The two
hand-placed guards sit at the one layer where cover→AC and adv/disadv resolve, mirroring
`HasCrossbowExpert` and the Alert conclusion. DEFERRED: Crossbow-Expert-style bonus-action attack is a
separate feat; 2024 Sharpshooter shape nuances.

**Shipped (Polearm Master butt-strike slice).** The bonus-action half of Polearm Master is now
wired as `/bonus polearm <target>`. After the Attack action, a character with the feat holding a
**glaive/halberd/quarterstaff/spear** strikes with the opposite (blunt) end: one melee attack at
the same ability mod, damage die overridden to **1d4 bludgeoning**. New `Service.PolearmMasterBonusAttack`
(`polearm_master.go`) mirrors the lightweight monk `MartialArtsBonusAttack` template — bonus-action
gate → feat gate (`HasFeatureByName(..., "Polearm Master")`) → `Turn.ActionUsed` gate → main-hand
weapon gate (`IsPolearmButtWeapon`, an ID allow-list; **pike deliberately excluded** — it grants only
the OA half) → `buildAttackInput` → `resolveAndPersistAttack` → `applyHitDamage` → `markRageAttacked`
→ `populatePostHitPrompts`. The butt weapon is a **value-clone of the equipped polearm** with only
`Damage`/`DamageType` overridden, so proficiency, the STR/DEX choice, the heavy-weapon small-creature
penalty, name, and mastery carry over faithfully (vs. a from-scratch `ImprovisedWeapon`, which would
re-derive them). Discord wiring is symmetric with martial-arts: `BonusCombatService` method +
`dispatchPolearm` + `case "polearm"` + `bonusSubcommandKeys`/`help_content.go` entries. No seed change
(slug `polearm-master` already seeded). **Catalog:** a new **feat-gating axis** — `ActionCatalogEntry.Feats []string`
— was added so the `/bonus` drift-guard (`TestActionCatalog_MatchesDiscordDispatch`) stays honest for a
feat-gated key; `TestActionCatalog_EntriesWellFormed` now accepts a feat gate alongside class/Universal.
Tests: `polearm_master_test.go` (happy path 1d4 bludgeoning not 1d10 slashing; feat/Attack-action/polearm
gates; NPC; bonus-action-used) + `IsPolearmButtWeapon` unit + `bonus_handler_test.go` dispatch. Coverage:
gates met (combat 91.46%, discord 86.24%, refdata 97.92%). DEFERRED (new COV items): (1) **reach
opportunity-attack half** — needs the reaction/OA trigger system, a separate slice; (2) **feat-gated
catalog rows are not yet surfaced on the character sheet** — `AvailableActions`/`buildActionGroups`
(`portal/character_sheet.go`) gate by class only, so the polearm row is honest-but-invisible until feats
are threaded into the sheet; (3) **magic-weapon bonus** (+1 glaive) does not apply to the butt-strike
because FES is skipped (monk-tier parity; more visible here since the clone is a real weapon).

**Shipped (Crossbow Expert bonus-attack slice).** The feat's third rider — the bonus-action
hand-crossbow attack — is now wired as `/bonus crossbow <target>` (its two passives, loading-ignore
and no-disadvantage-firing-in-melee, were already live). After attacking with a one-handed weapon,
a character with the feat fires a hand crossbow they hold (main **or** off hand) as a bonus action.
New `Service.CrossbowExpertBonusAttack` (`crossbow_expert.go`) mirrors the **full-tier**
`GWMBonusAttack` path — NOT the lightweight monk/Polearm template — because a hand crossbow is a real
ranged weapon: it keeps the weapon's own die and runs cover, obscurement, the Feature Effect System
(Sneak Attack, magic-crossbow bonuses, Sharpshooter), Vex mastery, and the once-per-turn tracker, and
sets `input.HasCrossbowExpert = true` so the melee-adjacency no-disadvantage rider carries onto the
bonus swing. Gate order mirrors OffhandAttack: bonus-action → feat (`HasFeatureByName "Crossbow
Expert"`) → **an attack was made this turn** (`AttacksRemaining >= resolveAttacksPerAction`, the
attack-was-made basis, which — unlike Polearm's `Turn.ActionUsed` — correctly excludes a
cast-a-spell action) → hand crossbow in hand (`IsHandCrossbow`, `equippedHandCrossbow` main-then-off)
→ cover gate → **spend a bolt** → bonus action → build/resolve. **Ammo:** the one thing no other
bonus-attack path needed (all melee) — the main Attack path's inventory-deduction block was extracted
to a shared `Service.deductWeaponAmmunition` helper, reused by both. During `/simplify` the helper was
deepened to also fold in the C-37 post-combat recovery tracking (`recordAmmoForAttack`), which the
first extraction left behind in `Attack` — so a bolt fired on the bonus shot is now half-recovered at
End Combat exactly like one fired on `/attack`, and the two call sites can't drift. No seed change
(feat + hand crossbow + bolt all seeded). Discord/catalog wiring symmetric with polearm
(`BonusCombatService` method + `dispatchCrossbowExpert` with `Walls` for ranged cover + `case
"crossbow"` + `bonusSubcommandKeys` + catalog `Feats:["crossbow-expert"]` row + `help_content.go`).
Tests: `crossbow_expert_test.go` (happy path 1d6+DEX with a bolt spent 20→19; ammo tracked for
recovery; off-hand crossbow; feat / attack-first / hand-crossbow / NPC / bonus-used / out-of-ammo
gates) + `IsHandCrossbow` unit + `bonus_handler_test.go` dispatch. Coverage: gates met (combat 91.41%,
discord 86.22%, refdata 97.92%). DEFERRED (new COV items): (1) the "attacked with a **one-handed**
weapon" clause is not enforced (no weapon-of-Attack-action tracking — same simplification as
OffhandAttack's TWF prereq); (2) feat-gated catalog rows still not surfaced on the sheet (shared with
the Polearm deferral); (3) 2024 Crossbow Expert shape nuances.

**Shipped (Shield Master bonus-shove slice).** The first (bonus-action) of Shield Master's three
halves is now wired as `/bonus shield <target> [push|prone]`. After taking the Attack action, a
character with the Shield Master feat who is holding a shield may shove a creature within 5 ft as a
**bonus action** — knock it prone or push it 5 ft — using the same contested check as `/shove`. The
whole contested-check body was **reused, not duplicated**: the resource-agnostic core of `Service.Shove`
was extracted into `resolveShove(ctx, cmd, roller, resource ResourceType)` (`grapple_shove.go`), and the
two callers differ only in which resource they spend — `Shove` passes `ResourceAction`, the new
`Service.ShieldMasterShove` (`shield_master.go`) passes `ResourceBonusAction`. The resource is still spent
only after the read-only size/adjacency/push-occupancy pre-checks, so a failed pre-check burns neither the
action nor the bonus action (behavior-parity of `/shove` verified: no pre-check dropped, reordered, or
duplicated). Gate order: `CanActRaw` → bonus-action → character (not NPC) → `HasFeatureByName "Shield
Master"` → **attack made this turn** (`AttacksRemaining >= resolveAttacksPerAction`, the same
cast-a-spell-excluding basis as Crossbow Expert, not Polearm's `Turn.ActionUsed`) → **shield equipped**
(`hasEquippedShield`, off-hand slot is `ArmorType == "shield"`). No seed change (slug `shield-master`
already seeded). Discord/catalog wiring symmetric with polearm/crossbow, except the dispatcher posts
`ShoveResult.CombatLog` directly (a shove is not an attack, so no `FormatAttackLog`) and parses an optional
`push|prone` mode token (default push): `BonusCombatService` method + `dispatchShieldMaster` + `case
"shield"` + `bonusSubcommandKeys` + catalog `Feats:["shield-master"]` row + `help_content.go`. Tests:
`shield_master_test.go` (prone + push happy paths asserting **BonusActionUsed and NOT ActionUsed**;
feat / shield / attack-made / NPC / bonus-used gates) + `bonus_handler_test.go` dispatch (default-push,
prone-mode, missing-target). Coverage: gates met (combat 91.4%, discord 86.0%, refdata 97.92%).
DEFERRED (all reaction/save work — the other two halves of the feat): (1) **DEX-save damage evasion**
(reaction: take no damage on a successful DEX save-for-half); (2) **+shield-AC to DEX saves vs
single-target effects** (a save rider); (3) the "one-handed weapon" clause is not enforced for shove
either, but shove has no weapon so it's moot; (4) feat-gated catalog rows still not surfaced on the sheet
(shared with Polearm/Crossbow). **/simplify** left the slice unchanged: the shared `resolveShove` seam was
confirmed a clean, parity-safe generalization; the flagged consolidations (the feat-gate prologue now
copied across attack.go/crossbow/shield, and the cold-path double `GetCharacter`) are a cross-file cleanup
best done as one dedicated pass, not piecemeal here.

**Shipped (Shield Master Interpose Shield slice).** The second of Shield Master's three halves is
now wired: **Interpose Shield** — a character with the feat holding a shield takes **no damage** on a
made DEX save-for-half (upgrading the normal half→none). It mirrors the **COV-3 Evasion** wiring at
the same `ResolveAoESaves` chokepoint (single edit covers both single-target COV-1 casts and real AoE
casts, since both funnel through it). New pure `ApplyInterposeShield(damage, saveSuccess)` next to
`ApplyEvasion` (success→0, fail→**full**/unchanged — unlike Evasion it never helps a failed save) and
detector `combatantHasInterposeShield` next to `combatantHasEvasion` (reuses `HasFeatureByName "Shield
Master"` + `hasEquippedShield`). The prior single `if combatantHasEvasion` became a priority `switch`:
**Evasion is checked first because it strictly dominates** (both zero a made save, but Evasion halves a
failed one where Interpose gives full), so a PC with both gets the better fail outcome. The Interpose
case is gated on `sr.Success` first, so its shield lookup only runs for made-save targets. No seed
change (the `shield-master` feat is already seeded, name-detected). Tests: `feature_integration_test.go`
(`ApplyInterposeShield` both branches), `shield_master_evasion_test.go` (made+shield→0, failed+shield→full,
feat-without-shield→normal half, no-feat→normal half, and Evasion-beats-Interpose precedence on a failed
save). Coverage: gates met (combat 91.4%, discord 86.0%, refdata 97.92%).

**RAW simplification (documented, from `/simplify` altitude).** Interpose Shield is RAW a **reaction**
(costs the character's reaction, one-per-round, a player choice), whereas Evasion beside it is a genuine
passive. This auto-applies Interpose for **free** at the save chokepoint like Evasion — the reaction
COST, the one-per-round economy, and a pre-declare prompt are **not** charged. The save-resolution path
has no reaction surface (unlike the enemy-turn Turn Builder where Uncanny Dodge, COV-16, pays the cost),
and per `feedback_reaction_predeclare_no_retroactive` a reaction must be declared *before* the roll —
charging it post-save would be a retroactive spend. So the reaction economy is **deferred to a future
save-path reaction lane** (the same lane COV-1's PC-auto-prompt and COV-16's `/attack` defender-prompt
await); when built, Interpose moves **out** of the Evasion `switch` into that reaction machinery, leaving
only genuine passives (Evasion, Improved Evasion) there. `/simplify` also **skipped**: charging the
reaction here (would violate pre-declare + fragile in the async batch path — hence the deferral) and the
duplicate `GetCharacter` across the two detectors (cold DM-triggered path; fix would re-signature the
COV-3 sibling). Applied: rename `ShieldMasterEvasion`→`InterposeShield` (it isn't the Evasion class
feature) + the reaction-vs-passive disclosure comment.

**Shipped (Shield Master +shield-AC-to-DEX-saves slice — the THIRD and final half).** "If you aren't
incapacitated, add your shield's AC bonus to any DEX save vs a spell/effect that targets **only you**"
is now wired at the single-target save-spell **enqueue** (`spellcasting.go` COV-1 branch, the one place
that knows the effect is single-target — `!hasAreaOfEffect`). The bonus rides the pending save's existing
`CoverBonus` field — the persisted per-target additive bonus `RecordAoEPendingSaveRoll` already adds to
the roll total (`total + CoverBonus >= DC`) — so **zero new resolution plumbing**: the /save path and the
DM path both pick it up for free. New `shieldMasterDexSaveBonus(ctx, target, ability)` (`shield_master.go`)
returns the equipped shield's AC bonus (via new `equippedShieldACBonus`, reading real `AcBase` — a magic
shield gives its full bonus, unlike `RecalculateAC`'s hardcoded +2) gated on dex-save → **not incapacitated**
(`IsIncapacitatedRaw`) → PC → `HasFeatureByName "Shield Master"` → shield equipped. Tests:
`shield_master_save_test.go` (`equippedShieldACBonus` 5 branches; `shieldMasterDexSaveBonus` 7 branches incl.
case-insensitive, non-dex, no-feat, no-shield, incapacitated, monster-target; + two end-to-end `Cast` tests
asserting CoverBonus threads through as 2 / 0). No seed change.

*Reachability (verified via subagent).* Both `Cast`/`CastAoE` are **PC-caster-gated** and enemies have **no
spellcasting entry point** — so a PC makes a single-target DEX save only when another PC casts a single-target
DEX-save spell at them (friendly fire). Same rare-but-real trigger class as COV-3 Evasion and the Interpose
half above (both ride PC DEX saves that only fire on friendly-fire casts today); all three are integration-
tested by seeding the scenario, not by live play. Correct + complete when an enemy-cast path lands.

*Deferred rename debt (from `/simplify` altitude Q1).* `CoverBonus` is semantically the generic "per-target
additive save-total bonus" at the resolution layer (cover is merely producer #1), but the **name lies** at the
DB column (`cover_bonus`) / DM-dashboard JSON / svelte layer. A rename to `save_bonus` is **disproportionate for
this slice** (DB migration + regenerated sqlc + two JSON contracts + `dashboard/svelte/api.js` + ~7 test files),
so it's deferred and tracked HERE: whoever adds single-target cover must **ADD** to `CoverBonus`, not overwrite
(one writer today: the enqueue), and the rename rides that work. The `/save` Discord UI does **not** surface
`cover_bonus` (bonus folded in silently server-side) so there's no player-facing mislabel — only the DM view.

**Shield Master COMPLETE** (all 3 halves shipped: `/bonus shield` bonus-action shove + Interpose Shield DEX-save
damage evasion + this +shield-AC-to-DEX-saves rider). **Deferred:**
- **Interpose reaction economy** — see the RAW simplification above (save-path reaction lane).
- **RAW incapacitation on the OTHER halves** — this slice added the incapacitation guard only to the save-bonus
  rider; the bonus-shove already gates on `CanActRaw`, and Interpose's reaction cost is itself deferred.

**Altitude note (why no new `EffectType`).** Savage Attacker is a *reroll transform* of the base
weapon dice, which the declarative FES `Effect` model (Modifier / Dice / ReplaceValue) cannot
express, and the one nominal transform lane (`EffectReplaceRoll`, backing Great Weapon Fighting)
has **no production consumer** — so there is no working transform machinery to generalize toward. A
dedicated call-site helper is the correct depth; build the shared lane when a *second* transform
feat (or a wired GWF) needs it.

**Deferred follow-ups (new COV items when picked up):**
- **2024 shape** — 2024 Savage Attacker drops the melee restriction (any weapon) and applies to
  the Attack action. The seed carries the 2014 melee-only text; re-seed + relax the `isMelee` gate
  when the 2024 pass reaches feats (sibling of COV-12).
- **Off-hand / GWM-bonus reroll already covered** — all three paths share the flag; no extra work.

**Shipped (Tough slice — first character-derivation feat, `internal/levelup`).** Tough raises a
character's **hit-point maximum by 2 per character level** (and grants those HP, so current HP rises
with max) the instant the feat is gained. New `levelup/feat_hp.go`: `featMaxHPBonus(feat, totalLevel)`
returns `2*totalLevel` for a feat carrying the seeded mechanical-effect slug `hp_plus_2_per_level`
(Tough), 0 otherwise — **slug detection, not name** (mirrors the feature-effect engine's `effect_type`
dispatch; any future feat with the same slug earns it free). Wired into `Service.ApplyFeat` (the
**feat-acquisition seam** — where an ASI-chosen feat is committed to the character's Features), right
after the feature write and **after the idempotency guard**, so a re-approve never double-bumps. The
bump is an **imperative delta** on the persisted `HPMax`/`HPCurrent` columns via the existing
`UpdateCharacterStats` (nil `SpellSlots`/`PactMagicSlots`/`Features` → the adapter's `pickNullable`
preserves the just-written features); `char.Level/ProficiencyBonus/Classes` are re-sent unchanged.
Tests: `feat_hp_test.go` (`featMaxHPBonus` scales-with-level / non-hp-effect / no-effect; `ApplyFeat`
raises max+current with the gap preserved on a damaged char; idempotent double-apply bumps once;
non-HP feat leaves HP untouched) + one line making the levelup mock faithful (`UpdateCharacterStats`
now writes `HPCurrent`). Coverage: gates met (levelup 89.2%). No seed/data change.

**Altitude (why `ApplyFeat`, not `CalculateHP`).** `character.CalculateHP` is the "morally correct"
home (Tough is a pure function of level), but three facts block it: (1) the derivation is deliberately
feats-agnostic and the fresh-build path computes HP **before** features exist; (2) there is **no
general feat picker** — the ASI-feat flow into `ApplyFeat` is the only live acquisition path; (3)
decisive — HP lives in a **persisted store separate from the derivation**, and `CalculateHP` is **not
re-run on a feat pick**, so a feat-aware derivation still wouldn't fire on acquisition. Threading
`feats` through `CalculateHP` + its 4 callers would add surface without being the path that grants the
HP. A local delta at the mutation seam is the right depth.

**Shipped (CON-feat→HP resync slice, `internal/levelup`).** `applyFeatASI` bumps CON for a
CON-raising feat (Durable / Resilient / Tavern Brawler chosen on CON) and calls `recomputeAndPersistAC`,
but **never resynced HP** — so max HP stayed flat even though `CalculateHP` uses `conMod × level`. Now
fixed: a new pure `conHPDelta(oldScores, newScores, totalLevel)` (`feat_hp.go`) returns
`(Δ CON modifier) × totalLevel` — mirroring `CalculateHP`'s CON term — and `applyFeatASI` applies it via
the shared `bumpPersistedHP` helper right after the AC recompute. Detection is by the **CON-modifier
delta**, not feat name: any current-or-future feat that lifts CON across an even boundary earns the HP,
an odd bump that leaves the modifier unchanged (or any non-CON ASI) yields delta 0 and writes nothing.
The `/simplify` pass then **unified the two disjoint feat→HP writers** (Tough's flat block + this CON
resync) onto `bumpPersistedHP(ctx, char, delta)`, which (a) DRYs the nil-JSONB `pickNullable`-preservation
invariant that was duplicated, and (b) **mirrors the delta onto the in-memory `char` snapshot** so the
two writers *compose* rather than clobber — closing the stale-snapshot footgun should a future feat ever
be both flat-HP **and** CON-ASI. Delta-onto-persisted (never recompute-from-scratch) is deliberate: a
from-scratch `CalculateHP` would drop both the damage gap (`HPCurrent`) and any Tough flat bonus already
baked into `HPMax` (the derivation knows neither). Tests: `feat_hp_test.go` (`conHPDelta` table:
mod-rises / odd-bump-no-change / unchanged / level-scaling; `ApplyFeat` Durable raises max+current with
the gap preserved and persists the new CON; odd CON bump leaves HP flat; non-CON ASI feat leaves HP flat).
Coverage: gates met (levelup 89.2%). No seed/data change.

**Deferred follow-ups (new COV items when picked up):**
- **Builder-rebuild loss → now formalized as COV-17 (Tier 5).** A portal builder edit drops **all**
  `Source:"feat"` features (not just Tough's HP) — `CollectFeatures` regenerates class/subclass/racial
  only — silently disabling every feat rider. COV-17 scopes the fix as three slices at the shared
  `UpdateCharacterRecord` preserve seam: S1 feat-feature preservation, S2 flat-feat-HP re-add, S3
  expended feature-uses preservation. (CON-feat HP already survives via preserved ability scores.)
- **Parameterize the magnitude.** The `+2` is hardcoded in Go while the slug encodes only the type; the
  seed precedent (`bonus_initiative` carries `"value":"5"`) shows the pattern — re-seed as
  `effect_type:"hp_per_level"` + `value:"2"` and parse it, generalizing to any "HP per level" feat.

**Wired today:** GWM, **Sharpshooter (COV-9: −5/+10 toggle + passive ignore-half/¾-cover &
no-long-range-disadvantage riders)**, Defensive Duelist,
**Crossbow Expert (COV-9: loading-ignore + no-melee-disadvantage passives + `/bonus crossbow`
hand-crossbow bonus attack)**, Tavern Brawler, Dual Wielder,
**Savage Attacker (COV-9, once/turn melee damage reroll)**,
**Alert (COV-9, +5 initiative)**, **Polearm Master (COV-9, `/bonus polearm` butt-strike; OA half deferred)**,
**Shield Master (COV-9 COMPLETE — `/bonus shield` bonus-action shove + `ApplyInterposeShield` DEX-save damage evasion + `shieldMasterDexSaveBonus` +shield-AC-to-single-target-DEX-saves rider)**,
**Tough (COV-9, +2 HP/level via `ApplyFeat`; first character-derivation feat)**,
**CON→HP resync (COV-9, `conHPDelta`/`bumpPersistedHP` — Durable / Resilient / Tavern Brawler on CON now raise max HP on pick)**.

**Description-only, no combat effect** (in `seed_feats.go`, matched by neither name nor
slug in combat):

| Feat | Effect to wire | Mirror |
| --- | --- | --- |
| Polearm Master | ~~butt-end bonus attack~~ **DONE 2026-07-05** (`/bonus polearm`, `polearm_master.go`); reach OA still deferred (needs reaction/OA trigger) | monk `MartialArtsBonusAttack` template + weapon clone |
| Sentinel | OA on disengage/attack-others; hit sets speed 0 | reaction window `reactions.go` (needs a movement/OA trigger — not yet built) |
| Shield Master | ~~bonus-action shove~~ **DONE 2026-07-05** (`/bonus shield`); ~~DEX-save damage evasion~~ **DONE 2026-07-05** (`ApplyInterposeShield` in `ResolveAoESaves`, mirrors COV-3 Evasion; reaction economy auto-simplified/deferred); shield-AC-to-DEX-saves rider still deferred | Interpose = COV-3 Evasion mirror (`ApplyEvasion`/`combatantHasEvasion` at the save chokepoint), NOT the COV-16 attack-path reaction |
| ~~Savage Attacker~~ **DONE 2026-07-04** | reroll melee weapon damage once/turn, keep higher | `savage_attacker.go` — `rollWeaponDamageSavage` at the `resolveWeaponDamage` call site + once/turn key on `OncePerTurnEffectsFired` |
| ~~Alert~~ **DONE 2026-07-05** | +5 initiative (2014) | `alert.go` — `alertInitiativeBonus` at the `RollInitiative` roll site (`getInitiativeModifiers`); +5 in the roll total, DexMod tie-break kept pure |
| War Caster | advantage on concentration saves; cast as OA | concentration save only auto-rolls on turn timeout (`timer_resolution.go:247`, bare `Roll("1d20")`) — bypasses advantage-aware `save.Service`; NOT a clean rider (needs a player-driven concentration roll first) |
| Charger / Mobile / Lucky / Mage Slayer / Heavy Armor Master | movement / reroll / damage-reduction riders | various — scope each; HAM needs a magical/non-magical damage flag (absent from `ApplyDamageInput`) |
| ~~Tough~~ **DONE 2026-07-05** | +2 max HP per level | `levelup/feat_hp.go` — `featMaxHPBonus` (slug `hp_plus_2_per_level`) applied as an HPMax/HPCurrent delta in `Service.ApplyFeat` (the feat-acquisition seam), NOT threaded into `CalculateHP` (persisted HP store ≠ derivation; see Shipped block) |

**Also:** ~~Crossbow Expert's **bonus-action hand-crossbow attack** is not wired~~ **DONE
2026-07-05** (`/bonus crossbow`, `crossbow_expert.go`; full-tier GWM template + shared ammo helper).

**Note:** `feat.MechanicalEffect` JSON in seed is descriptive metadata only — combat does
**not** parse it. Wiring a feat = add a name/slug branch in the effect pipeline, same as the
6 wired feats. Pick the high-impact ones first (Polearm Master, Sentinel, Shield Master).

---

### COV-16 — Uncanny Dodge: post-hit damage-halving reaction (split from COV-3)
**Status:** DONE (enemy-turn Turn Builder slice) 2026-07-05 · **Severity:** low-medium · **Pkg:** `internal/combat` + `internal/refdata`

**Shipped.** Uncanny Dodge is wired end-to-end as the **first post-hit damage-halving
reaction**, on the enemy-turn Turn Builder path. It reuses the existing reaction plumbing
rather than adding a parallel system: `ReactionOption` gained a `HalveDamage bool` flag (the
+AC flavor keeps `ACBonus`), and a new pure `uncannyDodgeReaction(char.Features)` builder
(slug-detected via `hasFeatureEffect(features, "uncanny_dodge")`, exactly mirroring COV-3's
`combatantHasEvasion`) is appended in `AvailableReactions` alongside `defensiveDuelistReaction`.
Because both flavors flow through the **one** `AvailableReactions` list → single
`AttackStep.ChosenReaction` slot → `CanDeclareReaction` free-reaction gate → `markPCReactionUsed`
consumption, the one-reaction-per-round economy is enforced for free (a PC can't stack Defensive
Duelist *and* Uncanny Dodge on the same attack). Consumed in `ExecuteEnemyTurn`
(`turn_builder_handler.go`): when the chosen reaction is `HalveDamage`, the pre-rolled damage is
halved via the already-unit-tested `ApplyUncannyDodge` (`feature_integration.go`, `dmg/2`) **before**
it is staged into `pendingHit`/written to HP — forward-only, no full-damage-then-heal-back (honors
`feedback_reaction_predeclare_no_retroactive`). Seed: `uncanny_dodge` added to Rogue
`features_by_level["5"]` (2024 L5), mirroring the COV-3 Evasion seed at L7; level-gated by
`derive_stats`, no migration/test-hook change (Go-literal seed). The new `HalveDamage` field rides
the plan JSON, so the dashboard Turn Builder renders the button and the execute request
deserializes it with **zero new Discord/dashboard code**.

**RAW correctness (found during `/simplify` altitude review).** Uncanny Dodge triggers only "when
an attacker hits you," so a declared halving reaction against an attack that **misses** is now
dropped in `ExecuteEnemyTurn` before the consume/announce step — it is neither spent (stays
available) nor written to the combat log. A +AC reaction is still consumed regardless, since it was
applied at roll time to decide the hit. The `ReactionOption` doc comments were also de-leaked from
"pre-roll reaction window" to "declared against an incoming attack" (the two flavors resolve at
different times: +AC at roll time, halving post-hit).

Tests: `reactions_test.go` (builder present/absent; `AvailableReactions` includes it for a
feature-carrying PC; `FormatReactionDeclared` halve-line, no `+AC`), `turn_builder_handler_test.go`
(execute halves 8→4 **before** the HP write + marks used; **not** consumed/announced on a miss),
`refdata_integration_test.go` (`TestIntegration_SeedRogueUncannyDodgeFeature` locks the L5 seed→present
link, mirroring the Evasion guard). Coverage: gates met (combat 91.4%, discord 86.0%, refdata 97.92%).

**Deferred follow-ups (new items when picked up):**
- **Live `/attack` defender prompt** — the mid-`/attack` interactive path is not wired. Every
  currently-wired `populatePostHitPrompts` hook (Divine Smite, GWM, Stunning Strike) is *attacker*-side;
  there is no general *defender*-post-hit-prompt mechanism yet (the `UncannyDodgePromptArgs`/
  `PromptUncannyDodge` scaffold in `internal/discord/class_feature_prompt.go` is unwired). When that
  defender-prompt lane is built, it calls the **same** `ApplyUncannyDodge` + `HalveDamage` consumption,
  halving before its own HP write. Enemy-turn-first was the right order; this slice leaves the math +
  consumption reusable for it.
- **Shield Master's DEX-save damage-evasion half (COV-9)** — **DONE 2026-07-05**, but it ended up
  mirroring **COV-3 Evasion** (`ApplyInterposeShield`/`combatantHasInterposeShield` at the
  `ResolveAoESaves` save chokepoint), NOT this `HalveDamage` attack-path reaction: its trigger is a DEX
  save, which lives on the save-resolution path (no reaction surface), so the passive-style auto-apply
  fit there. Its RAW reaction cost is deferred to the same future save-path reaction lane this item's
  live-`/attack` prompt awaits.
- **Offered on a pre-rolled miss** — `AvailableReactions` still surfaces Uncanny Dodge roll-agnostically
  (its signature doesn't see the roll), same as Defensive Duelist; harmless now that execute drops an
  untriggered one, but gating the *offer* on the pre-rolled hit would tidy the Turn Builder UI.

---

**Original problem (for reference):**

**Problem.** `UncannyDodgeFeature()` (`feature_integration.go:139`) emits
`EffectReactionTrigger{On:"uncanny_dodge"}` into `ProcessorResult.ReactionTriggers`
(`effect.go:393`), a slice with **no production reader**. `ApplyUncannyDodge`
(`feature_integration.go:154`, `dmg/2`) has zero live callers. So a Rogue 5+ never halves
an incoming hit. The `uncanny_dodge` slug is also **not seeded** (deliberately — see COV-3).

**Why it isn't COV-3's shape.** The wired reaction system (`reactions.go`) models only
**pre-roll +AC** reactions: `ReactionOption{ACBonus}` folded into the attack, re-evaluated
via `applyReactionToRoll` (hit→miss only, damage untouched). Uncanny Dodge is a **post-hit
damage halving** — it triggers *after* a hit is confirmed and reduces that attack's damage.
There is no damage-reduction reaction slot today.

**Constraint (memory `feedback_reaction_predeclare_no_retroactive`).** No retroactive
resolution / no post-hit heal-back. The halving must reduce damage **before** it is written
to HP — not apply full then refund. The enemy-turn plan already **pre-rolls** each attack
and applies damage at execute time (Turn Builder), so halving the pre-rolled damage at
execute (before the HP write) is compliant; a mid-attack interactive prompt during a live
`/attack` would need the post-hit prompt pattern (`class_feature_prompt.go`) with the same
"halve before write" ordering.

**Sketch.**
1. Seed `uncanny_dodge` into Rogue `features_by_level["5"]` (2024 L5) — mirror the COV-3
   Evasion seed; gated by `derive_stats.go:223`.
2. Add a damage-reduction reaction flavor: either extend `ReactionOption` with a
   `DamageMultiplier`/`Halve` field, or a parallel `AvailableDefensiveReactions` for
   post-hit reactions. Gate: PC target, Rogue 5+ (`uncanny_dodge` feature present), reaction
   free (`CanDeclareReaction`), attacker visible.
3. Consume it in the enemy-turn execute loop (halve the pre-rolled damage of the chosen
   attack before `ApplyDamage`) and mark the reaction used (`markPCReactionUsed`). For live
   `/attack`, offer via the post-hit prompt and halve before the HP write.
4. Discord: reaction prompt for the targeted PC (mirror the Defensive Duelist / Turn Builder
   reaction UX).

**Mirror.** `ApplyUncannyDodge` (the math, already coded + unit-tested), Defensive Duelist
reaction plumbing (`reactions.go`, `turn_builder_handler.go:308-320`), post-hit prompt
pattern (`class_feature_prompt.go` + `discord/attack_handler.go:444-537`). Reuse the COV-3
`combatantHasEvasion` shape for a `combatantHasUncannyDodge` feature lookup.

**Acceptance.** A Rogue 5+ hit by a visible attacker may declare Uncanny Dodge; the
triggering attack's damage is halved once per round; the reaction is consumed; no
heal-back (damage written already halved). Red test first: reaction offered → chosen →
damage halved before HP write.

---

## Tier 4 — Stale 2024 data (rules drift, no engine change)

### COV-10 — `features_by_level` only seeds levels 1–3
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/refdata`

Every class's `features_by_level` populates only L1–3 (plus one subclass), so **all
higher-level 2024 signature features are absent from the data model**, not just the engine:
Brutal Strike (L9), Tactical Master (L9), Studied Attacks (L13), Cunning Strike (L5), etc.
**This is the blocker under COV-3 and COV-8.** Extend the seed to the levels those items need
(don't have to seed all 20 at once — seed the levels you wire).

### COV-11 — Subclass unlock levels pre-2024
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/refdata`

2024 standardizes every subclass to **L3**. Seed still has: Cleric **L1**, Druid **L2**,
Sorcerer **L1**, Warlock **L1**, Wizard **L2** (`seed_classes.go:105/139/352/…/417`).
Confirm the product intends 2024 rules before changing (may be deliberate for early
subclass identity). If yes: bump `SubclassLevel` to 3 for those five and move the seeded
subclass feature accordingly.

### COV-12 — True Strike seeded as 2014 cantrip
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/refdata`

Seeded as the 2014 concentration cantrip granting advantage (`seed_spells_cantrips.go:30`,
also carries `concentration:true`). 2024 True Strike is a **weapon-attack cantrip** that
scales radiant damage and uses the spellcasting ability. Re-seed to 2024 shape.

### COV-13 — Thunder Step departure damage is a string, not resolved
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/combat`

Thunder Step's `additional_effects:"3d10 thunder to creatures within 10ft of departure"`
(`seed_spells_3.go:46`) is printed (`spellcasting.go:279`) but **no saves/damage** are
applied to bystanders, even though the spell's top-level `damage` (carried to the
destination) *is* rolled. Resolve the departure-point AoE.

### COV-14 — Eldritch Blast modeled as single projectile, not multi-beam
**Status:** OPEN · **Severity:** low-medium · **Pkg:** `internal/combat`

EB is `"1d10"` + `cantrip_scaling`, so at L5 it scales to a single `2d10` roll on one attack
rather than 2 separate beams (separate attack rolls, separate targets). Only the Agonizing
Blast *bonus* multiplies by beam count (`agonizing_blast.go:35`). Correct multi-beam requires
N attack rolls at levels 5/11/17. **Blocks per-beam Repelling Blast (COV-6).**

### COV-15 — Fighting Style / Metamagic not enforced end-to-end
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/portal` (builder)

The combat engine *can* apply specific fighting styles (archery/defense/dueling/GWF,
`feature_integration.go:399-406`) and specific metamagic (`metamagic.go`), but the **builder
only writes the generic `choose_fighting_style` / `choose_2_metamagic_options` placeholder**
(`seed_classes.go:163/348`) — the player's actual pick is never injected as a feature.
Mirror `injectClassFeatureChoices` (`builder_service.go:563`), which already materializes
pact-boon/invocation/expertise picks; add fighting-style + metamagic picks the same way.

---

## Tier 5 — Builder-rebuild drops persisted overlays (live state reset on edit)

### COV-17 — A builder edit silently wipes feats, feat-HP, and expended feature-uses
**Status:** ✅ COMPLETE 2026-07-05 (S1 + S2 + S3 all DONE) · **Severity:** high (S1) / low (S2) / medium (S3) · **Pkg:** `internal/portal` (+ `internal/character` for S2)

**Problem.** Any builder edit runs `UpdateCharacterRecord`
(`builder_store_adapter.go:344`), which **rebuilds the character from a fresh derivation**
and overwrites most columns. It preserves *some* live state by reading the `existing` row —
HPCurrent (capped), TempHP, spell slots, pact slots, gold, attunements (`:361-392`) — but
several persisted overlays are **regenerated fresh and lost**:
1. **ASI-applied feats** — `Features` (`:384`) is overwritten with `CollectFeatures` output,
   which regenerates **class/subclass/racial only** (`derive_stats.go:210`). Every feature the
   level-up ASI flow wrote with `Source:"feat"` (`levelup/service.go:437`) — Durable, Tough,
   Alert, Savage Attacker, Sharpshooter, Polearm/Crossbow/Shield Master, … — **vanishes**,
   silently disabling every combat rider keyed on `HasFeatureByName`. This quietly guts the
   entire COV-9 feat effort the instant a player re-saves the builder.
2. **Tough's flat +2/level HP** — `HPMax` (`:370`) is the pure `CalculateHP` derivation, which
   is feats-agnostic; there is no post-pass re-adding flat feat HP. (Note: **CON-feat HP is
   fine** — ability scores are preserved via `submissionFromCharacter` `:543-547`, so the
   rebuilt `CalculateHP` uses the bumped CON. Only the *flat* feat bonus is lost.)
3. **Expended feature-uses** — `FeatureUses` (`:383`) is `InitFeatureUses(...)` fresh at max
   (`:161`), with **no expended-preservation merge** — so a builder edit refills rage / ki /
   channel-divinity / lay-on-hands / bardic / action-surge / second-wind / sorcery / wild-shape
   pools to full, erasing what was spent this day.

**Verified.**
- `CollectFeatures` (`derive_stats.go:210`) reads only class/subclass/racial ref data; never the
  persisted row. `submissionFromCharacter` (`builder_store_adapter.go:560`) restores **only**
  pact-boon/invocation picks (`classFeatureChoicesFromFeatures`), dropping all other feats.
- `CharacterSubmission` (`builder_service.go:39-70`) has **no** feat field.
- Pact/invocation features carry `Source:"invocation"` / `"pact_boon"` (`invocations.go:17-18`),
  **not** `"feat"` — so a `Source=="feat"` filter is safe and won't double-add them (they are
  regenerated via the submission path).

**Mirror (shared by all three slices).** `preserveExpendedSlots` / `preserveExpendedPactSlots`
(`builder_store_adapter.go:402/434`): read `existing`, merge the live delta into the fresh value
at persist time, one helper called at the `UpdateCharacterRecord` write. The store adapter already
holds `existing.Features` / `existing.FeatureUses` (`:349`). This single chokepoint catches **all**
rebuild paths — no submission-schema or service-layer change needed. Preservation tests to mirror:
`TestBuilderStoreAdapter_UpdateCharacterRecord_PreservesLiveState` (`:1500`),
`…_PreservesExpendedPactSlots` (`:1541`), `…_CapsHPToNewMax` (`:1578`).

---

**Slice S1 — Preserve `Source:"feat"` features. ✅ DONE 2026-07-05.**
- Shipped `preservePersistedFeats(existing, fresh)` (`builder_store_adapter.go`, next to the
  `preserveExpended*` siblings): unmarshal existing, filter `Source == featFeatureSource`, append
  each to the fresh list de-duped by `Name`, re-marshal. Early-returns `fresh` when existing is
  absent / feat-less / unparseable, and when `fresh` is invalid it still carries the feats forward
  onto a fresh list. Wired at the `Features:` field of the `UpdateCharacter` write.
- Added `featFeatureSource = "feat"` const (`invocations.go`, beside `invocationFeatureSource` /
  `pactBoonFeatureSource`) so the source tag is no longer a bare literal — `/simplify` house-style fix.
- Idempotent: `CollectFeatures` never emits feats, so fresh has none → append-once per rebuild.
  Carries each feat's full struct incl. `MechanicalEffect`, so combat riders survive verbatim.
- Test `…_PreservesFeatFeatures` (red first): seeds a `Source:"feat"` Durable feature, runs
  `UpdateCharacterRecord`, asserts the feat + its MechanicalEffect survive and no name is duplicated.
- `/simplify` (4 agents): all CLEAN — persist seam is the right altitude (feats are levelup-owned,
  preserved like expended slots, NOT threaded through the submission), no reuse/efficiency/complexity
  hits. A generic `preserve*` extraction was judged premature until S2/S3 land.

**Slice S2 — Re-add flat feat HP on rebuild. ✅ DONE 2026-07-05.**
- Lifted the flat-HP rule into a cross-package SSOT: new `character.FeatFlatHPBonus(features
  []character.Feature, totalLevel int32)` + exported `character.FeatFlatHPPerLevelSlug`
  (`internal/character/feat_hp.go`, next to `CalculateHP`). Sums +2/level per feat feature whose
  serialized `MechanicalEffect` carries the slug; empty/unparseable → 0.
- `levelup.ApplyFeat` now calls `character.FeatFlatHPBonus([]character.Feature{feature}, char.Level)`
  on the `character.Feature` it just built (its `MechanicalEffect` round-trips the slug via
  `specializeFeatEffects`, which copies `effect_type` verbatim) — the old `featMaxHPBonus` +
  `hpPerLevelEffectSlug` const deleted from `levelup/feat_hp.go`. Both writers now share ONE rule, so
  they can no longer diverge on representation (that divergence was the original portal bug).
- `portal.UpdateCharacterRecord` adds `featFlatHPBonus(existing.Features, c.level)` to the
  feats-agnostic `c.hpMax` (thin `pqtype.NullRawMessage` unmarshal wrapper delegating to the SSOT);
  `hpCurrent` caps to the re-added max. **CON-feat HP already survived** (preserved ability scores),
  so only the flat bonus needed re-adding.
- Test `…_PreservesFeatHP` (red first): Tough char (persisted `[{"effect_type":"hp_plus_2_per_level"}]`),
  fresh derivation 30, level 4 → HPMax stays 38. Plus `character.TestFeatFlatHPBonus` unit table
  (scales/sums/non-hp/empty/unparseable/none). Shipped Tough `ApplyFeat` integration tests unchanged +
  green.
- `/simplify` (4 agents): all CLEAN on all 4 axes — `internal/character` is the right SSOT home (no
  cycle; both callers already import it), the `character.Feature` representation is the correct
  unification (not coincidence), the two-fn split + portal adapter are justified, double-parse of
  `existing.Features` is acceptable house style. One required fix applied: the stale `ApplyFeat`
  comment claiming feat HP "is lost on a builder rebuild" (false after S1+S2) now points at the two
  preserve paths.

**Slice S3 — Preserve expended feature-uses on rebuild. ✅ DONE 2026-07-05.**
- Shipped `preserveExpendedFeatureUses(existing, fresh)` — a line-for-line mirror of
  `preserveExpendedSlots` over `map[string]character.FeatureUse`: for each pool present in both, carry
  the spent delta (`expended = max(oldMax-oldCurrent,0)`; `newCurrent = max(newMax-expended,0)`) onto
  the freshly-`InitFeatureUses`'d max, `Recharge`/`Max` taken from the fresh derivation. Wired at the
  `FeatureUses:` field of `UpdateCharacter`.
- Covers rage / ki / channel-divinity / lay-on-hands / bardic / action-surge / second-wind /
  wild-shape / sorcery-points — all previously refilled to full on any builder edit.
- Test `…_PreservesExpendedFeatureUses` (red first): barbarian who spent 1 of 2 rage uses keeps 1
  after a non-mechanical edit (not 2). Shape-change edge (rage 2→3 on level-up → keeps the burned use,
  grants the new headroom = "3 max, 2 left") is the same semantics the slot sibling already applies.
- `/simplify` (4 agents): all CLEAN. Generic `preserveExpended*` extraction **decisively rejected** —
  rule-of-three is a false positive (the pact sibling is a *scalar*, not a `map[string]T`, so only two
  true instances), and Go's lack of structural typing means a `preserveExpendedMap[T]` needs
  getter/setter closures at each call site that cost more than the ~12 duplicated lines. Triple
  duplication is the correct altitude; revisit only if a 4th map-shaped pool appears.
- **Pre-existing, NOT this slice (noted by altitude agent):** `InitFeatureUses` multiclass Cleric+Paladin
  both write the same `FeatureKeyChannelDivinity` map key → one overwrites the other. The merge
  faithfully mirrors whatever `InitFeatureUses` emits and does not worsen it. Flag separately if
  multiclass Channel Divinity ever matters.

**Relationship.** All three shared the one persist-time seam and the `preserveExpended*` mirror.
**All DONE 2026-07-05** — a builder edit no longer wipes ASI feats (S1), drops Tough's flat HP (S2),
or refills expended feature-use pools (S3). COV-17 CLOSED.

---

## Quick verification commands

```sh
# Which feat/mastery slugs the engine actually consumes:
grep -n "case \"" internal/combat/feature_integration.go

# Confirm a spell field is/ isn't read in combat:
grep -rn "ConditionsApplied" internal/combat --include="*.go" | grep -v _test

# Wired feats (by name/slug branches):
grep -rn "HasFeatureByName\|HasFeat(" internal/combat --include="*.go" | grep -v _test

# Build + gates:
make cover-check   # 90% overall / 85% per-pkg
make sqlc-check    # if you touched .sql queries
```

## Suggested pickup order

1. ~~**COV-1 + COV-2** (Tier 1) — makes ~20 save/condition spells actually do something.~~
   **DONE 2026-07-04.** Save damage + save-or-suck conditions both land through the shared
   resolver. Follow-ups (per-turn re-saves, condition riders) noted inline under COV-2.
2. ~~**COV-3** Evasion~~ **DONE 2026-07-04** — wired end-to-end (the "engine already works"
   premise was false; it needed a real `ResolveAoESaves`→`ApplyEvasion` consumer). Uncanny
   Dodge split to **COV-16** (needs a new post-hit damage-halving reaction).
2b. ~~**COV-4** (Second Wind)~~ **DONE 2026-07-04** — `/bonus second-wind` self-heal
   `1d10 + level`, mirrors Lay on Hands; also fixed the L1-vs-L2 seed gate. Use-count
   scaling (2/3/4 per 2024) deferred inline under COV-4.
3. **COV-10** — unblocks COV-8; seed the levels you need as you wire each martial rider.
4. ~~**COV-5** (Hunter's Mark)~~ **DONE 2026-07-04** — on-hit 1d6-force rider wired as a Hex
   mirror (`spell_marker.go` shared helpers); free-cast pool deferred inline.
4b. ~~**COV-6** (invocations, EB-rider slice)~~ **DONE 2026-07-04** — Repelling Blast (push via
   `applyPushEffect`) + Eldritch Spear (300 ft range) wired as EB-cantrip riders
   (`eldritch_blast_invocations.go`). Per-beam push blocked on **COV-14**. **COV-9** (top feats)
   still parallelizable, each mirrors a wired template.
4c. ~~**COV-7** (Pact of the Blade)~~ **DONE 2026-07-04** — pact-weapon attacks use CHA
   (`pact_blade.go`, `effectiveAbilityMod`); no seed change (boon slug already persisted).
4d. ~~**COV-6 `lifedrinker` + `thirsting_blade`**~~ **DONE 2026-07-04** — unblocked by COV-7:
   Lifedrinker = flat +CHA necrotic on-hit rider (`LifedrinkerFeature`), Thirsting Blade = 2nd
   attack (`max(_,2)` in `resolveAttacksPerAction`). Chain/Tome boons + off-hand pact-weapon path
   still deferred. Remaining open mirrors: **COV-9** (top feats).
4e. ~~**COV-16** (Uncanny Dodge)~~ **DONE 2026-07-05** — first post-hit damage-halving reaction on the
   enemy-turn Turn Builder path: `ReactionOption.HalveDamage` + `uncannyDodgeReaction` builder +
   `ExecuteEnemyTurn` halve-before-write over the existing `ApplyUncannyDodge`; Rogue L5 seed. Live
   `/attack` defender-prompt path deferred (no defender-post-hit-prompt lane yet).
4f. ~~**COV-9 Shield Master Interpose Shield**~~ **DONE 2026-07-05** — DEX-save damage evasion (take no
   damage on a made DEX save-for-half + shield) wired as a **COV-3 Evasion mirror** at `ResolveAoESaves`
   (`ApplyInterposeShield` + `combatantHasInterposeShield`, priority `switch` with Evasion dominating).
   RAW reaction cost/economy auto-simplified + deferred to a future save-path reaction lane. Shield
   Master's +shield-AC-to-DEX-saves rider still open.
5. Tier 4 data fixes (COV-11..15) — low risk, do alongside related feature work.
6. ~~**COV-17 (Tier 5) — builder-rebuild overlay preservation.**~~ **COMPLETE 2026-07-05** —
   all 3 slices shipped: ASI-applied feats (`preservePersistedFeats`, S1), Tough flat +2/level HP
   (`character.FeatFlatHPBonus` SSOT re-added on rebuild, S2), and expended feature-use pools
   (`preserveExpendedFeatureUses`, S3) now all survive a builder edit.
