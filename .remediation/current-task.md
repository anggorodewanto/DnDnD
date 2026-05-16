finding_id: J-H06
severity: High
title: /whisper accepts empty message and spams a dm-queue item
location: internal/discord/whisper_handler.go:61-80
spec_ref: Phase 109
problem: |
  The handler never checks for empty/whitespace message before posting to dm-queue.
suggested_fix: |
  Reject strings.TrimSpace(message) == "" with an ephemeral hint.
acceptance_criterion: |
  /whisper with empty or whitespace-only message returns an error. A test demonstrates this.
