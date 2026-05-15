# Worker Report: C-H03

**Finding:** Crossbow Expert does not waive ranged-with-hostile-adjacent disadvantage  
**Status:** ✅ Fixed  
**Worker:** worker-C-H03  
**Date:** 2026-05-16

## Changes Made

### 1. `internal/combat/advantage.go`
- Added `HasCrossbowExpert bool` field to `AdvantageInput` struct.
- Added `&& !input.HasCrossbowExpert` to the hostile-near-attacker ranged disadvantage condition.

### 2. `internal/combat/attack.go`
- Wired `HasCrossbowExpert` from `AttackInput` into `AdvantageInput` construction.

### 3. `internal/combat/advantage_test.go`
- Added `TestDetectAdvantage_RangedWithHostileNearby_CrossbowExpert_NoDisadvantage`: ranged weapon with `HostileNearAttacker=true` and `HasCrossbowExpert=true` gets no disadvantage.

## Verification

- `make test` — all tests pass.
- `make cover-check` — coverage thresholds met.
- Existing `TestDetectAdvantage_RangedWithHostileNearby` still confirms disadvantage without the feat.
