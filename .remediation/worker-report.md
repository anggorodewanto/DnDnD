# Worker Report: D-H06

## Finding
RevertWildShape doesn't restore SpeedFt from the snapshot.

## Root Cause
`RevertWildShapeService` restored HP/AC from the wild shape snapshot but did not update the turn's `MovementRemainingFt` to the druid's original speed. After revert, the turn retained the beast's movement budget.

## Fix Applied

**File:** `internal/combat/wildshape.go` (RevertWildShapeService, ~line 455)

Added snapshot parsing before the turn persist to restore `MovementRemainingFt` from `snap.SpeedFt`:

```go
if cmd.Combatant.WildShapeOriginal.Valid {
    var snap WildShapeSnapshot
    if err := json.Unmarshal(cmd.Combatant.WildShapeOriginal.RawMessage, &snap); err == nil && snap.SpeedFt > 0 {
        updatedTurn.MovementRemainingFt = snap.SpeedFt
    }
}
```

## Test Added

**File:** `internal/combat/wildshape_test.go`

`TestService_RevertWildShape_RestoresSpeed` — sets up a turn with beast movement (40ft), reverts wild shape, asserts `result.Turn.MovementRemainingFt == 30` (druid's snapshot speed).

## Verification
- Red: test failed with `expected: 30, actual: 40`
- Green: test passes after fix
- All tests pass (`go test ./... -short` excluding `internal/database`)
