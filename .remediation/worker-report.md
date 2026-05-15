# Worker Report: C-H05 — Fall damage missing 20d6 cap

**Worker:** worker-C-H05
**Status:** ✅ Complete
**Date:** 2026-05-16

## Finding

`FallDamage` in `internal/combat/altitude.go` computed `numDice := int(altitudeFt) / 10` with no upper bound. A 500ft fall produced 50d6 instead of the PHB-specified maximum of 20d6.

## Fix Applied

Added `if numDice > 20 { numDice = 20 }` immediately after computing `numDice` (line 114 of `altitude.go`).

## Test Added

`TestFallDamage_500ft_CappedAt20d6` in `altitude_test.go` — asserts that a 500ft fall yields exactly 20 dice and corresponding capped damage.

## Verification

- `make test` — all tests pass (including new test)
- `make cover-check` — all coverage thresholds met
