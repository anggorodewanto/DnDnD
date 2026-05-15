finding_id: E-C01
severity: Critical
title: Single-target spell casts never apply damage or healing
location: internal/combat/spellcasting.go:584-598, internal/discord/cast_handler.go
spec_ref: Phase 58 "Spell Casting — Basic"; spec §891-1072
problem: |
  Cast() computes ScaledDamageDice and ScaledHealingDice as strings and emits them in the combat log but never rolls the dice nor calls UpdateCombatantHP / ApplyDamage. Fire Bolt, Inflict Wounds, Guiding Bolt, Cure Wounds, Healing Word, etc. all just print the dice string with no HP change on the target.
suggested_fix: |
  After step 12 (spell attack roll) in Cast(), roll the scaled damage dice on hit and route through s.ApplyDamage. For healing spells, roll ScaledHealingDice and call UpdateCombatantHP (clamped to HpMax). Mirror Lay-on-Hands' update path.
acceptance_criterion: |
  A successful spell attack (hit) rolls damage dice and reduces target HP. A healing spell rolls healing dice and increases target HP (clamped to max). Tests demonstrate both paths.
