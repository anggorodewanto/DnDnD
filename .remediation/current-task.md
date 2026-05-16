finding_id: D-H03
severity: High
title: Auto-ability selection for finesse weapons silently disables rage damage
location: internal/combat/attack.go:1583 (attackAbilityUsed)
spec_ref: spec §Feature Effect System "Rage" ability_used: str; Phase 46
problem: |
  attackAbilityUsed picks the higher of STR/DEX for finesse weapons. A raging barbarian with STR 14/DEX 16 wielding a rapier is force-assigned "dex", and the rage ability_used: str filter fails, dropping the +2/+3/+4 rage damage.
suggested_fix: |
  When the attacker is raging and the weapon is finesse, prefer STR (since rage damage only applies on STR attacks).
acceptance_criterion: |
  A raging barbarian with higher DEX than STR using a finesse weapon gets "str" as the ability used. A test demonstrates this.
