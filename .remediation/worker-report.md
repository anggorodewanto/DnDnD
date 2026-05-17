# Worker Report: I-H04

## Finding
**ResolvePendingAction** did not use `withTurnLock`, allowing concurrent mutations without advisory lock protection. **applyMoveEffect** called `store.UpdateCombatantPosition` directly, bypassing the service layer (which handles zone triggers, wall checks, etc.).

## Changes Made

### 1. `internal/combat/dm_dashboard_handler.go`

**ResolvePendingAction** — wrapped mutation logic in `withTurnLock`:
- Fetch active turn before mutations (returns 404 if no active turn)
- Parse effects JSON before the lock (preserves 400 for validation errors)
- All state reads, effect applications, status updates, and action logging now run inside `withTurnLock`

**applyMoveEffect** — replaced raw store call:
- `h.svc.store.UpdateCombatantPosition(...)` → `h.svc.UpdateCombatantPosition(...)`
- This routes through the service layer, enabling zone triggers and wall validation

### 2. `internal/combat/dm_dashboard_handler_test.go`

- Added `TestResolvePendingAction_AcquiresTurnLock`: uses `fakeTxBeginner` (always-fail `BeginTx`) to prove the handler acquires the lock — expects 500 when lock acquisition fails.
- Updated `TestResolvePendingAction_NoActiveTurn`: now expects 404 (turn is required for lock).
- Added `getActiveTurnByEncounterIDFn` mock to 10 existing tests that now reach the turn-fetch step.

## Behavior Change
`ResolvePendingAction` now requires an active turn. Previously it would resolve without logging if no turn existed; now it returns 404. This is consistent with the spec requirement that mutations must be lock-protected.

## Test Results
```
ok  github.com/ab/dndnd/internal/combat  17.160s
```
All 28 `TestResolvePendingAction_*` tests pass. Full package passes.
