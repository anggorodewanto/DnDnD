---
id: C-31-fall-damage-unwired
group: C
phase: 31
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# FallDamage helper exists but is never invoked in production

## Finding
`FallDamage` in `internal/combat/altitude.go` is fully implemented and unit-tested but is referenced only by `altitude.go` and `altitude_test.go`. When a flying combatant goes prone or otherwise loses fly speed, no fall damage is applied — that branch of the Phase 31 spec is dead code.

## Code paths cited
- `internal/combat/altitude.go` — `FallDamage` defined
- `internal/combat/altitude_test.go` — only other reference
- `internal/discord/fly_handler.go` — never calls `FallDamage`
- Prone-application paths (`condition.go` ApplyCondition, damage drop-to-0 in `concentration.go:385`) — should trigger fall for airborne tokens

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 31 altitude & flying)
- `.review-state/group-C-phases-29-43.md` Phase 31 findings

## Acceptance criteria (test-checkable)
- [ ] When an airborne combatant gains the `prone` condition, fall damage is computed via `FallDamage` and applied through the damage pipeline
- [ ] When a flying combatant loses fly speed (e.g. spell ends, incapacitation) while altitude > 0, fall damage is applied and altitude resets to 0
- [ ] Existing `FallDamage` unit tests still pass
- [ ] Integration test in `internal/discord/fly_handler_test.go` (or equivalent) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-prone-on-drop — prone application at drop-to-0 should also trigger fall if airborne
- C-40-charmed-attack / C-40-frightened-move — none

## Notes
Hook should fire on every transition into `prone` or every loss-of-fly-speed event, not only on the `/fly` handler path. Centralize so all callers reuse the same logic.
