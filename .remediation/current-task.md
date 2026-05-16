finding_id: E-H05
severity: High
title: Spell attack rolls never apply advantage/disadvantage
location: internal/combat/spellcasting.go:638
spec_ref: Phase 58; spec §989
problem: |
  Cast hard-codes dice.Normal for the d20 roll. Hidden caster, invisible target, target prone, attacker restrained/poisoned — none adjust the spell attack roll.
suggested_fix: |
  Pass a RollMode parameter to the spell attack roll. The simplest fix: add a RollMode field to CastCommand that callers can set based on conditions. Default to Normal for backward compat.
acceptance_criterion: |
  A spell attack with RollMode=Advantage rolls with advantage. A test demonstrates this.
