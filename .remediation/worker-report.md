# Worker Report: cross-cut-H01

## Finding

**Title:** routePhase43DeathSave skips instant-death when rawNewHP == 0  
**Location:** `internal/combat/damage.go:340-344`

## Changes Made

### Fix: `internal/combat/damage.go`

Replaced the conditional overflow computation:

```go
// OLD (buggy):
overflow := 0
if rawNewHP < 0 {
    overflow = -rawNewHP
}
```

With a direct PHB-compliant formula:

```go
// NEW (fixed):
overflow := adjusted - int(target.HpCurrent)
if overflow < 0 {
    overflow = 0
}
```

This computes overflow as "damage remaining after HP drops to 0" per PHB p.197, regardless of whether `rawNewHP` is exactly 0 or negative. The old code only computed overflow when `rawNewHP < 0`, leaving the `rawNewHP == 0` boundary unhandled.

### Test: `internal/combat/deathsave_integration_test.go`

Added `TestApplyDamage_InstantDeath_LowHPMassiveDamage`:
- PC at 1 HP, maxHP 10, takes 15 slashing damage
- Overflow = 15 − 1 = 14 ≥ 10 maxHP → instant death
- Asserts: `NewHP == 0`, `IsAlive == false`, `Killed == true`, `InstantDeath == true`

## Verification

- `go test ./internal/combat/ -cover` → PASS, 92.4% coverage (above 85% threshold)
- `make test` → only pre-existing failure in `TestIntegration_MigrateDown` (database migration test, unrelated)
- `make cover-check` → same pre-existing failure only
- All 17 instant-death / overflow / drop-to-zero tests pass

## Not Touched

- No unrelated code modified
- No commits made
