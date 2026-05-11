---
id: D-56-drag-release
group: D
phase: 56
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Dragging is not wired into /move

## Finding
`CheckDragTargets`, `FormatDragPrompt`, `DragMovementCost`, `ReleaseDrag` exist as helpers but the `/move` handler (`move_handler.go`) does not call them. Players grappling another creature can move at normal speed without prompt; grappled targets are not dragged or released.

## Code paths cited
- `internal/combat/grapple_shove.go` — `CheckDragTargets`, `FormatDragPrompt`, `DragMovementCost`, `ReleaseDrag` (orphaned)
- `internal/discord/move_handler.go` — missing drag invocation
- `internal/discord/shove_handler.go` — grapple wired correctly (already)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 56)
- `docs/dnd-async-discord-spec.md` Grapple / Drag rules

## Acceptance criteria (test-checkable)
- [ ] When a grappling combatant invokes `/move`, `CheckDragTargets` runs and emits the drag prompt
- [ ] Drag movement cost (per `DragMovementCost`) is applied to the grappler's available speed
- [ ] Grappled target tile updates along the grappler's path
- [ ] `/move` (or explicit release surface) routes to `ReleaseDrag` and frees the grappled target
- [ ] Test in `internal/discord/move_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `/move` is already wired for opportunity attacks (D-55) — drag wiring must not regress OA hooks.

## Notes
None.
