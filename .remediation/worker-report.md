finding_id: E-C03
status: done
files_changed:
  - internal/combat/advantage.go
  - internal/combat/advantage_test.go
test_command_that_validates: go test ./internal/combat/ -run TestDetectAdvantage_TargetDodging_Disadvantage -v
acceptance_criterion_met: yes
notes: Added `case "dodge": disadvReasons = append(disadvReasons, "target dodging")` to the target-conditions switch in DetectAdvantage. Wrote a failing test first that asserts DetectAdvantage returns Disadvantage mode with reason "target dodging" when the target has the "dodge" condition. After the fix, all tests pass and coverage thresholds are met. The pattern matches existing target conditions like "restrained" and "stunned".
follow_ups:
  - Verify integration with the Dodge action handler that applies the "dodge" condition to combatants.
  - Consider whether the attacker-can-see-target prerequisite (PHB Dodge rules) needs enforcement here or upstream.
