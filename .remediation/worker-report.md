finding_id: A-C02
status: done
files_changed:
  - internal/dashboard/approval.go
  - internal/dashboard/approval_store.go
  - internal/dashboard/approval_handler.go
  - internal/dashboard/approval_handler_test.go
  - internal/dashboard/approval_edge_test.go
  - internal/dashboard/approval_iter2_test.go
  - internal/dashboard/approval_ws_test.go
test_command_that_validates: go test ./internal/dashboard/ -run TestApprovalEndpoints_RejectCrossCampaign -v
acceptance_criterion_met: yes
notes: |
  Added CampaignID field to ApprovalEntry struct and populated it from the DB row in both ListPendingApprovals and GetApprovalDetail. Added a checkCampaignOwnership helper to ApprovalHandler that resolves the DM's campaign (via resolveCampaign, which uses CampaignLookup or the static campaignID) and compares it against the approval's CampaignID, returning 403 on mismatch. The check is called in Approve and parseFeedbackRequest (shared by RequestChanges and Reject). Existing tests needed CampaignID set in their fixtures to match the handler's campaign.
follow_ups:
  - "GetApproval (GET /{id}) does not enforce campaign ownership — a DM can read details of approvals in other campaigns. Severity: Low (read-only, no mutation)."
