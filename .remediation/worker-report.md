finding_id: G-C04
status: done
files_changed:
  - internal/inventory/service.go
  - internal/inventory/service_test.go
test_command_that_validates: go test ./internal/inventory/ -run TestUseConsumable_Antitoxin -v
acceptance_criterion_met: yes
notes: Added `AppliedCondition string` field to `UseResult` struct and set it to "antitoxin" in the antitoxin branch of `UseConsumable`. The existing test was extended to assert `result.AppliedCondition == "antitoxin"`. All tests pass and coverage thresholds are met. The caller (handler) is responsible for persisting this condition on the combatant.
follow_ups:
  - The save handler should consult the "antitoxin" condition to grant advantage on poison saves
  - Consider adding duration tracking (1 hour / 10 rounds) for the antitoxin condition
