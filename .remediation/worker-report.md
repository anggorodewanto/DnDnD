# Worker Report: G-H09

**Finding:** Individual /rest only checks if the caller is a combatant (`ActiveEncounterForUser`), not if any encounter is active in the campaign. A bystander could /rest while their party is mid-fight.

**Fix applied:**

1. Added `campaignEncounterChecker rest.PartyEncounterChecker` field to `RestHandler` struct.
2. Added `SetCampaignEncounterChecker` setter method.
3. Added campaign-level guard in `Handle()` — after the per-user check, calls `HasActiveEncounter(ctx, campaign.ID)` and rejects with the same message if any encounter is active.

**Files changed:**
- `internal/discord/rest_handler.go` — struct field, setter, and campaign-level check in Handle.
- `internal/discord/rest_handler_test.go` — new test `TestRestHandler_BlockedWhenCampaignHasActiveEncounter` (user NOT a combatant, but campaign has active encounter → rejected).

**TDD cycle:**
- Red: test compiled but `SetCampaignEncounterChecker` undefined → build failure.
- Green: added field, setter, and check → test passes.
- `make test` ✅ all pass.
- `make cover-check` ✅ thresholds met.

**No commits made.**
