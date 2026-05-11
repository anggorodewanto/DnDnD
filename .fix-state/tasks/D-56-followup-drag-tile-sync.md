---
id: D-56-followup-drag-tile-sync
group: D
phase: 56
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: Drag tile-sync along path during /move

## Finding
D-56 main implementation wired `/bonus drag`, `/bonus release-drag`, and `/move` x2 drag cost. The deferred part: when a grappling combatant moves, the grappled target's tile is not synced step-by-step along the path (currently only the final position is updated). Spec calls for the dragged combatant to occupy a tile adjacent to the dragger at every step of the path so visibility / opportunity-attack calculations are correct.

## Code paths cited
- `internal/discord/move_handler.go` — path-following logic; needs to update drag target position per step.
- `internal/combat/drag.go` or wherever drag-target state is stored.

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md (Phase 56 drag rules)

## Acceptance criteria (test-checkable)
- [ ] During multi-tile /move, drag target position is updated at each intermediate step (not just the endpoint)
- [ ] Adjacent-tile invariant: drag target is always within 5ft of the dragger throughout movement
- [ ] Test in `internal/discord/move_handler_drag_test.go` walks a 3-tile path and asserts target tile at each step
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- `internal/discord/move_handler.go` — schedule with other move-related tasks.

## Notes
Surfaced by DISPATCH-D implementer; would require new `move_confirm:` button encoding to carry intermediate drag state. Deferred from D-56 main impl.
