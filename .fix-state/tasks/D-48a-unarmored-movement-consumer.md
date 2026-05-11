---
id: D-48a-unarmored-movement-consumer
group: D
phase: 48a
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Unarmored Movement turn-start effect has no production consumer

## Finding
Unarmored Movement is a `TriggerOnTurnStart` `EffectModifySpeed`, but no production caller of `ProcessEffects` for that trigger was found in this audit. The speed bonus may never apply to combatant movement.

## Code paths cited
- `internal/combat/monk.go` — `UnarmoredMovementFeature` (`TriggerOnTurnStart` / `EffectModifySpeed`)
- `internal/combat/effect.go` — `ProcessEffects` (no turn-start production caller located)
- `internal/combat/initiative.go` — turn-start hook site (candidate consumer)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 48a)
- `docs/dnd-async-discord-spec.md` Monk Unarmored Movement

## Acceptance criteria (test-checkable)
- [ ] On turn start for an unarmored monk (no shield, no armor), effective speed includes the Unarmored Movement bonus
- [ ] On turn start when monk is wearing armor / shield, the bonus does NOT apply
- [ ] Test in `internal/combat/monk_test.go` (integration through turn-start hook) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Turn-start consumer code path is shared with other triggers; a generic consumer fix may affect other features.

## Notes
Investigate whether `ProcessEffects(TriggerOnTurnStart, ...)` is actually invoked at turn-start in production code; if not, wire it.
