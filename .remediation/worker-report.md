finding_id: F-C04
status: done
files_changed:
  - internal/combat/legendary.go
  - internal/combat/legendary_test.go
test_command_that_validates: go test ./internal/combat/ -run TestBuildTurnQueueEntries_LairActionLosesTies -v
acceptance_criterion_met: yes
notes: Added IsLairAction bool field to TurnQueueEntry struct. Added sort.SliceStable after building all entries in BuildTurnQueueEntries that sorts by initiative descending and pushes lair actions last among entries sharing the same initiative. The failing test confirmed the bug (lair at index 0, combatant at index 1 when both at init 20), and the fix correctly places the combatant before the lair action. All existing tests continue to pass including make test and make cover-check.
follow_ups:
  - Verify the handler test in legendary_handler_test.go (TestGetTurnQueue_WithLegendaryAndLair) still reflects correct ordering expectations if combatants at init 20 are added in future test scenarios.
