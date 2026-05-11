---
id: D-54-cunning-action
group: D
phase: 54
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Cunning Action (rogue bonus dash/disengage/hide) not wired

## Finding
`CunningAction` is not wired in `bonus_handler.go` (only step-of-the-wind / patient-defense / etc. are wired). Cunning-action dash/disengage/hide are unreachable from Discord.

## Code paths cited
- `internal/combat/standard_actions.go` — `CunningAction` service (orphaned)
- `internal/discord/bonus_handler.go` — dispatch table missing cunning-action

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 54)
- `docs/dnd-async-discord-spec.md` Cunning Action

## Acceptance criteria (test-checkable)
- [ ] `/bonus cunning-action dash` routes to `CunningAction` with dash mode
- [ ] `/bonus cunning-action disengage` routes to `CunningAction` with disengage mode
- [ ] `/bonus cunning-action hide` routes to `CunningAction` with hide mode
- [ ] Class/level gate (rogue level 2+) is enforced
- [ ] Test in `internal/discord/bonus_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `bonus_handler.go` is shared with D-46 (rage), D-47 (wild-shape), D-48b (flurry), D-49 (bardic), D-52 (lay-on-hands).
- Cunning-action hide overlaps with D-57 hide service wiring.

## Notes
None.
