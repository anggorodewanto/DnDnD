finding_id: C-C04
status: done
files_changed:
  - internal/combat/altitude.go
  - internal/combat/altitude_test.go
  - internal/discord/fly_handler.go
  - internal/discord/fly_handler_test.go
test_command_that_validates: go test ./internal/combat/ -run "TestValidateFly_NoFlySpeed|TestValidateFly_WithFlySpeed" -count=1
acceptance_criterion_met: yes
notes: Added HasFlySpeed bool field to FlyRequest and a guard clause at the top of ValidateFly that rejects with "You don't have a fly speed." when false. Added CombatantHasFlySpeed helper that checks for the fly_speed condition in the combatant's conditions JSON. Updated the fly handler to pass this check. Existing tests updated to set HasFlySpeed: true where valid flight is expected. Two new unit tests cover the no-fly-speed rejection and the with-fly-speed allowance paths.
follow_ups:
  - The CombatantHasFlySpeed helper currently only checks the fly_speed condition. Innate creature fly speed (e.g., wild-shaped into a flying beast) should also grant flight — this requires checking the creature's speed JSON when is_wild_shaped is true.
  - The handler should also allow descent (target altitude 0) even without fly speed, since a creature already airborne that loses fly speed should be able to descend gracefully (or fall). This edge case may need a separate spec decision.
