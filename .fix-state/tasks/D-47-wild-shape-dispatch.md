---
id: D-47-wild-shape-dispatch
group: D
phase: 47
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Wild Shape has no Discord wiring

## Finding
`ActivateWildShape` and `RevertWildShapeService` are not registered in any `/bonus` dispatch. `bonus_handler.go` only routes rage / martial-arts / step-of-the-wind / patient-defense / font-of-magic / lay-on-hands / bardic-inspiration. Players cannot invoke `/bonus wild-shape` today.

## Code paths cited
- `internal/combat/wildshape.go` — `ActivateWildShape`, `RevertWildShapeService` (orphaned)
- `internal/discord/bonus_handler.go` — dispatch table missing wild-shape
- `internal/combat/concentration.go:357-362` — auto-revert hook (already present)
- `internal/combat/spellcasting.go:401-403` — spellcasting block (already present)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 47)
- `docs/dnd-async-discord-spec.md` Wild Shape section

## Acceptance criteria (test-checkable)
- [ ] `/bonus wild-shape <beast>` routes to `ActivateWildShape` and transforms the druid
- [ ] `/bonus wild-shape revert` (or equivalent surface) routes to `RevertWildShapeService`
- [ ] Test in `internal/discord/bonus_handler_test.go` (or new dispatch test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Also touches `internal/discord/bonus_handler.go`, which is shared with multiple group-D wirings (rage, monk, lay-on-hands, bardic).

## Notes
Audit also flagged that concentration is implicitly maintained because nothing breaks it on transformation; verify behavior matches spec while wiring tests.
