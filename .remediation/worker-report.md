# Worker Report: C-H09 — Diagonal pathfinding ignores wall edges

## Status: FIXED ✅

## Summary

Diagonal moves in the A* pathfinder never checked `blockedEdges`, allowing movement through L-shaped wall corners. Fixed by checking both perpendicular edges for diagonal moves; if both are blocked, the diagonal is impassable.

## Changes

### `internal/pathfinding/pathfinding.go` (line ~242)

Added diagonal wall check after the existing cardinal wall check:

```go
// Diagonal: blocked if BOTH perpendicular edges are walled (L-corner)
if isDiagonal(dr, dc) {
    e1 := blockedEdges[makeEdge(cur.Row, cur.Col, cur.Row+dr, cur.Col)]
    e2 := blockedEdges[makeEdge(cur.Row, cur.Col, cur.Row, cur.Col+dc)]
    if e1 && e2 {
        continue
    }
}
```

### `internal/pathfinding/pathfinding_test.go`

1. **Added** `TestFindPath_DiagonalBlockedByBothPerpendicularWalls` — 2×2 grid with both perpendicular walls; asserts diagonal is blocked (`res.Found == false`).

2. **Fixed** `TestFindPath_DiagonalCornerCuttingAllowed` — previously tested with TWO walls (incorrectly expecting diagonal to pass). Updated to use only ONE wall, which is the actual corner-cutting scenario per spec.

## Test Results

All tests pass (`go test ./...` excluding `internal/database`): 29 packages OK.
