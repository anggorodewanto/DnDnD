finding_id: D-C02
status: done
files_changed:
  - internal/portal/init_feature_uses.go
  - internal/portal/builder_store_adapter.go
  - internal/portal/builder_store_adapter_test.go
test_command_that_validates: go test ./internal/portal/ -run TestBuilderStoreAdapter_CreateCharacterRecord_InitializesFeatureUses -v
acceptance_criterion_met: yes
notes: Created InitFeatureUses helper that computes initial feature uses from class entries and ability scores. Wired it into CreateCharacterRecord so the FeatureUses field is populated at character creation. Covers rage (barbarian), ki (monk 2+), channel-divinity (cleric/paladin), bardic-inspiration (bard), lay-on-hands (paladin), action-surge (fighter 2+), second-wind (fighter 2+), wild-shape (druid 2+), and sorcery-points (sorcerer 2+). All existing tests pass, make test and make cover-check succeed.
follow_ups:
  - Verify the dashboard charcreate path also calls InitFeatureUses (or shares the same code path)
  - Consider adding a migration/backfill script for existing characters with null feature_uses
  - Unlimited rage at level 20 (RageUsesPerDay returns -1) is stored as -1; verify rest/recharge logic handles this
