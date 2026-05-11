---
id: D-57-cunning-action-hide
group: D
phase: 57
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Cunning-action Hide is not wired into Discord

## Finding
Cunning-action hide (`/bonus cunning-action hide`) is not wired into Discord. Same gap as the broader Cunning Action wiring (D-54-cunning-action), but tracked here because Phase 57 specifically requires the bonus-action hide surface for rogues.

## Code paths cited
- `internal/combat/standard_actions.go` — `CunningAction` hide-mode (orphaned)
- `internal/combat/standard_actions.go:312-442` — underlying Hide service
- `internal/discord/bonus_handler.go` — dispatch missing cunning-action hide

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 57)
- `docs/dnd-async-discord-spec.md` Cunning Action / Stealth interaction

## Acceptance criteria (test-checkable)
- [ ] `/bonus cunning-action hide` routes to the Cunning Action hide path
- [ ] Rogue level 2+ gate is enforced
- [ ] Stealth check and hidden-flag side effects match the `/action hide` flow (D-57)
- [ ] Test in `internal/discord/bonus_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Overlaps with D-54-cunning-action (broader cunning-action wiring) and D-57-hide-action.

## Notes
If D-54-cunning-action is implemented first to cover all three modes (dash/disengage/hide), this task may close as duplicate. Keep separate until implementer confirms.
