# Worker Report: H-H13

## Finding
HandleApproveASI did not verify the authenticated DM owns the character's campaign, allowing a DM of campaign A to approve ASI for a character in campaign B.

## Fix Applied

### `internal/levelup/handler.go`
1. Added `CampaignOwnershipChecker` interface with `GetCampaignDMUserID(ctx, characterID) (string, error)`.
2. Added `ownershipChecker` field to `Handler` struct and `SetOwnershipChecker` setter.
3. In `HandleApproveASI`: when `ownershipChecker` is set, extracts the authenticated user via `auth.DiscordUserIDFromContext`, resolves the character's campaign DM, and returns 403 if they don't match.

### `internal/levelup/handler_test.go`
Added two tests:
- `TestHandler_HandleApproveASI_Forbidden_WrongCampaignDM` — DM of campaign B tries to approve ASI for character in campaign A → 403.
- `TestHandler_HandleApproveASI_Allowed_CorrectCampaignDM` — correct DM approves → 200.

## Verification
- `make test` — all tests pass.
- `make cover-check` — all thresholds met (levelup package at 90.00%).
