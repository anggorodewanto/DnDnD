# Worker Report: I-H06 — Cross-tenant reads on character overview / narration history / message history

## Status: FIXED

## Problem
The `characteroverview.Handler.Get`, `narration.Handler.History`, and `messageplayer.Handler.History` endpoints accepted `campaign_id` from query params without verifying the authenticated DM owns that campaign. A DM of campaign A could read data belonging to campaign B.

## Fix Applied

### Approach
Added a `CampaignVerifier` interface to each handler package with a single method:
```go
type CampaignVerifier interface {
    IsCampaignDM(ctx context.Context, discordUserID, campaignID string) (bool, error)
}
```

Injected via functional options (`WithCampaignVerifier`) to maintain backward compatibility with existing callers. Each handler checks ownership after parsing `campaign_id` and before querying data. Returns 403 if the check fails.

### Files Modified
- `internal/characteroverview/handler.go` — Added `CampaignVerifier` interface, `HandlerOption`, ownership check in `Get`
- `internal/characteroverview/handler_test.go` — Added `TestHandler_Get_ForbiddenWhenNotCampaignDM`, `TestHandler_Get_AllowedWhenCampaignDM`
- `internal/narration/handler.go` — Added `CampaignVerifier` interface, `HandlerOption`, ownership check in `History`
- `internal/narration/handler_test.go` — Added `TestHandler_History_ForbiddenWhenNotCampaignDM`, `TestHandler_History_AllowedWhenCampaignDM`
- `internal/messageplayer/handler.go` — Added `CampaignVerifier` interface, `HandlerOption`, ownership check in `History`
- `internal/messageplayer/handler_test.go` — Added `TestHandler_History_ForbiddenWhenNotCampaignDM`, `TestHandler_History_AllowedWhenCampaignDM`
- `cmd/dndnd/main.go` — Wired `dashboardCampaignLookup{queries}` as the verifier for all three handlers

### Verification
- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
- `go build ./...` — compiles cleanly

### TDD Sequence
1. **Red:** Wrote 6 new tests (2 per package) asserting 403 for cross-campaign access and 200 for legitimate access. Tests failed to compile (undefined symbols).
2. **Green:** Implemented `CampaignVerifier` interface + `WithCampaignVerifier` option + ownership check in each handler. All tests pass.
3. **Wiring:** Connected `dashboardCampaignLookup` (which already implements `IsCampaignDM`) to each handler in `main.go`.
