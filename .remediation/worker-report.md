# F-H06 Remediation Report

## Finding
Legendary-action budget round-trips through the URL/request body — no server persistence. Two dashboards can desync the budget.

## Fix Applied

Added server-side legendary budget tracking via `GetLegendaryBudget(ctx, combatantID) (int, error)` and `DecrementLegendaryBudget(ctx, combatantID) error` on the `Store` interface.

### Files Modified
1. **internal/combat/service.go** — Added `GetLegendaryBudget` and `DecrementLegendaryBudget` to the `Store` interface.
2. **internal/combat/legendary_handler.go** — `ExecuteLegendaryAction` now reads budget from store; rejects with 400 if budget is 0; decrements on success. Falls back to client-supplied budget when store returns -1 (uninitialized).
3. **internal/combat/store_adapter.go** — In-memory implementation with `sync.Mutex`-protected map (same pattern as `SetLastLairAction`).
4. **internal/combat/service_test.go** — Added mock fields and method implementations.
5. **internal/combat/legendary_handler_test.go** — Added two TDD tests:
   - `TestExecuteLegendaryAction_ServerSideBudgetExhausted` — budget=0 returns 400.
   - `TestExecuteLegendaryAction_ServerSideBudgetDecremented` — successful action decrements store.

## TDD Cycle
- **Red:** Wrote failing tests referencing `getLegendaryBudgetFn` / `decrementLegendaryBudgetFn` before implementation.
- **Green:** Added interface methods, mock, store_adapter, and handler logic.
- **Verify:** All tests pass (`go test ./internal/combat/...` — 16.8s, 0 failures). Full project builds cleanly.

## Test Results
```
=== RUN   TestExecuteLegendaryAction_ServerSideBudgetExhausted
--- PASS: TestExecuteLegendaryAction_ServerSideBudgetExhausted (0.00s)
=== RUN   TestExecuteLegendaryAction_ServerSideBudgetDecremented
--- PASS: TestExecuteLegendaryAction_ServerSideBudgetDecremented (0.00s)
```

All 12 `TestExecuteLegendaryAction_*` tests pass. No regressions.
