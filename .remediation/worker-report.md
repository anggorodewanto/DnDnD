finding_id: F-C03
status: done
files_changed:
  - cmd/dndnd/discord_adapters.go
  - cmd/dndnd/main_wiring_test.go
test_command_that_validates: go test ./cmd/dndnd/ -run TestBuildVisionSources_PCDevilsSight -v
acceptance_criterion_met: yes
notes: Added a case-insensitive check for "Devil's Sight" in the PC branch of buildVisionSources using the existing combat.HasFeatureByName helper. The fix reads the character's Features JSON (already fetched via GetCharacter) and sets src.HasDevilsSight = true when the feature is present. A new test TestBuildVisionSources_PCDevilsSight confirms the behavior. All existing tests continue to pass and coverage thresholds are met.
follow_ups:
  - Consider adding a similar test for case variations (e.g., "devil's sight", "DEVIL'S SIGHT") to document the case-insensitive behavior explicitly.
