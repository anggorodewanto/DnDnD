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
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/combat`

**Problem.** 29 invocations are catalog-defined (`refdata/invocation_catalog.go`) and
builder-pickable (ISSUE-060), but **only `agonizing_blast` is combat-wired**
(`combat/agonizing_blast.go`). `repelling_blast` (push on EB hit), `lifedrinker` (+necrotic),
`eldritch_spear` (range), `thirsting_blade` (extra attack) are inert.

**Mirror.** `agonizing_blast.go` (reads the invocation off the character, modifies the EB
resolution). Repelling Blast reuses `applyPushEffect` (`mastery.go:191`). Thirsting Blade
reuses the extra-attack path.

**Depends on:** `repelling_blast` per-beam push needs COV-9 (multi-beam EB) to target beams
individually; the others don't.

**Acceptance.** Each newly-wired invocation changes EB/attack resolution as written; red
test per invocation.

---

### COV-7 — Pact Boons have no combat consumer
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/combat`

**Problem.** Pact boons are builder-pickable but inert — `invocation_catalog.go:45`:
"Pact boons have no mechanical combat consumer yet." Pact of the Blade (summon/attack with
pact weapon, use CHA), Pact of the Chain (familiar), Pact of the Tome (extra cantrips).

**Mirror.** Blade's CHA-based attack ≈ existing attack path with an ability override; Tome's
extra cantrips ≈ builder grant already done for invocations. Scope per-boon.

**Acceptance.** At minimum Pact of the Blade lets the warlock attack with the pact weapon
using CHA. Chain/Tome may be builder-only + noted.

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
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/combat` + `internal/refdata`

**Wired today:** GWM, Sharpshooter, Defensive Duelist, Crossbow Expert (partial),
Tavern Brawler, Dual Wielder.

**Description-only, no combat effect** (in `seed_feats.go`, matched by neither name nor
slug in combat):

| Feat | Effect to wire | Mirror |
| --- | --- | --- |
| Polearm Master | butt-end bonus attack + reach opportunity attack | GWM bonus-attack prompt (`class_feature_prompt.go`); reach OA needs new trigger |
| Sentinel | OA on disengage/attack-others; hit sets speed 0 | reaction window `reactions.go` |
| Shield Master | bonus-action shove; DEX-save damage evasion | mastery push `applyPushEffect`; save rider |
| Savage Attacker | reroll weapon damage once/turn | once/turn damage rider in `populateAttackFES` |
| War Caster | advantage on concentration saves; cast as OA | concentration save hook `concentration.go:324` |
| Charger / Mobile / Alert / Lucky / Mage Slayer / Heavy Armor Master / Tough | movement / init / reroll / damage-reduction riders | various — scope each |

**Also:** Crossbow Expert's **bonus-action hand-crossbow attack** is not wired (only its
loading-ignore + no-disadvantage-in-melee are).

**Note:** `feat.MechanicalEffect` JSON in seed is descriptive metadata only — combat does
**not** parse it. Wiring a feat = add a name/slug branch in the effect pipeline, same as the
6 wired feats. Pick the high-impact ones first (Polearm Master, Sentinel, Shield Master).

---

### COV-16 — Uncanny Dodge: post-hit damage-halving reaction (split from COV-3)
**Status:** OPEN · **Severity:** low-medium · **Pkg:** `internal/combat` + `internal/discord` (+ seed)

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
   mirror (`spell_marker.go` shared helpers); free-cast pool deferred inline. **COV-6**
   (invocations), **COV-9** (top feats), **COV-16** (Uncanny Dodge) — parallelizable, each
   mirrors a wired template.
5. Tier 4 data fixes (COV-11..15) — low risk, do alongside related feature work.
