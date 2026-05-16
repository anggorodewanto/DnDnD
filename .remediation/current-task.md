finding_id: I-H06
severity: High
title: Cross-tenant reads on character overview / narration history / message history
location: internal/characteroverview/handler.go:35-47; internal/narration/handler.go:95-118; internal/messageplayer/handler.go:74-108
spec_ref: Spec §65 "System verifies the authenticated Discord user ID matches the campaign's designated DM."
problem: |
  RequireDM only verifies the caller is a DM. These handlers accept campaign_id as a query arg and return data without checking the caller is the DM of that specific campaign.
suggested_fix: |
  Verify campaign ownership against the authenticated user before returning rows.
acceptance_criterion: |
  A DM requesting data for another campaign's characters/narration/messages gets 403. A test demonstrates this.
