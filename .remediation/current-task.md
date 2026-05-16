finding_id: D-H05
severity: High
title: Monk Unarmored Movement not gated on "no shield"
location: internal/combat/monk.go:487 (UnarmoredMovementFeature)
spec_ref: Phase 48a; PHB Monk
problem: |
  UnarmoredMovementFeature only filters on NotWearingArmor. A monk with a shield still gets the +10-30ft speed bonus.
suggested_fix: |
  Add a HasShield/NotUsingShield condition to the feature filter.
acceptance_criterion: |
  A monk with a shield does NOT get the Unarmored Movement speed bonus. A test demonstrates this.
