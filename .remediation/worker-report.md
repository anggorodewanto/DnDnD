# Worker Report: I-H11

## Finding
DM character creation handler (`HandleCreate`) did not verify the authenticated user owns the `campaign_id` from the request body. Any authenticated DM could create characters in another DM's campaign.

## Fix Applied

### Files Modified
1. **internal/dashboard/charcreate_handler.go**
   - Added `dmVerifier DMVerifier` field to `CharCreateHandler`
   - Added `SetDMVerifier(v DMVerifier)` method
   - In `HandleCreate`: after parsing the request body, calls `h.dmVerifier.IsCampaignDM(ctx, userID, campaignID)` and returns 403 if the user doesn't own the campaign

2. **internal/dashboard/charcreate_handler_test.go**
   - Added `TestCharCreateHandler_HandleCreate_ForbiddenWhenNotCampaignDM`: DM of campaign A tries to create a character in campaign B → gets 403, service is never called
   - Added `mockDMVerifierForCreate` test helper implementing `DMVerifier`

3. **cmd/dndnd/main.go**
   - Wired `charCreateHandler.SetDMVerifier(dmVerifier)` so production uses the real campaign ownership check

## TDD Cycle
- **Red:** Test compiled but failed (method `SetDMVerifier` did not exist)
- **Green:** Added field, setter, and ownership check → test passes
- **Verify:** `make test` ✅ | `make cover-check` ✅

## Acceptance Criterion
✅ A DM creating a character in another DM's campaign gets 403. A test demonstrates this.
