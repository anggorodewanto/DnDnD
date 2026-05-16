# Worker Report: F-H05 — Lair Action Tracker Persistence

## Status: ✅ COMPLETE

## Summary

The `LairActionTracker` was purely in-memory and lost on restart. The handler relied on the client passing `last_used` as a query parameter or request body field. After a restart, the server had no memory of the last lair action used, allowing consecutive repeats.

## Changes Made

### 1. Store Interface (`internal/combat/service.go`)
Added two methods:
- `SetLastLairAction(ctx, encounterID, action string) error`
- `GetLastLairAction(ctx, encounterID) (string, error)`

### 2. Store Adapter (`internal/combat/store_adapter.go`)
Implemented the new methods with an in-process `sync.Mutex`-protected map. This survives within a single process lifetime. A future DB migration can back this with a column on the encounters table.

### 3. Handler (`internal/combat/legendary_handler.go`)
- `GetLairActionPlan`: Now hydrates the tracker from `GetLastLairAction` (falling back to query param for backward compat).
- `ExecuteLairAction`: Now calls `SetLastLairAction` after a successful lair action execution.

### 4. Mock Store (`internal/combat/service_test.go`)
Added `setLastLairActionFn` and `getLastLairActionFn` fields and method implementations.

### 5. Tests
- `legendary_test.go`: Added `TestLairActionTracker_PersistsSurvivesRehydration` — unit test proving rehydration from stored state blocks repeats.
- `legendary_handler_test.go`: Added `TestExecuteLairAction_PersistsLastUsedAction` — integration test proving the full round-trip: execute → persist → GET plan hydrates from store and blocks the repeated action.

## Test Results

```
ok  github.com/ab/dndnd/internal/combat  16.815s
```

All existing tests continue to pass. The new tests pass green.

## TDD Cycle

1. **Red**: Wrote `TestExecuteLairAction_PersistsLastUsedAction` — failed because handler didn't call store methods.
2. **Green**: Added `SetLastLairAction` call in `ExecuteLairAction` and `GetLastLairAction` hydration in `GetLairActionPlan`.
3. **Refactor**: Minimal — no refactoring needed for this small change.

## Notes

- The in-process map in `storeAdapter` is a stopgap. For full restart persistence, a `last_lair_action TEXT` column should be added to the `encounters` table and wired through sqlc. That change would be in `internal/database` (excluded from this task).
- Backward compatibility preserved: the `?last_used=` query param still works as an override.
