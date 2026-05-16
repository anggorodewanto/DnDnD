finding_id: cross-cut-H03
severity: High
title: Attack roll always adds proficiency bonus regardless of weapon proficiency
location: internal/combat/attack.go:103-106 (AttackModifier)
spec_ref: PHB p.194 Attack Rolls
problem: |
  AttackModifier returns ability + profBonus unconditionally. A wizard wielding a longsword still gets +PB.
suggested_fix: |
  Add a proficient bool parameter and gate the proficiency add on it.
acceptance_criterion: |
  AttackModifier with proficient=false returns only the ability modifier. A test demonstrates this.
