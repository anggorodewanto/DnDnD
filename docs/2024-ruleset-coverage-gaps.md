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
- Wired spell effects: spell-attack damage, AoE save damage, healing, teleport
  (self / self+creature), agonizing-blast EB, Invisibility, Hex, Fly, Spare the Dying,
  zone spells (Spirit Guardians, Wall of Fire, Fog Cloud, Darkness, Silence, Moonbeam…),
  Counterspell, Divine Smite.

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
**Status:** OPEN · **Severity:** high · **Pkg:** `internal/combat` (+ maybe seed)

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

### COV-3 — Evasion / Uncanny Dodge never emitted (near-free)
**Status:** OPEN · **Severity:** low · **Pkg:** `internal/refdata` (seed) + verify combat

**Problem.** `FeatureDefinition`s for both are **already coded**
(`feature_integration.go:110` Evasion, `:139` Uncanny Dodge) but never fire, because
`seed_classes.go` only populates Rogue features for levels 1–3, so the `evasion` /
`uncanny_dodge` `mechanical_effect` slugs are never present on a character.

**Fix.** Add the slugs to Rogue's `features_by_level` at the correct levels (Evasion L7,
Uncanny Dodge L5 in 2024) in `seed_classes.go`. Engine side already works.

**Acceptance.** A L7 rogue takes half/none on a made DEX-save AoE (Evasion); a L5 rogue
halves one attack's damage via reaction (Uncanny Dodge). Red test: seed → feature present
→ effect fires.

**Risk.** Needs the L1–3-only `features_by_level` limitation addressed at least for these
two levels (see COV-10). Small.

---

### COV-4 — Second Wind: pool seeded, no spend command
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/combat`

**Problem.** The Second Wind use-pool is seeded and rest-recharged
(`init_feature_uses.go:43`) but there is **no combat command to spend it** — no
`second-wind` consumer anywhere in `internal/combat/`.

**Mirror.** `internal/combat/lay_on_hands.go` (spend a pool, heal, as a bonus action) and
`action_surge.go`. Copy the Lay on Hands command shape: bonus action, spend one use, heal
`1d10 + fighter level`.

**Acceptance.** `/second-wind` (or Turn-Builder bonus action) heals 1d10+level once per
short rest, decrements the pool, blocked when pool empty / not a bonus action available.

---

### COV-5 — Ranger free Hunter's Mark (2024 Favored Enemy)
**Status:** OPEN · **Severity:** medium · **Pkg:** `internal/combat` (+ seed)

**Problem.** 2024 Favored Enemy = a number of free Hunter's Mark casts. Seed still carries
the 2014 text (`seed_classes.go:271` "advantage on tracking"). Hunter's Mark exists as a
spell seed but has **no on-hit rider and no free-cast**, unlike Hex which is fully wired.

**Mirror.** Hex is a near-exact template: `internal/combat/hex.go` (on-hit +1d6 rider,
source-scoped `hexed` tag, cleared on concentration end) + `HexFeature`
(`feature_integration.go:341`) + cast branch (`spellcasting.go:830`). Hunter's Mark is the
same shape with a `hunters_mark` tag and 1d6 rider; add the free-cast pool for Favored Enemy.

**Acceptance.** Ranger casts Hunter's Mark; subsequent weapon hits on the marked target add
1d6; concentration end clears it; Favored Enemy grants N free casts/day.

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

1. **COV-1 + COV-2** together (Tier 1) — highest leverage; makes ~20 save/condition spells
   actually do something. One coupled workstream.
2. **COV-3** (Evasion/Uncanny Dodge) + **COV-4** (Second Wind) — near-free, engine ready.
3. **COV-10** — unblocks COV-8; seed the levels you need as you wire each martial rider.
4. **COV-5** (Hunter's Mark), **COV-6** (invocations), **COV-9** (top feats) — parallelizable,
   each mirrors a wired template.
5. Tier 4 data fixes (COV-11..15) — low risk, do alongside related feature work.
