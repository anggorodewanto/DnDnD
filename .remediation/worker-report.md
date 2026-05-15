finding_id: H-C05
status: done
files_changed:
  - internal/levelup/handler.go
  - internal/levelup/handler_test.go
test_command_that_validates: go test ./internal/levelup/ -run TestHandler_HandleLevelUp_RejectsLevelAbove20 -v
acceptance_criterion_met: yes
notes: Added `|| req.NewLevel > 20` to the existing validation guard in HandleLevelUp (line 109). The handler now returns 400 with message "character_id, class_id, and new_level (1-20) are required" for any newLevel outside [1,20]. A new test confirms newLevel=21 is rejected. All existing tests continue to pass, and `make cover-check` reports the levelup package at 90.37%.
follow_ups:
  - Consider adding an equivalent server-side check in the Discord slash-command path if level-up can be triggered there.
  - The HTML template already has max="20" on the input, but client-side validation alone is insufficient.
