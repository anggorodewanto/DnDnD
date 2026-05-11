---
id: D-46-rage-auto-end-quiet
group: D
phase: 46
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Rage auto-end-when-quiet fires prematurely

## Finding
`RageAttackedThisRound` and `RageTookDamageThisRound` are reset by `DecrementRageRound` / `ClearRageFromCombatant` but never set to true anywhere — no attack or damage path updates them. `ShouldRageEndOnTurnEnd` (which requires `!attacked && !tookDamage`) therefore fires as soon as rage exists for a full round, ending rage prematurely except on the activation turn.

## Code paths cited
- `internal/combat/rage.go` — `RageAttackedThisRound`, `RageTookDamageThisRound`, `ShouldRageEndOnTurnEnd`, `DecrementRageRound`, `ClearRageFromCombatant`
- `internal/combat/initiative.go:396` — turn-end hook
- `internal/combat/attack.go` — attack pipeline (missing set)
- `internal/combat/damage.go` — damage pipeline (missing set)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 46)
- `docs/dnd-async-discord-spec.md` Rage auto-end-when-quiet rule

## Acceptance criteria (test-checkable)
- [ ] When the raging combatant makes an attack during a round, `RageAttackedThisRound` is set true
- [ ] When the raging combatant takes damage during a round, `RageTookDamageThisRound` is set true
- [ ] Rage does NOT end on turn-end of a round in which the raging combatant attacked or took damage
- [ ] Rage DOES end on turn-end of a quiet round (no attacks, no damage)
- [ ] Test in `internal/combat/rage_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Touches `internal/combat/attack.go` and `internal/combat/damage.go` which are hot integration paths.

## Notes
The two flag setters are independent — both must be wired.
