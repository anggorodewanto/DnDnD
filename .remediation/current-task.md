finding_id: C-H01
severity: High
title: Auto-crit applies to ranged attacks within 5ft against paralyzed/unconscious
location: internal/combat/attack.go:727-748 (CheckAutoCrit)
spec_ref: Phase 34; spec line 694
problem: |
  CheckAutoCrit only gates on distFt > 5 and does not consider weapon type. A point-blank ranged shot against a paralyzed target auto-crits. Per RAW, only melee attacks within 5ft auto-crit.
suggested_fix: |
  Add a melee-only gate: if IsRangedWeapon(weapon) && !thrown, return false.
acceptance_criterion: |
  CheckAutoCrit returns false for ranged weapons within 5ft against paralyzed targets. Returns true for melee. Tests demonstrate both.
