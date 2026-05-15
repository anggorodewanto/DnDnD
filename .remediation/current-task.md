finding_id: A-H05
severity: High
title: Portal token redemption has a TOCTOU race
location: internal/portal/token_service.go:82-90 + internal/portal/token_store.go:81-88
spec_ref: spec §Discord integration; Phase 14
problem: |
  RedeemToken calls ValidateToken (SELECT) then MarkUsed (UPDATE) without a transaction or conditional WHERE. Two concurrent redemptions both pass validate and both succeed at MarkUsed.
suggested_fix: |
  Make MarkUsed atomic: UPDATE portal_tokens SET used = true WHERE id = $1 AND used = false RETURNING id. Treat "no row" as "already used."
acceptance_criterion: |
  MarkUsed uses a conditional UPDATE (WHERE used = false) and returns an error when no row is affected (token already used). A test demonstrates the atomic behavior.
