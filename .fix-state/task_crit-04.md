# task crit-04 — FES never reaches `Attack()` (input.Features unset; EffectContext zero)

## Finding (verbatim from chunk4_conditions_classes.md, Phase 45)

> `attack.go:456` reads `input.Features`, but no caller ever assigns it. `BuildFeatureDefinitions` is referenced only from tests + `internal/magicitem/integration_test.go:183`.
> `BuildAttackEffectContext` is invoked at `attack.go:457` with only `Weapon` populated — `HasAdvantage/AllyWithinFt/WearingArmor/OneHandedMeleeOnly/IsRaging/AbilityUsed/UsedThisTurn` are all zero. Even if Features were wired, Sneak Attack's `AdvantageOrAllyWithin: 5` filter would never pass in production.
> `ApplyEvasion`, `ApplyUncannyDodge` have no callers in production code.
> `ApplyGreatWeaponFighting` has no callers (the damage-roll path in `attack.go` uses `dice.Roller` directly with no reroll-on-1s-and-2s hook).

Spec sections: "Feature Effect System" (Phase 44/45 in `docs/phases.md`), spec lines 1521-1528.

Recommended approach (chunk4 follow-up #1): "Wire `Features` and the `EffectContext` through `Attack()`. Inside `s.Attack()` (`attack.go:773`), build `BuildFeatureDefinitions(classes, charFeatures, magicItemDefs)` and assign `input.Features = ...`. Populate `BuildAttackEffectContext` with the real `IsRaging` (from combatant), `HasAdvantage` (after `DetectAdvantage`), `AllyWithinFt` (compute from grid), `WearingArmor` (from char.EquippedArmor), `OneHandedMeleeOnly`, `AbilityUsed` (str/dex), and `UsedThisTurn` (from per-turn tracking). After this, Sneak Attack, Rage damage, fighting styles, Pack Tactics start to work."

## Plan

1. Add the missing context fields to `AttackInput` (`IsRaging`, `WearingArmor`, `OneHandedMeleeOnly`, `AllyWithinFt`, `AbilityUsed`, `UsedThisTurn`).
2. In `ResolveAttack`, swap the order so `DetectAdvantage` runs *before* the Feature Effect System lookup — that way `HasAdvantage` is the post-cancellation resolved mode and Sneak Attack's `AdvantageOrAllyWithin` filter actually fires. Roll the resulting `ProcessorResult.ExtraDice` into `DamageTotal` (with crit doubling) so Sneak Attack damage is realised.
3. Add a `s.populateAttackFES` helper that:
   - reads `IsRaging` from the combatant,
   - computes `AbilityUsed` via a new `attackAbilityUsed` mirror of `abilityModForWeapon`,
   - decides `OneHandedMeleeOnly` from weapon properties + two-handed grip + off-hand state,
   - reads `WearingArmor` from `char.EquippedArmor`,
   - lists the encounter's combatants, then derives `AllyWithinFt` via a new `nearestAllyDistanceFt` (same-side, alive, ignoring self+target),
   - calls `BuildFeatureDefinitions(classes, feats)` to fill `Features`.
4. Call the helper from both `Service.Attack` and `Service.attackImprovised` after obscurement is resolved, so the data flows into `ResolveAttack`.
5. TDD: red→green with two end-to-end Service.Attack tests (raging Barbarian gets +2 damage; Rogue with prone target gets sneak attack 3d6) plus negative control + helper coverage.

## Files touched

- `internal/combat/attack.go` — added `AttackInput` fields; restructured `ResolveAttack` (advantage detection first, FES second, extra dice rolled into damage); added `rollFESExtraDice`, `attackAbilityUsed`, `nearestAllyDistanceFt`, `populateAttackFES`; wired both `Service.Attack` and `Service.attackImprovised` to call `populateAttackFES`.
- `internal/combat/attack_fes_test.go` — new file with the integration tests + helper tests.
- `.fix-state/log.md` — appended worker note (deferred items only — no code).

## Tests added

In `internal/combat/attack_fes_test.go`:

- `TestServiceAttack_RagingBarbarian_AddsRageDamageBonus` — end-to-end: a level-5 raging Barbarian with longsword scores 1d8(5) + STR(+3) + Rage(+2) = 10. Was 8 before the fix.
- `TestServiceAttack_NonRagingBarbarian_OmitsRageDamageBonus` — negative control: same fixture with `IsRaging=false` → 8 (no bonus).
- `TestServiceAttack_Rogue_SneakAttackFiresOnAdvantage` — end-to-end: level-5 Rogue with rapier vs prone target within 5ft. Prone-within-5ft grants advantage → sneak attack 3d6 (each die fixed at 5) added to 1d8(5) + DEX(+3) = 23. Was 8 before the fix.
- `TestBuildAttackEffectContext_WiresAllRequestedFields` — contract test enumerating every chunk-4-listed flag.
- `TestAttackAbilityUsed_AllPaths` — table-driven helper test (finesse, ranged, monk, melee non-finesse).
- `TestCountAlliesWithinFt`, `TestNearestAllyDistanceFt_DeadOrSelfExcluded` — helper coverage (nearest ally + dead/self exclusion + sentinel).

`go test ./internal/combat/... ./internal/magicitem/...` green, `make cover-check` green (combat 94.09%, magicitem 100%).

## Implementation notes

- **Why DetectAdvantage moved before FES:** the previous order called `BuildAttackEffectContext` with only `Weapon` populated, so any FES filter that needed `HasAdvantage` (Sneak Attack, Pack Tactics) silently no-op'd. Swapping the order is a precondition for any of the chunk-4 fixes to take effect end-to-end.
- **HasAdvantage = post-cancellation:** I set `HasAdvantage: rollMode == dice.Advantage`, so cancelled adv+disadv = normal roll = no Sneak Attack via "with advantage". This matches RAW (the cancellation rule applies before consulting "you have advantage").
- **Magic items deferred:** `magicitem.CollectItemFeatures` already imports `combat`, so combat can't reciprocate without breaking the build. The chunk's Recommended #1 line "magicItemDefs" is left as a follow-up — adding magic-item features will require either a higher-layer orchestrator (cmd/dndnd) or relocating the InventoryItem/AttunementSlot types into combat. Documented in `log.md`.
- **OncePerTurn filter:** `Effect.Conditions.OncePerTurn` only blocks when `ctx.UsedThisTurn[type]` is true. There is no per-turn feature-usage column on `turns`, and adding one is out of scope. I pass an empty map, which keeps the filter permissive — a multi-attack action could fire Sneak Attack twice in one turn until per-turn tracking lands. Documented as a follow-up.
- **NPC vs PC ally partition:** `nearestAllyDistanceFt` treats `IsNpc` as the side discriminator. For Pack Tactics on monsters, this is exactly right: a goblin's ally is another NPC. For PC squads with PC summons, summons today don't carry an `IsNpc=false` marker; they show up as NPC, which is consistent with the existing `Hostile` model. If summons later need to count as PC allies, that's a separate fix.
- **ExtraDice critical doubling:** `Roller.RollDamage(expr, true)` already doubles dice counts in-place, so `rollFESExtraDice(extra, true, roller)` correctly doubles e.g. Sneak Attack 3d6 → 6d6 on a crit, matching RAW.
- **OneHandedMeleeOnly logic:** true iff the weapon is melee, not "two-handed", not currently wielded with versatile two-handed grip, and the off-hand is empty. Matches the Dueling fighting style spec.
- **`AttackResult.DamageDice` not updated for FES extra dice:** the human-readable damage dice string still reflects only the base weapon dice. Updating the display string to show "+ 3d6 sneak attack" is a UI concern; the numeric DamageTotal is correct. Left as a follow-up when /attack handler renders the line.
- **`charPtr` rename in `attackImprovised`:** the existing `s, err := ParseAbilityScores(...)` shadowing inside the if-block prevented me from using `char` directly outside, so I aliased to `charPtr` for clarity.
- **No production-code uses of `ApplyEvasion` / `ApplyUncannyDodge` / `ApplyGreatWeaponFighting`:** intentionally not addressed; per task constraints these are separate tasks.

## Review (reviewer fills) — Verdict: PASS | REVISIT

## Rev 2 — Chebyshev fix

Reviewer flagged that `nearestAllyDistanceFt` was using `combatantDistance` (Euclidean rounded to nearest 5ft) where it should match /move pathfinding (Chebyshev × 5ft, diagonals=5ft). Fix landed in this revision:

- `internal/combat/attack.go:1199-1230` — `nearestAllyDistanceFt` now computes `max(|dc|,|dr|) * 5` directly, matching `pathfinding.go:163-171` and `opportunity_attack.go:165,172`. Z-axis ignored (FES "ally within Xft" filters are 2D adjacency checks, per added godoc).
- `internal/combat/attack_fes_test.go:307-321` — `TestCountAlliesWithinFt` farAlly now asserts 20ft (was 25ft); inline comment explains the Chebyshev derivation. The dead-ally and self-exclusion test was unaffected (sentinel-only assertion).

`go test ./internal/combat/...` green. (Recovered after a sub-agent API error mid-run; the function-body and test edits had already landed before the crash, so this revision documents and re-asserts.)

STATUS: READY_FOR_REVIEW

## Review

**Verdict: REVISIT**

The wiring work is solid: `Service.Attack` and `Service.attackImprovised` both call `populateAttackFES`, which populates `input.Features` via `BuildFeatureDefinitions(classes, feats)` and fills every EffectContext field the chunk-4 finding called out (IsRaging, HasAdvantage, AllyWithinFt, WearingArmor, OneHandedMeleeOnly, AbilityUsed, UsedThisTurn). `DetectAdvantage` correctly runs before the FES branch (verified at attack.go:482 vs FES at :511). `rollFESExtraDice(exprs, critical, roller)` delegates to `Roller.RollDamage` which doubles `Group.Count` in-place when `critical=true` (roller.go:106-110) — Sneak Attack 3d6 → 6d6 on crit, matches RAW. The two end-to-end tests exercise the production `Service.Attack` path (not stubs): the raging Barbarian asserts 10 damage where pre-fix would yield 8; the Rogue asserts 23 where pre-fix would yield 8. Both would have failed before the fix. Magic-item feature plumbing (cycle), OncePerTurn turn-tracking, and ApplyEvasion/UncannyDodge/GreatWeaponFighting are all explicitly deferred with rationale in the implementation notes and `log.md`. Coverage 94.1% on `internal/combat`, ≥85% threshold met.

However, one fix is required:

1. **`nearestAllyDistanceFt` uses Euclidean distance, not Chebyshev × 5ft.** The helper calls `combatantDistance` → `Distance3D` (altitude.go:13), which computes `sqrt(dx² + dy² + dz²)` and rounds to the nearest 5ft — that is Euclidean. /move pathfinding uses Chebyshev distance × 5ft (pathfinding.go:163-171, "diagonal = 5ft too"), and so does the OA reach check (`chebyshevDist` in opportunity_attack.go:165,172). The reviewer prompt explicitly designates Chebyshev as the required metric to match /move pathfinding cost. The drift is masked for the immediate 5ft-adjacency use cases (a diagonal-adjacent ally is at sqrt(50)≈7.07ft, which rounds-to-5 and still passes `AllyWithin: 5`), but for any future feature with a larger threshold (e.g. an aura at 10ft, or Pack Tactics extended), the off-diagonal answers diverge — the test itself asserts 25ft for a 4×3-square offset where Chebyshev yields 20ft. Replace `combatantDistance` here with a chebyshev helper (`max(|dc|,|dr|) * 5`, plus altitude handling if desired), and update `TestCountAlliesWithinFt`'s far-ally assertion from 25 to 20.


## Review (rev 2)

**Verdict: PASS**

Confirmed:
- `internal/combat/attack.go:1199-1230` — `nearestAllyDistanceFt` now computes `max(|dc|,|dr|) * 5` directly (no `combatantDistance` call). Inline godoc cites `/move` parity and Chebyshev rationale. Z-axis intentionally dropped, justified in the godoc.
- `internal/combat/attack_fes_test.go:316-321` — `TestCountAlliesWithinFt` adjacent ally still 5ft; far ally G6→C3 = max(4,3)*5 = **20ft** (was 25ft pre-fix). Inline comment shows the derivation. Dead/self-exclusion test (`TestNearestAllyDistanceFt_DeadOrSelfExcluded`) was sentinel-only so unaffected.
- `go test ./internal/combat/ -run 'TestCountAlliesWithinFt|TestNearestAllyDistanceFt'` → both PASS.
- Full combat package green (`go test ./internal/combat/...` ok 16.4s).
- Surface drift: none. `combatantDistance` (Euclidean Distance3D) is correctly retained for the eight non-FES callers (monk reach, lay-on-hands, opportunity range, spellcasting range, channel divinity, teleport companion) — those measure raw feet semantics, not grid-pathfinding cost. The Chebyshev metric is correctly scoped to the FES "ally within Xft" filter only.

Now matches `/move` pathfinding (`pathfinding.go:163-171`) and OA reach (`opportunity_attack.go:165,172`). Diagonals = 5ft as required.
