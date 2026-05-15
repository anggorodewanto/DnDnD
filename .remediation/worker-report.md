finding_id: I-C01
status: done
files_changed:
  - internal/dashboard/feature_provider.go
  - internal/dashboard/feature_provider_test.go
  - internal/dashboard/charcreate.go
  - internal/dashboard/charcreate_test.go
  - internal/dashboard/charcreate_service_test.go
  - internal/dashboard/charcreate_handler_test.go
test_command_that_validates: go test ./internal/dashboard/ -run "TestCollectFeatures_CaseInsensitiveLookup|TestRacialTraits_CaseInsensitiveLookup" -v
acceptance_criterion_met: yes
notes: Added strings.ToLower normalization in CollectFeatures (for c.Class and c.Subclass map lookups) and in RacialTraits (for the race map lookup). This ensures that display names like "Barbarian" match slug-keyed maps like "barbarian". Existing tests were updated to use lowercase map keys reflecting the real RefDataFeatureProvider behavior. Two new tests explicitly verify case-insensitive lookup.
follow_ups:
  - Consider normalizing the form option values to slugs at the source (charcreate_handler.go) to avoid relying on ToLower at every lookup site.
  - Audit other map lookups in the dashboard package that may have the same slug-vs-display-name mismatch.
