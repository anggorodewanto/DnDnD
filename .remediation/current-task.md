finding_id: H-H10
severity: High
title: DeriveSpeed ignores race
location: internal/portal/builder_store_adapter.go:275
spec_ref: spec §"Player Portal" line 2392; SRD races
problem: |
  DeriveSpeed(_ string) int { return 30 } always returns 30. Dwarf/Halfling/Gnome should be 25ft.
suggested_fix: |
  Look up the race by ID and use its speed_ft. Fall back to 30 if unknown.
acceptance_criterion: |
  DeriveSpeed("dwarf") returns 25. DeriveSpeed("human") returns 30. A test demonstrates both.
