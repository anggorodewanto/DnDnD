finding_id: C-H11
severity: High
title: Concentration-on-damage save uses simplified DC formula
location: internal/combat/concentration.go:422-448
spec_ref: Phase 39/42
problem: |
  Cannot verify from code alone that two separate damage hits in the same round produce two CON saves. Need a regression test.
suggested_fix: |
  Add a regression test where two damage applications both enqueue pending saves and resolve independently.
acceptance_criterion: |
  A test demonstrates that two damage hits to a concentrating target produce two separate concentration saves.
