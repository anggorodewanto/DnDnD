finding_id: C-C01
status: done
files_changed:
  - internal/combat/attack.go
  - internal/combat/attack_test.go
  - internal/combat/obscurement.go
test_command_that_validates: go test ./internal/combat/ -run TestColToIndex -v
acceptance_criterion_met: yes
notes: Fixed colToIndex in attack.go to use proper base-26 conversion (accumulate result*26 + (ch-'A'+1), then subtract 1 for 0-based). Also fixed the exported ColToIndex in obscurement.go which had identical broken logic — it now delegates to colToIndex. Removed the now-unused "strings" import from obscurement.go. All existing tests continue to pass, and multi-letter columns (AA=26, AB=27, AZ=51, BA=52) now resolve correctly.
follow_ups:
  - none
