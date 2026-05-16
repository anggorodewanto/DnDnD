finding_id: E-H02
severity: High
title: AoE pending save DC subtraction loses cover information
location: internal/combat/aoe.go:592
spec_ref: Phase 59; spec §891
problem: |
  Storing DC - CoverBonus means the saver never sees the bonus in their roll log, and the DC displayed is artificially lowered. Cover bonus should be added to the saver's roll, not subtracted from DC.
suggested_fix: |
  Keep DC = original spell DC. At resolution time add the cover bonus to the player's d20 total before comparing to DC.
acceptance_criterion: |
  The pending save stores the original DC (not DC-cover). At resolution, cover bonus is added to the roll total. A test demonstrates this.
