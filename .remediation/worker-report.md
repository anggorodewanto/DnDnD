finding_id: cross-cut-C01
status: done
files_changed:
  - internal/combat/channel_divinity_integration_test.go
  - internal/combat/channel_divinity_test.go
  - internal/rest/rest_test.go
test_command_that_validates: go test ./internal/rest/ -run TestShortRest_ChannelDivinity_RechargesOnShortRest -v
acceptance_criterion_met: yes
notes: All channel-divinity test fixtures across 2 files (35 total occurrences) were changed from Recharge "long" to "short" per PHB p.59/p.85. The InitFeatureUses helper in internal/portal/init_feature_uses.go already correctly used "short" and required no change. A regression test was added to rest_test.go confirming ShortRest recharges channel-divinity. All tests pass and coverage thresholds are met.
follow_ups:
  - Verify any seed/migration scripts outside internal/ also use "short" for channel-divinity
  - Consider adding a linter rule or constant to prevent reintroduction of this bug
