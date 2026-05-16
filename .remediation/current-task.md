finding_id: H-H13
severity: High
title: /api/levelup/asi/approve endpoint has no character-owner / DM check
location: internal/levelup/handler.go:129 (HandleApproveASI)
spec_ref: spec §"DM approval" line 2497
problem: |
  The handler takes CharacterID from the JSON body without verifying the DM session is authorised for that character's campaign. A DM of campaign A can approve an ASI for a character in campaign B.
suggested_fix: |
  Resolve campaign from the character row and verify the authenticated DM's discord_user_id matches.
acceptance_criterion: |
  HandleApproveASI returns 403 when the DM doesn't own the character's campaign. A test demonstrates this.
