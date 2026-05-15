finding_id: D-C01
status: done
files_changed:
  - internal/refdata/seed_classes.go
  - internal/combat/feature_integration_test.go
test_command_that_validates: go test ./internal/combat/ -run TestBuildFeatureDefinitions_BarbarianSeedRage -v
acceptance_criterion_met: yes
notes: The barbarian Rage feature in seed_classes.go used a descriptive comma-separated mechanical_effect string ("advantage_str_checks_saves,resistance_bludgeoning_piercing_slashing,bonus_rage_damage") that splitMechanicalEffects split into tokens none of which matched the "rage" case in BuildFeatureDefinitions. Changed the seed to use "rage" which is the token the FES expects. Added an integration test that verifies BuildFeatureDefinitions produces a RageFeature from the barbarian's seed-level features. All tests pass and coverage thresholds are met.
follow_ups:
  - Consider adding similar seed-integration tests for other classes (e.g. Reckless Attack uses "advantage_str_melee_attacks,attacks_against_have_advantage" which also won't match any switch case)
