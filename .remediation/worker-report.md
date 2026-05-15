finding_id: G-C01
status: done
files_changed:
  - internal/magicitem/effects.go
  - internal/magicitem/effects_test.go
test_command_that_validates: go test ./internal/magicitem/ -run "TestParsePassiveEffects_ModifySaveAlias|TestParsePassiveEffects_GrantResistanceAlias" -v
acceptance_criterion_met: yes
notes: Added "modify_save" as a comma-separated alias alongside "modify_saving_throw" and "grant_resistance" alongside "resistance" in the convertPassiveEffect switch statement. Two new tests verify that both aliases produce identical combat.Effect output to their canonical counterparts. All existing tests continue to pass and coverage thresholds are met.
follow_ups:
  - Audit other passive effect type strings in spec vs code for similar mismatches
  - Consider adding a validation/warning log when an unrecognized effect type is encountered rather than silently skipping
