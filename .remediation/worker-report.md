# Worker Report: D-H01

**Finding:** Step of the Wind dash adds remaining movement, not base speed
**Status:** ✅ Fixed

## Root Cause

In `internal/combat/monk.go:444`, the dash case of `StepOfTheWind` used:
```go
updatedTurn.MovementRemainingFt += cmd.Turn.MovementRemainingFt
```
This added whatever movement was *left* rather than the monk's base speed. A monk who already moved 20ft of their 30ft would only gain +10ft instead of +30ft.

## Fix Applied

Replaced with `resolveBaseSpeed` lookup, mirroring the standard Dash action in `standard_actions.go`:
```go
case "dash":
    speed, err := s.resolveBaseSpeed(ctx, cmd.Combatant)
    if err != nil {
        return KiAbilityResult{}, err
    }
    updatedTurn.MovementRemainingFt += speed
```

## TDD Evidence

- **Red:** Added `TestServiceStepOfTheWind_Dash_AddsBaseSpeedNotRemaining` — monk with 30ft speed, 10ft remaining, expects 40ft after dash. Failed with actual=20.
- **Green:** Applied fix. Test passes with expected=40.
- **Full suite:** `make test` passes. `make cover-check` passes.

## Files Changed

- `internal/combat/monk.go` — fixed dash case (line ~444)
- `internal/combat/monk_test.go` — added regression test
