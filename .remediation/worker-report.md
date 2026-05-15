finding_id: H-C04
status: done
files_changed:
  - internal/ddbimport/service.go
  - internal/ddbimport/service_test.go
  - internal/ddbimport/coverage_test.go
test_command_that_validates: go test ./internal/ddbimport/ -run TestService_Import_FirstImportStagesPending -v
acceptance_criterion_met: yes
notes: First-time DDB imports now route through the same pending_ddb_imports staging path that re-syncs use. The pendingImport struct was extended with createParams/isCreate fields. ApproveImport distinguishes create vs update by the isCreate flag (and loadPendingImport uses CharacterID==uuid.Nil when loading from DB). Existing tests were updated to call ApproveImport before asserting on CreateCharacter behavior. All tests pass and coverage thresholds are met.
follow_ups:
  - Callers of Import (e.g. Discord handler) may need updating to handle PendingImportID on first imports and surface the approval flow to the DM
  - Consider adding an integration/e2e test that exercises the full Discord slash-command → staging → DM-approve flow for first imports
