finding_id: E-C01
status: done
files_changed:
  - internal/combat/spellcasting.go
  - internal/combat/spellcasting_test.go
test_command_that_validates: go test ./internal/combat/ -run "TestCast_AppliesDamageOnHit|TestCast_AppliesHealingOnCast" -count=1 -v
acceptance_criterion_met: yes
notes: Added DamageTotal and HealingTotal fields to CastResult. After the spell attack roll (step 12), added step 12α that rolls scaled damage dice on hit and routes through ApplyDamage, and step 12β that rolls scaled healing dice and updates target HP via UpdateCombatantHP (clamped to HpMax). Both paths handle the "+mod" placeholder by substituting the spellcasting ability modifier before rolling. All existing tests continue to pass.
follow_ups:
  - Consider adding critical hit double-dice logic for spell attacks
  - Consider death-save reset on healing spells (like lay_on_hands does)
  - Consider resistance/immunity logging in the CastResult for spell damage
