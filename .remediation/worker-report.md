# Worker Report: H-H12

**Worker:** worker-H-H12  
**Finding:** Plus-2 ASI silently truncates at cap (loses 1 point) without warning  
**Status:** ✅ Fixed

## Changes

### `internal/levelup/asi.go` (line 81)
- Replaced `current >= maxAbilityScore` check with `current+2 > maxAbilityScore`.
- Error message now reads: `"%s is already %d — +2 would exceed 20"`.
- The silent cap in `setScore` is no longer reachable for this path since we reject before calling it.

### `internal/levelup/asi_test.go` (new file)
- Added `TestApplyPlus2_RejectsWhenExceeds20`: calls `applyPlus2` with STR=19, asserts error is returned.

### `internal/levelup/levelup_test.go` (line 174)
- Renamed `TestApplyASI_CapAt20` → `TestApplyASI_RejectsWhenPlus2Exceeds20` and updated to expect an error (was asserting the old buggy cap-to-20 behavior).

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
