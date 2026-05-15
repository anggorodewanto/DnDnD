finding_id: D-C03
status: done
files_changed:
  - internal/check/check.go
  - internal/check/check_test.go
test_command_that_validates: go test ./internal/check/ -run "TestSingleCheck_Rage" -v
acceptance_criterion_met: yes
notes: Added `IsRaging bool` and `Ability string` fields to `SingleCheckInput`. When `IsRaging` is true and `Ability` equals "str" (case-insensitive), the roll mode is combined with Advantage using the existing `dice.CombineRollModes` function, matching the pattern used for exhaustion/condition effects. Three tests cover: rage+STR skill check gets advantage, rage+non-STR gets no advantage, and rage+raw STR check gets advantage. All existing tests continue to pass, `make test` and `make cover-check` succeed.
follow_ups:
  - Wire `IsRaging` and `Ability` population in the Discord handler that calls SingleCheck (separate concern per task brief)
  - Consider whether the FES/ProcessEffects path should eventually replace this direct field check for consistency
