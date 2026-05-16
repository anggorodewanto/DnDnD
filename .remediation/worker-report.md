# Worker Report: H-H02 — DM approve/deny buttons have no role check

## Status: ✅ FIXED

## Summary

`HandleDMApprove` and `HandleDMDeny` in `internal/discord/asi_handler.go` now verify that the interacting user is the campaign's DM before processing the action. Non-DM users receive an ephemeral rejection message.

## Changes

### `internal/discord/asi_handler.go`
- Added `dmUserFunc func(guildID string) string` field to `ASIHandler` struct.
- Added `SetDMUserFunc` setter method (nil-safe; nil skips the check for backwards compat).
- Added `isDMUser` helper: returns true if `dmUserFunc` is nil, or if the DM user ID is empty, or if the interacting user matches the DM.
- Added DM identity guard at the top of `HandleDMApprove` and `HandleDMDeny`.

### `internal/discord/asi_handler_test.go`
- Added `TestASIHandler_HandleDMApprove_RejectsNonDM`: verifies a non-DM user gets an ephemeral rejection and `ApproveASI` is never called.
- Added `TestASIHandler_HandleDMDeny_RejectsNonDM`: same for deny path.

### `cmd/dndnd/discord_handlers.go`
- Added `dmUserFunc` field to `discordHandlerDeps`.
- Wired `SetDMUserFunc` on the ASI handler after construction.

### `cmd/dndnd/main.go`
- Extracted `dmUserFunc` closure to a local variable (reused by both `buildDiscordHandlers` and `buildRegistrationDeps`).
- Passed `dmUserFunc` into `discordHandlerDeps`.

## TDD Workflow

1. **Red:** Wrote two failing tests (`RejectsNonDM` for approve and deny). Compilation failed because `SetDMUserFunc` didn't exist.
2. **Green:** Implemented `dmUserFunc` field, `SetDMUserFunc`, `isDMUser`, and the guards. Tests pass.
3. **Verify:** `make test` ✅ — all tests pass. `make cover-check` ✅ — coverage thresholds met.
