finding_id: A-C02
severity: Critical
title: Dashboard approval endpoints aren't scoped to the DM's own campaign
location: internal/dashboard/approval_handler.go:230-338 (Approve/Reject/RequestChanges)
spec_ref: spec §Authentication & Authorization (line 65); Phase 16
problem: |
  `RequireDM` checks that the user is the DM of *some* campaign, but the handler routes (`/dashboard/api/approvals/{id}/approve|reject|request-changes`) look up the approval row by `id` only — no check that the player_character belongs to a campaign the authenticated DM owns. A DM of campaign A can approve, reject, or retire any character in campaign B by guessing/learning UUIDs. No test covers this.
suggested_fix: |
  Resolve the approval row, get its campaign_id, and verify the authenticated DM owns that campaign (use `IsCampaignDM` or compare `detail.CampaignID` against the DM's campaign) before mutating. Add a regression test for cross-campaign rejection.
acceptance_criterion: |
  Approve/Reject/RequestChanges endpoints return 403 when the authenticated DM does not own the campaign that the approval row belongs to. A test demonstrates this by having DM-A try to approve a character in DM-B's campaign.
