# Worker Report: A-H05 — Portal token redemption TOCTOU race

## Finding

`RedeemToken` performed a SELECT (via `ValidateToken`) then a separate UPDATE (via `MarkUsed`) without atomicity. Two concurrent calls could both pass the `used` check and both succeed.

## Fix Applied

### `internal/portal/token_store.go` — `MarkUsed`

Changed the UPDATE to include `AND used = false` in the WHERE clause. If zero rows are affected, the function returns `ErrTokenUsed`. This makes the used-check atomic at the database level.

### Test Added

`TestTokenService_RedeemToken_ConcurrentDoubleRedeem` in `token_service_test.go` — fires two goroutines calling `RedeemToken` on the same token simultaneously. Asserts exactly one succeeds and the other returns `ErrTokenUsed`. Run with `-count=10` to confirm consistency.

## TDD Workflow

1. **Red:** Added concurrent test → failed consistently ("both concurrent RedeemToken calls succeeded").
2. **Green:** Applied conditional UPDATE fix → test passes on all 10 iterations.
3. **Verify:** `make test` ✅ | `make cover-check` ✅ (portal package at 87.95%).

## Files Changed

- `internal/portal/token_store.go` (MarkUsed — 6 lines added)
- `internal/portal/token_service_test.go` (new test — 30 lines added)
