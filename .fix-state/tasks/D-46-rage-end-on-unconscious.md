---
id: D-46-rage-end-on-unconscious
group: D
phase: 46
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Rage end-on-unconscious is not wired into damage/death pipeline

## Finding
`ShouldRageEndOnUnconscious` exists in `internal/combat/rage.go` but no caller hooks it into the damage / death pipeline. A raging combatant who drops to 0 HP retains the `IsRaging` flag.

## Code paths cited
- `internal/combat/rage.go` — `ShouldRageEndOnUnconscious` (unused)
- `internal/combat/damage.go` — death/unconscious transition (no caller)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 46)
- `docs/dnd-async-discord-spec.md` Rage end conditions

## Acceptance criteria (test-checkable)
- [ ] When a raging combatant drops to 0 HP (unconscious), rage ends and `IsRaging` becomes false
- [ ] Combat-log line is emitted for rage ending due to unconsciousness
- [ ] Test in `internal/combat/rage_test.go` (or damage integration test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Damage pipeline (`internal/combat/damage.go`) is also touched by other features (death saves, concentration).

## Notes
Service-level wiring only — no Discord surface change.
