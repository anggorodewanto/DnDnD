finding_id: I-H11
severity: High
title: DM character creation handler is not protected by DM auth
location: internal/dashboard/charcreate_handler.go:83-103, 112-138
spec_ref: Spec §DM Dashboard; phases 93a/93b
problem: |
  requireAuthHelper only asserts the request has a Discord user ID. Combined with campaign_id from the body, any DM can create a character in another DM's campaign.
suggested_fix: |
  Verify the authenticated DM owns the campaign_id before creating the character.
acceptance_criterion: |
  A DM creating a character in another DM's campaign gets 403. A test demonstrates this.
