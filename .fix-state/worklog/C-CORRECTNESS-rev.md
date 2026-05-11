# C-CORRECTNESS bundle 3 review

Independent verification of the seven-task bundle. All hooks land as
described in the impl worklog.

## Per-task verdict

- **C-31-fall-damage-unwired — APPROVED.** `ApplyCondition`
  (`condition.go:180-189`) calls `applyFallDamageOnProne`
  (`altitude.go:135-171`) only when the new condition is `prone` AND
  `AltitudeFt > 0`. Helper rolls `FallDamage`, resets altitude via
  `UpdateCombatantPosition(altitude=0)`, then routes damage through
  `ApplyDamage` so resistances / concentration saves fire.
- **C-35-attacker-size — APPROVED.** `populateAttackContext`
  (`attack.go:1310-1320`) runs after `buildAttackInput` in all three
  entry points (`Attack` L932, `attackImprovised` L1056, `OffhandAttack`
  L1159). `resolveAttackerSize` looks up creature row for NPCs and
  defaults PCs to Medium. Non-empty command value wins.
- **C-35-hostile-near — APPROVED.** Same helper auto-fills
  `HostileNearAttacker` via `detectHostileNear`: opposite-faction +
  alive + non-incapacitated + Chebyshev ≤1 tile. Command `true` wins.
- **C-37-ammo-recovery — APPROVED.** `AmmoSpentTracker` keyed by
  (enc, combatant, ammoName); `Service.Attack` calls
  `recordAmmoForAttack` after the inventory deduction (L987).
  `EndCombat` calls `recoverEncounterAmmunition` (`service.go:907`)
  after the combatants list is materialised at L867; recovery uses
  `RecoverAmmunition(spent)` and clears the tracker for the encounter.
- **C-38-reckless-target-side — APPROVED.** `DetectAdvantage`
  (`advantage.go:116-122`) adds `"target reckless"` when the target's
  condition list contains `reckless`. `Service.Attack` calls
  `applyRecklessMarker` when `cmd.Reckless` (L982-984), writing a
  transient condition with `DurationRounds=1`,
  `ExpiresOn=start_of_turn`, `SourceCombatantID=attacker`.
- **C-40-charmed-attack — APPROVED.** `validateCharmedAttack` runs at
  the top of `Service.Attack` (L840) and `Service.OffhandAttack`
  (L1093); improvised dispatch is reached via `Attack` so the same
  gate applies. Error is `"... is charmed by ... and cannot attack
  them"`.
- **C-42-exhaustion-speed — APPROVED.** Both branches of
  `ResolveTurnResources` (`turnresources.go:224, 264`) now route
  through `EffectiveSpeedWithExhaustion(speed, conds, exhaustion)`.

## Findings

- All seven tasks ship dedicated tests in
  `internal/combat/bundle3_test.go` and the extended
  `turnresources_test.go`. Tests target the exact unwired path (e.g.
  `TestServiceAttack_AutoPopulatesHostileNear_RangedWithAdjacentHostile`
  checks the `DisadvantageReasons` slice, not a mocked field).
- Batch-1/2 hooks intact: `resolveAttackCover` at L878/L1039/L1143,
  `markRageAttacked` + `populatePostHitPrompts` at L975-979 (and
  parallel sites), `ApplyDamage` seam in `damage.go:175` unchanged.
- `EndCombat` correctly orders the combatants snapshot (L867) before
  `recoverEncounterAmmunition` (L907) — no nil-list risk.
- Coverage holds: `internal/combat` 93.0% (gate 85%).

## Verification

- `make build` — PASS.
- `make test` — PASS.
- `make cover-check` — PASS (combat 93.0%).
- `go test ./internal/combat/ -count=1 -run "TestServiceAttack_Charmed|
  TestDetectAdvantage_TargetReckless|TestServiceAttack_Reckless|
  TestPopulateAttackContext|TestServiceAttack_AutoPopulatesHostileNear|
  TestEndCombat_RecoversHalfSpentAmmunition|
  TestApplyCondition_AirborneProne|TestApplyCondition_GroundedProne|
  TestAmmoSpentTracker|TestRecordAmmoSpent|
  TestResolveTurnResources_Exhaustion|
  TestResolveTurnResources_NPCExhaustion"` — 20/20 PASS.

## Next steps

1. Close all seven C-CORRECTNESS task files; mark bundle CLOSED.
2. File follow-ups already named by impl: race-aware PC size lookup,
   Discord-side ammo-recovery surface line, persistent ammo-spent
   column for pod-restart resilience.
3. No regressions or rework requested.
