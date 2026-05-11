---
id: C-30-occupant-size
group: C
phase: 30
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Move handler hardcodes every occupant as SizeMedium

## Finding
`buildOccupants()` in `move_handler.go:753` assigns `pathfinding.SizeMedium` to every occupant regardless of the underlying creature's actual size category. As a result, Phase 29's "size difference >= 2" pass-through rule never fires in production — Tiny and Large/Huge combatants block each other identically to Medium.

## Code paths cited
- `internal/discord/move_handler.go:753` — hardcoded `SizeCategory: pathfinding.SizeMedium` with comment "would look up creature size"
- `internal/pathfinding/pathfinding.go` — size-diff pass-through rule consumer

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 29 pathfinding + Phase 30 movement)
- `.review-state/group-C-phases-29-43.md` Phase 30 findings

## Acceptance criteria (test-checkable)
- [ ] `buildOccupants()` resolves each occupant's actual `SizeCategory` from the underlying combatant/creature record
- [ ] A Tiny mover can pass through a Large blocker's tile (size diff >= 2) end-to-end via `/move`
- [ ] A Medium mover is still blocked by a Medium occupant (size diff < 2 unchanged)
- [ ] Test in `internal/discord/move_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None directly; future spellcasting / AoE handlers may need the same size lookup helper

## Notes
Solve once with a size-resolution helper that both `buildOccupants` and any future occupant-aware handler (AoE, flying, etc.) can reuse.
