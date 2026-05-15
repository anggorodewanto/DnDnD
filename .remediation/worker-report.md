finding_id: B-C02
status: done
files_changed:
  - internal/dice/dice.go
  - internal/dice/dice_test.go
test_command_that_validates: go test ./internal/dice/ -run TestParseExpression_RejectsZeroSidesOrCount -v
acceptance_criterion_met: yes
notes: Added validation in ParseExpression that rejects Count < 1 or Sides < 1 with a descriptive error message. The fix is a single early-return guard inside the dice group parsing loop. Test was written first (red), confirmed failing for "1d0", "0d6", and "0d0", then the fix was applied (green). Full test suite and coverage checks pass. Package coverage remains at 92.1%.
follow_ups:
  - Consider adding a defensive guard in rollGroups before calling randFn(g.Sides) as belt-and-suspenders protection against future callers bypassing ParseExpression.
