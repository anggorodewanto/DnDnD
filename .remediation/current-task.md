finding_id: F-H04
severity: High
title: Free-object interaction whitelist is too permissive / English-only
location: internal/combat/interact.go:13-52
spec_ref: spec §Free Object Interaction lines 1198-1201; Phase 74
problem: |
  autoResolvablePatterns matches by prefix on "open", "grab", etc. A player typing "/interact open the locked treasure chest" auto-resolves even though the lock state matters.
suggested_fix: |
  Reject auto-resolve if the description contains "lock", "trap", "stuck", "barred" — route to DM queue instead.
acceptance_criterion: |
  An interaction containing "locked" is NOT auto-resolved (routes to DM queue). An interaction "open the door" IS auto-resolved. Tests demonstrate both.
