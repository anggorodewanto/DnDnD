# Worker Report: G-H01 — Gold split silently discards remainder

## Status: ✅ FIXED

## Summary

`SplitGold` in `internal/loot/service.go` zeroed the pool's `GoldTotal` after distributing shares. When the total was not evenly divisible (e.g., 7gp / 3 players), the remainder was lost.

## Changes

### 1. Test added (Red)

`internal/loot/service_test.go` — `TestSplitGold_RemainderRetained`: splits 7gp among 3 players, asserts each gets 2gp and pool retains 1gp.

### 2. Fix applied (Green)

`internal/loot/service.go` line ~321: replaced `GoldTotal: 0` with `GoldTotal: pool.GoldTotal % int32(len(pcs))`.

### 3. Mock test updated

`internal/loot/service_mock_test.go` — `TestSplitGold_ZeroPoolGoldFailure`: updated error string assertion from `"zeroing pool gold"` to `"updating pool gold remainder"`.

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
