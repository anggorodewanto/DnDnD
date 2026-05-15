finding_id: I-C03
status: done
files_changed:
  - internal/narration/template_service.go
  - internal/narration/template_service_test.go
  - internal/narration/template_handler.go
  - internal/narration/template_handler_test.go
test_command_that_validates: go test ./internal/narration/ -run "CrossCampaign" -v
acceptance_criterion_met: yes
notes: Added campaignID parameter to Get/Update/Delete/Duplicate/Apply service methods. Each method now loads the template by ID, then verifies tpl.CampaignID == campaignID before proceeding; mismatch returns ErrTemplateNotFound (404). The handler extracts campaign_id from a required query parameter (matching the existing List endpoint pattern). Five new cross-campaign tests confirm the fix. All existing tests updated to pass the campaign_id. `make test` and `make cover-check` pass (narration package at 94.52%).
follow_ups:
  - Consider extracting campaign_id from auth middleware/context instead of query param for stronger enforcement
  - Integration test with real DB to confirm the check works end-to-end
