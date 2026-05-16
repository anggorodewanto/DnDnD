finding_id: F-H07
severity: High
title: Counterspell trigger does not validate spell range / line-of-sight
location: internal/combat/counterspell.go:65-116
spec_ref: spec §Counterspell resolution line 1093
problem: |
  TriggerCounterspell accepts any declaration ID and enemy caster ID without computing distance or checking LOS. A DM can fire Counterspell across the map.
suggested_fix: |
  Look up both combatants, compute distance, reject if > 60ft.
acceptance_criterion: |
  TriggerCounterspell returns error when distance > 60ft. A test demonstrates this.
