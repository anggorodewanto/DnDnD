finding_id: I-C03
severity: Critical
title: Narration-template Get/Update/Delete/Duplicate/Apply leak across campaigns
location: internal/narration/template_handler.go:110-193; internal/narration/template_service.go:89-143
spec_ref: Phase 100b "Templates are campaign-scoped"
problem: |
  Only Create and List accept campaign_id; Get, Update, Delete, Duplicate, and Apply look the template up purely by UUID and never check that the row's campaign_id matches the requesting DM's campaign.
suggested_fix: |
  Require campaign_id on every template endpoint and verify tpl.CampaignID == in.CampaignID before mutating. Return ErrTemplateNotFound on mismatch.
acceptance_criterion: |
  Get/Update/Delete/Duplicate/Apply return 404 (or 403) when the template belongs to a different campaign than the authenticated DM's. A test demonstrates cross-campaign access is blocked.
