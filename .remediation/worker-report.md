finding_id: H-C02
status: done
files_changed:
  - internal/levelup/filter_feats.go
  - internal/levelup/filter_feats_test.go
  - cmd/dndnd/discord_handlers.go
test_command_that_validates: go test ./internal/levelup/ -run TestFilterEligibleFeats -v
acceptance_criterion_met: yes
notes: Added FilterEligibleFeats in the levelup package that excludes feats by owned ID and by unmet prerequisites (delegates to CheckFeatPrerequisites). The production asiFeatLister now fetches the character, parses scores/proficiencies/features, and calls FilterEligibleFeats before returning results. Test confirms both exclusion paths (owned feats and unmet prereqs). Coverage remains above thresholds.
follow_ups:
  - Consider adding the feat description back to FeatOption in the filtered path (currently omitted since refdata.Feat.Description isn't passed through FeatInfo)
  - The 25-item Discord select-menu cap may hide eligible feats if many exist; consider pagination or search
