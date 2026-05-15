finding_id: G-C03
status: done
files_changed:
  - internal/inventory/active_ability.go
  - internal/inventory/active_ability_test.go
  - internal/inventory/recharge.go
  - internal/inventory/recharge_test.go
  - internal/inventory/phase88b_integration_test.go
  - internal/discord/use_handler.go
test_command_that_validates: go test ./internal/inventory/ -run "TestUseCharges_DestroyOnZero" -v
acceptance_criterion_met: yes
notes: Moved the d20 destroy-on-zero check from DawnRecharge into UseCharges (now a method on *Service). When charges reach 0 and DestroyOnZero is true, a d20 is rolled; on a 1, result.Destroyed is set to true. Added a backward-compatible package-level UseCharges wrapper for callers that don't need destroy logic. Updated the discord handler to call the service method directly. All existing tests updated to reflect the new timing.
follow_ups:
  - Handler should pass DestroyOnZero from item's recharge metadata when calling UseCharges (currently not wired from DB charges JSON)
  - If Destroyed=true, handler should remove the item from inventory and notify the channel
