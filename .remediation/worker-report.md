# Worker Report: cross-cut-H04

## Finding

Paladin branch in `ChannelDivinityMaxUses` incorrectly returned 2 at level >= 15. Per PHB p.85, Paladin never gains a second Channel Divinity use (only Cleric scales).

## Changes

### Test (`internal/combat/channel_divinity_test.go`)

Updated `TestChannelDivinityMaxUses_Paladin` to assert levels 15 and 20 return 1 (not 2).

### Fix (`internal/combat/channel_divinity.go`)

Removed the `level >= 15 → return 2` branch for paladin. The paladin case now returns 1 for level >= 3, 0 otherwise.

## Verification

- `make test` — all tests pass ✅
- `make cover-check` — all coverage thresholds met ✅
- TDD cycle followed: Red (test failed with actual=2, expected=1) → Green (fix applied, test passes)
