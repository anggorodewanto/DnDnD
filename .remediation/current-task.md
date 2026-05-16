finding_id: G-H02
severity: High
title: Long-rest hit-dice restoration order is non-deterministic for multiclass
location: internal/rest/rest.go:409-441
spec_ref: spec §Long Rest line 2609 (Phase 83a)
problem: |
  LongRest iterates over maxHitDice (a map[string]int) to allocate the half-level restoration budget. Map iteration order in Go is randomized.
suggested_fix: |
  Sort the die types before iterating (e.g., largest die first: d12, d10, d8, d6).
acceptance_criterion: |
  Hit dice restoration is deterministic (largest die first). A test with a multiclass character demonstrates consistent ordering.
