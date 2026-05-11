---
id: D-52-lay-on-hands-action-vs-bonus
group: D
phase: 52
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Lay on Hands is wired under /bonus instead of /action

## Finding
Lay on Hands is wired under `/bonus lay-on-hands` (`bonus_handler.go:125, 318`) but spec says it is an action. The service still validates `ResourceAction`, so the cost is correct, but the command surface is inconsistent with the spec and with `help_content.go:143`.

## Code paths cited
- `internal/discord/bonus_handler.go:125, 318` — incorrect command surface
- `internal/discord/action_handler.go` — missing `lay-on-hands` route
- `internal/discord/help_content.go:143` — documents as `/action lay-on-hands`
- `internal/combat/lay_on_hands.go` — service (validates `ResourceAction` correctly)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 52)
- `docs/dnd-async-discord-spec.md` Lay on Hands (action)

## Acceptance criteria (test-checkable)
- [ ] `/action lay-on-hands <target> <amount>` routes to `LayOnHandsService`
- [ ] `/bonus lay-on-hands` is either removed or kept as a deprecated alias with a warning (decision to be documented)
- [ ] Cure-poison / cure-disease (5 HP each) work via the action surface
- [ ] Adjacency check (skipped for self) preserved
- [ ] Test in `internal/discord/action_handler_test.go` (and updated `bonus_handler_test.go` if alias kept) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `bonus_handler.go` removal must not break rage / monk / bardic / wild-shape routes.

## Notes
Decide alias policy with the reviewer; spec compliance favors removing the bonus surface.
