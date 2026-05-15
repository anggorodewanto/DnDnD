# Worker Report: C-H06

**Finding:** Resistance halving can produce 0 damage (RAW says min 1)
**Status:** ✅ Fixed

## Changes

### `internal/combat/damage.go` (line 41)
Added min-1 clamp after resistance halving:
```go
if isResistant {
    damage := rawDamage / 2
    if damage < 1 && rawDamage >= 1 {
        damage = 1
    }
    return damage, "resistance to " + dt
}
```

### `internal/combat/damage_test.go`
Added two tests:
- `TestApplyDamageResistances_ResistanceMin1` — 1 fire damage with fire resistance returns 1 (not 0).
- `TestApplyDamageResistances_ResistanceZeroInputStaysZero` — 0 damage with resistance stays 0.

## Verification
- `make test` — all tests pass (no regressions).
- `make cover-check` — all coverage thresholds met.
