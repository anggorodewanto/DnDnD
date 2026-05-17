finding_id: C-H09
severity: High
title: Diagonal pathfinding ignores wall edges entirely
location: internal/pathfinding/pathfinding.go:242-244
spec_ref: Phase 29; spec line 1391
problem: |
  The code only checks blockedEdges for cardinal moves. For diagonals it never tests walls. The spec permits corner-cutting (two perpendicular walls meeting at a corner) but a diagonal move through a wall segment should be blocked.
suggested_fix: |
  For diagonal moves, check that at least one of the two perpendicular edges is NOT blocked. If both are blocked, the diagonal is impassable.
acceptance_criterion: |
  A diagonal move through two perpendicular walls (L-shaped corner) is blocked. A diagonal through only one wall (corner-cutting) is allowed. Tests demonstrate both.
