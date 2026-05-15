finding_id: A-C01
status: done
files_changed:
  - internal/discord/setup.go
  - internal/discord/setup_test.go
test_command_that_validates: go test ./internal/discord/ -run "TestHandleSetupCommand_Rejects"
acceptance_criterion_met: yes
notes: Added two server-side authorization guards in SetupHandler.Handle after GetCampaignForSetup returns. (1) When a campaign already exists, the invoker's user ID must match the campaign's DMUserID or the request is rejected. (2) When the campaign is auto-created (no prior row), the invoker must have the Discord Administrator permission bit set. A new helper `setupInvokerIsAdmin` reads `interaction.Member.Permissions` which Discord populates on interaction payloads. Existing tests were updated to supply proper Member context matching the authorization requirements.
follow_ups: []

---

**Summary:** Added two early-return authorization checks to the `/setup` handler in `internal/discord/setup.go`. When a campaign already exists for the guild, only the recorded DM user can invoke `/setup`; when no campaign exists (auto-create path), only a server administrator can claim the DM role. A `setupInvokerIsAdmin` helper inspects the `interaction.Member.Permissions` bitfield for the Administrator flag. Four existing tests were updated to include proper `Member` data, and four new tests validate both the rejection and acceptance paths. All tests pass and coverage remains above thresholds.
