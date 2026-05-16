finding_id: I-H07
severity: High
title: Narration & message-player handlers trust author_user_id from request body
location: internal/narration/handler.go:49-91; internal/messageplayer/handler.go:32-71
spec_ref: Phase 100a / Phase 101 — author attribution
problem: |
  The request payload includes author_user_id and the service stores it blindly. A DM could post in another DM's name.
suggested_fix: |
  Drop author_user_id from the request body and populate it from the request context (auth.DiscordUserIDFromContext).
acceptance_criterion: |
  The stored author_user_id always matches the authenticated user, regardless of what the request body says. A test demonstrates this.
