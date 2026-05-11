---
id: D-46-rage-spellcasting-block
group: D
phase: 46
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Rage does not block /cast or drop concentration

## Finding
Spec requires raging characters to be blocked from `/cast` and to drop concentration on rage activation. `spellcasting.go` never checks `IsRaging`, and `concentration.go` has no rage hook. Both behaviors are missing.

## Code paths cited
- `internal/combat/rage.go` — rage service, `IsRaging` flag
- `internal/combat/spellcasting.go` — lacks `IsRaging` check at cast gate
- `internal/combat/concentration.go` — no rage hook on activation
- `internal/discord/bonus_handler.go` — rage activation handler

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 46)
- `docs/dnd-async-discord-spec.md` Rage section: "Block `/cast` and drop concentration while raging."

## Acceptance criteria (test-checkable)
- [ ] `/cast` invocation while `IsRaging` is true is rejected with a clear error
- [ ] Activating rage on a combatant currently concentrating drops the concentration effect
- [ ] Test in `internal/combat/rage_test.go` (or `spellcasting_test.go` / `concentration_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Also touches `internal/discord/bonus_handler.go` (rage activation) and `internal/discord/cast_handler.go` (cast gate).

## Notes
Two distinct behaviors share one finding because both are part of the same spec sentence. If the implementer prefers, this can be split into two PRs; acceptance criteria remain the same.
