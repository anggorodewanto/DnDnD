finding_id: H-H02
severity: High
title: DM approve/deny buttons have no role check
location: internal/discord/asi_handler.go:456 (HandleDMApprove), 524 (HandleDMDeny)
spec_ref: spec §"DM approval" line 2497
problem: |
  Anyone who can see #dm-queue can click Approve or Deny — no check that the interacting user is the campaign's DM.
suggested_fix: |
  Look up the campaign for the guild and verify interaction.Member.User.ID matches the campaign's DM Discord ID.
acceptance_criterion: |
  HandleDMApprove/HandleDMDeny reject interactions from non-DM users. A test demonstrates this.
