finding_id: H-C03
status: done
files_changed:
  - internal/levelup/service.go
  - internal/levelup/service_test.go
test_command_that_validates: go test ./internal/levelup/ -run "TestService_ApplyLevelUp_AppendsClassFeatures|TestService_ApplyLevelUp_DeduplicatesFeatures" -v
acceptance_criterion_met: yes
notes: Added feature-append logic in ApplyLevelUp that reads classRef.FeaturesByLevel[strconv.Itoa(newClassLevel)] after computing the new class level. New features are appended to the character's existing features list with deduplication by name. The updated features are persisted via the StatsUpdate.Features field which the store adapter already handles via pickNullable. Two tests added: one verifying a Fighter leveling to 5 gets "Extra Attack", another verifying no duplicates on re-level.
follow_ups:
  - Consider deduping by name+source pair instead of name alone for edge cases where different classes grant identically-named features
  - Subclass features (from SubclassFeaturesByLevel if it exists) may need similar treatment
