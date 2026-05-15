finding_id: A-H01
status: done
files_changed:
  - internal/registration/service.go
  - internal/registration/resubmit_test.go
  - internal/dashboard/approval_store.go
  - internal/dashboard/resubmit_test.go
test_command_that_validates: go test ./internal/registration/ -run "TestValidTransitions" -short && go test ./internal/dashboard/ -run "TestDBApprovalStore_ResubmitToPending" -short
acceptance_criterion_met: yes
notes: Added "changes_requested" -> "pending" and "rejected" -> "pending" entries to both validTransitions (registration/service.go) and validApprovalTransitions (dashboard/approval_store.go). Added a Resubmit method to Service and ResubmitToPending to DBApprovalStore that invoke the existing transitionStatus with target "pending". Unit tests confirm both transitions are allowed. make test and make cover-check pass cleanly.
follow_ups: []
