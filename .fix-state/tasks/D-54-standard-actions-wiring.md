---
id: D-54-standard-actions-wiring
group: D
phase: 54
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Eight standard actions not wired into Discord

## Finding
None of the eight services (Dash, Disengage, Dodge, Help, Hide, Stand, DropProne, Escape) are wired into Discord. `action_handler.go` does not switch on subcommand names (only `cancel` and `ready` are handled; everything else becomes freeform → DM queue). Help text in `help_content.go:131-141` advertises commands that are not actually reachable.

## Code paths cited
- `internal/combat/standard_actions.go` — 927 lines of service code, all eight actions implemented
- `internal/discord/action_handler.go` — dispatch table missing seven actions (Hide tracked separately under D-57)
- `internal/discord/help_content.go:131-141` — advertises unreachable commands

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 54)
- `docs/dnd-async-discord-spec.md` Standard Actions section

## Acceptance criteria (test-checkable)
- [ ] `/action dash` routes to `DashService`
- [ ] `/action disengage` routes to `DisengageService`
- [ ] `/action dodge` routes to `DodgeService`
- [ ] `/action help <target>` routes to `HelpService`
- [ ] `/action stand` routes to `StandService`
- [ ] `/action drop-prone` routes to `DropProneService`
- [ ] `/action escape` routes to `EscapeService`
- [ ] Resource / condition / adjacency / movement validation from each service surfaces as user-visible errors
- [ ] Test in `internal/discord/action_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Hide is tracked under D-57; Cunning Action (dash/disengage/hide bonus variant) under D-54-cunning-action.

## Notes
This is the largest single dispatch refactor in group D — coordinate ordering with D-50, D-53, D-57.
