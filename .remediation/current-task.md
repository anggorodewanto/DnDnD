# Current Task

finding_id: A-M13
severity: Medium
title: HP recompute on multiclassing assumes secondary classes never reach level 1 with max die
location: internal/character/stats.go:30-42
spec_ref: spec §Character Leveling (line 2453); Phase 7
problem: CalculateHP uses array index `i == 0` to determine which class gets max hit die at level 1. If the classes array is ever reordered (DM dashboard edit, alphabetical sort), HP computes wrong. PHB says only the very first character level gets max die.
suggested_fix: Add an `IsPrimary` field to ClassEntry and use it in CalculateHP instead of relying on array position.

## Acceptance Criterion

`CalculateHP` produces correct HP regardless of class array order, using an explicit `IsPrimary` flag rather than positional index.
