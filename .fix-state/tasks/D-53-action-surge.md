---
id: D-53-action-surge
group: D
phase: 53
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Action Surge has no Discord wiring

## Finding
`combatService.ActionSurge` is never called by any handler. `/action surge` is documented in `help_content.go:141` but routes to the freeform path.

## Code paths cited
- `internal/combat/action_surge.go` — service (correct, validates not-already-surged, fighter-only, level 2+)
- `internal/discord/action_handler.go` — missing `surge` route
- `internal/discord/help_content.go:141` — advertises unreachable command
- `internal/refdata/.../turns` schema — `action_surged` column (already present)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 53)
- `docs/dnd-async-discord-spec.md` Action Surge

## Acceptance criteria (test-checkable)
- [ ] `/action surge` routes to `combatService.ActionSurge` for a fighter level 2+
- [ ] Use is deducted; `ActionUsed` is reset to false; `AttacksRemaining` is reset; `ActionSurged` is true
- [ ] Repeat invocation in the same turn is rejected
- [ ] Combat log line is emitted
- [ ] Test in `internal/discord/action_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Shares dispatch refactor with D-50 (Channel Divinity), D-54 (Standard Actions), D-57 (Hide).

## Notes
None.
