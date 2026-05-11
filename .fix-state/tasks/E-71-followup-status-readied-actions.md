---
id: E-71-followup-status-readied-actions
group: E
phase: 71
severity: LOW
status: open
owner:
reviewer:
last_update: 2026-05-11
parent: E-71-readied-action-expiry
---

# /status should surface readied actions via FormatReadiedActionsStatus

## Finding
`FormatReadiedActionsStatus` is fully implemented in `internal/combat/readied_action.go` and already covered by unit tests. It just has no caller in the Discord `/status` handler.

## Code paths cited
- internal/combat/readied_action.go:196 — `FormatReadiedActionsStatus`
- internal/discord — `/status` handler (location TBD)

## Acceptance criteria
- [ ] `/status` output for a combatant with an active readied action includes the "⏳ Readied Actions:" block

## Related
- E-71-readied-action-expiry (parent)

## Notes
Out of scope of the batch-2 implementer (discord package was carved out).
