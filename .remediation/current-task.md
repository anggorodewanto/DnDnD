finding_id: A-H04
severity: High
title: OAuth callback handler treats any 4xx error from Discord as a generic 403
location: internal/auth/oauth2.go:150-156, 178-182
spec_ref: spec §Authentication & Authorization; Phase 10
problem: |
  HandleCallback never validates that FetchUserInfo returned a non-empty Discord user ID. If Discord returns an empty body, user.ID could be empty string, which then goes into sessions.Create and player_characters.discord_user_id.
suggested_fix: |
  After FetchUserInfo, reject the callback if user.ID == "".
acceptance_criterion: |
  HandleCallback returns an error/redirect when user.ID is empty. A test demonstrates this.
