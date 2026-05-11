# DEATH-SAVE bundle — independent review

Reviewer: opus-4.7 (independent). Date: 2026-05-11.
Scope: 4 tasks under C-43 (instant-death, damage-at-0hp, prone-on-drop, heal-reset).

## Per-task verdicts

### C-43-instant-death — APPROVED

- `routePhase43DeathSave` in `damage.go:302` routes both drop-to-0 and damage-at-0 paths through `ProcessDropToZeroHP`/`CheckInstantDeath`. Overflow is computed from `rawNewHP` (pre-clamp) at `damage.go:231–246`, so overkill magnitude is preserved.
- `TestApplyDamage_InstantDeath_OverflowExceedsMax` (HP 20, dmg 41) and `TestApplyDamage_OverflowJustBelowMax_DyingNotDead` (dmg 39) bracket the rule and assert `InstantDeath` + skipped dying-conditions. Strip the helper and both tests fail.

### C-43-damage-at-0hp — APPROVED

- Same helper persists failures via `UpdateCombatantDeathSaves` (`damage.go:338`) before HP write. Normal +1, crit +2, 3rd failure flips `isAlive=false`. `IsCritical` plumbed through `ApplyDamageInput`.
- `TestApplyDamage_AtZeroHP_NormalHitAddsOneFailure`/`_CriticalAddsTwoFailures`/`_ThirdFailureKills`/`_OverflowGreaterThanMaxInstantDeath` cover every branch; instant-death precedence asserted by `assert.Empty(deathSaveWrites)` on the overflow case.

### C-43-prone-on-drop — APPROVED

- `applyDamageHP` (`concentration.go:383`) iterates `ConditionsForDying()` with a per-condition `HasCondition` skip. `applyDamageHP` signature unchanged — confirmed at line 320 (matches all existing callers).
- `TestApplyDamageHP_AppliesUnconsciousAtZeroHP` now asserts both `unconscious` and `prone`; `TestApplyDamageHP_DoesNotDoubleApplyUnconscious` pre-seeds both for the idempotency test. `TestApplyDamage_DropToZero_AppliesProneAndUnconscious` covers it at the integration layer.

### C-43-heal-reset — APPROVED

- `MaybeResetDeathSavesOnHeal` (`deathsave.go:260`) is PC-only, gated on pre-heal HP <= 0 AND post-heal HP > 0. Resets death saves and removes `ConditionsForDying()` bundle via existing condition pipeline.
- Wired at the two cited sites: `LayOnHands` (`lay_on_hands.go:142`) and `PreserveLife` (`channel_divinity.go:660`). Both pass the pre-heal `target` snapshot.
- `TestLayOnHands_HealsFromZero_ResetsDeathSaves` (Failures=2 → Failures=0) covers Lay on Hands.

## Verification

- `make build` clean.
- `make test` green.
- `make cover-check` — overall 92.81%, `internal/combat` 93.31% (threshold 85%). OK.
- `applyDamageHP` signature unchanged. No WHAT comments in touched files. No skipped hooks.

## Skipped heal sites — credibility

Worklog flags four deferrals: `timer_resolution.go` Nat-20 (HP write skips a death-save reset; pre-existing, confirmed at lines 156–164 — outside file zone), DM dashboard/workspace handlers (generic setters, route through `ApplyDamage` overrides), `internal/rest.LongRest` (pure-logic, returns `DeathSavesReset: true`; persistence is on the discord caller), spell-cast healing (DM applies via dashboard). All credible deferrals — none is a hot critical-path heal that would be hit during normal play (Lay on Hands + Preserve Life are the two routine PC heal-from-0 actions). Timer Nat-20 is worth a follow-up ticket but not blocking.

## Verdict

**Approved.** Four tasks done with strong tests, no regressions, coverage intact.

## Next steps

- File a follow-up for `timer_resolution.go` Nat-20 path to call `MaybeResetDeathSavesOnHeal` (or persist `outcome.DeathSaves`) so the Nat-20 1-HP regain actually clears tallies in the DB.
- Future Phase 43-followup: thread reset hook into spell-cast / DM-dashboard heal paths once those become PC-targeted heals rather than generic HP setters.
