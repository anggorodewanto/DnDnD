---
id: D-48b-flurry-of-blows
group: D
phase: 48b
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Flurry of Blows has no Discord wiring

## Finding
`combatService.FlurryOfBlows` is never called by any handler. PatientDefense and StepOfTheWind are wired into `bonus_handler.go`, but Flurry of Blows is not.

## Code paths cited
- `internal/combat/monk.go` — `FlurryOfBlows` (orphaned)
- `internal/discord/bonus_handler.go` — dispatch table missing flurry-of-blows

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 48b)
- `docs/dnd-async-discord-spec.md` Monk Ki — Flurry of Blows

## Acceptance criteria (test-checkable)
- [ ] `/bonus flurry-of-blows` (post-Attack-action) routes to `combatService.FlurryOfBlows`
- [ ] Ki cost is deducted; two bonus unarmed strikes are granted per spec
- [ ] Test in `internal/discord/bonus_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `internal/discord/bonus_handler.go` is shared with rage, lay-on-hands, bardic, wild-shape wirings.

## Notes
Spec requires Attack-action precondition; ensure dispatch enforces that.
