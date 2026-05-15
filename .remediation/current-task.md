finding_id: A-H08
severity: High
title: Fuzzy match suggestion message renders incorrectly when multiple matches
location: internal/discord/registration_handler.go:97-100
spec_ref: spec §Registration name matching (lines 47-48); Phase 14
problem: |
  When 2-3 fuzzy matches exist, code wraps the entire comma-joined block in a single **…** instead of bolding each name individually. Also shows literal <name> placeholder.
suggested_fix: |
  Bold each suggestion individually: "Did you mean: **Thorn**, **Thorin**, **Thora**?"
acceptance_criterion: |
  Multiple fuzzy matches are each individually bolded. A test demonstrates the correct format.
