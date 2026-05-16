finding_id: E-H01
severity: High
title: Help action grants advantage only on attacks, not on ability checks
location: internal/combat/standard_actions.go:254-261
spec_ref: Phase 54 "Help"; spec §1140
problem: |
  The implementation always sets help_advantage scoped to a TargetCombatantID (enemy). Ability-check Help (helping a teammate) cannot be modelled.
suggested_fix: |
  Make target optional. When omitted, set a help_check_advantage condition on the ally consumed by the next non-attack d20 roll.
acceptance_criterion: |
  Help action without a target enemy sets a help_check_advantage condition on the ally. A test demonstrates this.
