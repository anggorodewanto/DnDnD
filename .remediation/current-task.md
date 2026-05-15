finding_id: C-H03
severity: High
title: Crossbow Expert does not waive ranged-with-hostile-adjacent disadvantage
location: internal/combat/advantage.go:88-91
spec_ref: Phase 35; spec line 687
problem: |
  DetectAdvantage adds "hostile within 5ft" disadvantage when HostileNearAttacker && IsRangedWeapon. AttackInput.HasCrossbowExpert is populated but the disadvantage rule never consults it.
suggested_fix: |
  Add && !input.HasCrossbowExpert to the hostile-near-attacker ranged disadvantage branch.
acceptance_criterion: |
  A ranged attack with HasCrossbowExpert=true and HostileNearAttacker=true does NOT get disadvantage. A test demonstrates this.
