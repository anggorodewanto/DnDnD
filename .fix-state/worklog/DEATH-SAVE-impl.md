# DEATH-SAVE bundle — implementation worklog

Bundle: C-43 death-save wiring (4 tasks). Date: 2026-05-11.
Implementer: opus-4.7.

## Task status

### C-43-instant-death — DONE

- Wired `CheckInstantDeath` / `ProcessDropToZeroHP` into `Service.ApplyDamage` via the new helper `Service.routePhase43DeathSave` in `internal/combat/damage.go`.
- Overflow is computed from `rawNewHP` (pre-clamp HP delta) so the instant-death magnitude check sees the true overkill, not the clamped-to-0 value.
- Triggers from BOTH paths: drop-to-0 (prevHP > 0, newHP <= 0) and damage-at-0 (prevHP <= 0). Instant-death takes precedence over the failure-tally in the damage-at-0 path.
- New `ApplyDamageInput.IsCritical` field and `ApplyDamageResult.InstantDeath`/`DeathSaveOutcome`/expanded `Killed` semantics exposed so callers and dashboard surfaces can surface the new outcome.
- Existing `TestApplyDamage_PCAtZeroStaysAliveDying` was asserting the old (buggy) behavior — kept the same intent but reduced raw damage so overflow < HpMax (no instant-death) and the dying flow still runs.

### C-43-damage-at-0hp — DONE

- Same helper routes through `ApplyDamageAtZeroHP` when `target.HpCurrent <= 0` and `adjusted > 0`. Normal hit adds 1 failure, crit adds 2 (via the existing helper).
- Failures are persisted via `Store.UpdateCombatantDeathSaves` immediately, before the HP write.
- 3rd failure flips `outcome.IsAlive = false` which then drives `isAlive = false` in the outer `ApplyDamage`, so the HP row + IsAlive end up consistent.
- Crit information enters via `ApplyDamageInput.IsCritical` (defaults to false for AoE / non-attack damage).

### C-43-prone-on-drop — DONE

- `applyDamageHP` in `internal/combat/concentration.go` now iterates `ConditionsForDying()` (unconscious + prone) rather than hardcoding `unconscious`. Each condition is skipped if already present, so the per-tick idempotency from the original implementation is preserved per-condition.
- Updated the two affected tests in `internal/combat/concentration_test.go`:
  - `TestApplyDamageHP_AppliesUnconsciousAtZeroHP` now captures all condition names across both calls (map) and asserts BOTH `unconscious` and `prone` were applied.
  - `TestApplyDamageHP_DoesNotDoubleApplyUnconscious` was extended to pre-seed both conditions so the test still verifies the no-op skip.

### C-43-heal-reset — DONE

- Added `Service.MaybeResetDeathSavesOnHeal` in `internal/combat/deathsave.go`. PC-only, no-op when pre-heal HP > 0 or post-heal HP <= 0. Wraps a private `resetDyingState` that zeroes the death save tallies and removes the `ConditionsForDying()` bundle.
- Wired at two HP-restore call sites:
  - `Service.LayOnHands` in `internal/combat/lay_on_hands.go` (Paladin Lay on Hands).
  - `Service.PreserveLife` in `internal/combat/channel_divinity.go` (Cleric Channel Divinity: Preserve Life).
- Both pass the pre-heal `target` snapshot to `MaybeResetDeathSavesOnHeal` so the routing sees pre-heal HP.

## Heal-call sites considered but SKIPPED (out of scope / out of file zone)

- `internal/combat/timer_resolution.go`: the Nat-20 death-save branch in `TurnTimer.AutoResolveTurn` restores 1 HP without persisting reset death saves. This is a pre-existing data inconsistency (the `RollDeathSave` outcome's zero-valued `DeathSaves` is not written through). Timer file is outside this bundle's file zone — flag for a separate follow-up; the existing `RollDeathSave` test still passes since it asserts only the returned outcome.
- `internal/combat/dm_dashboard_handler.go` and `workspace_handler.go`: direct HP edits via DM dashboard / workspace REST. These are generic HP setters (used for both damage and heal), not heal-specific paths. Touching them risks reverting DM-intended damage edits. Better as a Phase 43 DM-UX follow-up.
- `internal/rest/rest.go` (`LongRest`): pure-logic package, already returns `DeathSavesReset: true`. The caller (`internal/discord/rest_handler.go`) is responsible for persisting; out of file zone.
- Spell-cast healing (`spellcasting.go`): combat spells report scaled healing dice but do not apply HP — the DM applies via dashboard PATCH. Same call-site limitation as above.

## Verification

- `go test ./internal/combat/` — green (16.9s).
- `make test` — green.
- `make cover-check` — coverage thresholds met (combat at 93.4%).
- `make build` — clean.

## Tests added

`internal/combat/deathsave_integration_test.go` (new file):

- `TestApplyDamage_DropToZero_AppliesProneAndUnconscious`
- `TestApplyDamage_InstantDeath_OverflowExceedsMax`
- `TestApplyDamage_OverflowJustBelowMax_DyingNotDead`
- `TestApplyDamage_AtZeroHP_NormalHitAddsOneFailure`
- `TestApplyDamage_AtZeroHP_CriticalAddsTwoFailures`
- `TestApplyDamage_AtZeroHP_OverflowGreaterThanMaxInstantDeath`
- `TestApplyDamage_AtZeroHP_ThirdFailureKills`
- `TestLayOnHands_HealsFromZero_ResetsDeathSaves`

All eight were RED before the fix and GREEN after.

## Files touched

- `internal/combat/damage.go` — extended `ApplyDamageInput` (+ `IsCritical`) and `ApplyDamageResult` (+ `InstantDeath`, `DeathSaveOutcome`). New helper `routePhase43DeathSave`.
- `internal/combat/concentration.go` — `applyDamageHP` now iterates `ConditionsForDying()`.
- `internal/combat/deathsave.go` — added `MaybeResetDeathSavesOnHeal` + `resetDyingState`.
- `internal/combat/lay_on_hands.go` — heal-from-zero reset wired into `Service.LayOnHands`.
- `internal/combat/channel_divinity.go` — heal-from-zero reset wired into `Service.PreserveLife`.
- `internal/combat/concentration_test.go` — updated two pre-existing tests to match new dying-condition bundle behavior.
- `internal/combat/damage_test.go` — reduced raw damage in `TestApplyDamage_PCAtZeroStaysAliveDying` so the test no longer accidentally triggers instant-death.
- `internal/combat/deathsave_integration_test.go` — new file (eight integration tests covering the four C-43 tasks).
