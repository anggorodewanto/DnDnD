---
id: E-71-readied-action-expiry
group: E
phase: 71
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Readied action expiry not invoked; /status integration unclear; discord missing spell flags

## Finding
- `Service.ExpireReadiedActions` is defined but has no production callers. Spec: "Expires at start of creature's next turn (with expiry notice)" — turn-start path in `initiative.go` does not call it, so readied actions (and any concurrently-held concentration on a readied spell) persist past spec expiry.
- `FormatReadiedActionsStatus` exists but no caller is surfaced in the `/status` handler.
- The discord-side `performReadyAction` offers a free-text description but does not appear to expose the spell-name / slot-level flags from `ReadyActionCommand`, so the readied-spell-with-slot path may be unreachable via Discord.

## Code paths cited
- internal/combat/readied_action.go — `Service.ExpireReadiedActions`, `FormatReadiedActionsStatus`, `ReadyActionCommand` (SpellName / SpellSlotLevel)
- internal/combat/initiative.go — turn-start path (missing `ExpireReadiedActions` call)
- internal/discord/action_handler.go:197-228 — `performReadyAction` (missing spell-name / slot-level wiring)
- `/status` handler (missing `FormatReadiedActionsStatus` invocation)
- db/migrations/20260314120003_add_readied_action_fields.sql

## Spec / phase-doc anchors
- docs/phases.md — Phase 71 ("Readied Actions"); expiry at start of creature's next turn with expiry notice; `/status` integration

## Acceptance criteria (test-checkable)
- [ ] Turn-start path calls `ExpireReadiedActions` and posts an expiry notice
- [ ] Any concentration held by an expiring readied spell is dropped per spec
- [ ] `/status` surfaces readied actions via `FormatReadiedActionsStatus`
- [ ] Discord `/action ready` exposes spell-name and slot-level flags so the readied-spell-with-slot path is reachable
- [ ] Test in `internal/combat/readied_action_test.go` and/or `internal/discord/action_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-zone-triggers (both touch turn-start / round-start hooks in initiative)
- E-66b-cast-extended-flag (concentration interaction)

## Notes
Three connected sub-items rolled into one task per the ids listed.
