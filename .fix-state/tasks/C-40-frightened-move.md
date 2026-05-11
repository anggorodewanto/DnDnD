---
id: C-40-frightened-move
group: C
phase: 40
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Frightened "can't approach source" never enforced at runtime

## Finding
`ValidateFrightenedMovement` in `condition_effects.go` is defined and unit-tested but is never called from `move_handler.go` or any movement validator. A frightened combatant can still move closer to the source of their fear.

## Code paths cited
- `internal/combat/condition_effects.go` — `ValidateFrightenedMovement` defined
- `internal/combat/condition_effects_test.go` — only consumer
- `internal/discord/move_handler.go` / `internal/combat/movement.go` `ValidateMove` — should reject any step that decreases distance to the fear source

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 40 condition effects, Frightened)
- `.review-state/group-C-phases-29-43.md` Phase 40 findings

## Acceptance criteria (test-checkable)
- [ ] `/move` rejects any path that brings a frightened combatant closer (any step or net distance, per the helper's contract) to the source of their `frightened` condition
- [ ] Frightened combatant may still move parallel/away from source
- [ ] Non-frightened combatant unaffected
- [ ] Test in `internal/discord/move_handler_test.go` (or `movement_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-40-charmed-attack — symmetric source-aware condition validator pattern
- C-30-occupant-size — both touch `move_handler.go`

## Notes
Apply at validation time, not after pathfinding succeeds, so the user gets a clean rejection message rather than a partially-consumed movement budget.
