---
id: E-59-aoe-pending-saves
group: E
phase: 59
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# AoE pending saves not persisted or pinged to players

## Finding
`CastAoE` computes `result.PendingSaves` (DC + cover bonus) but the Discord handler never persists them as `pending_saves` rows nor pings affected players via `/save`. Only damage-triggered concentration saves currently write pending_save rows. The spec call-out "Apply damage/effects once all saves resolved" is unreachable from `/cast` AoE today, and DM-rolled enemy saves have no flow back from the dashboard tying to the spell's pending saves.

## Code paths cited
- internal/combat/aoe.go — `CastAoE`, `ResolveAoESaves` (PendingSaves produced)
- internal/discord/cast_handler.go:239-285 — `handleAoECast` (only formats log; does not persist saves)
- internal/discord/cast_handler.go:283-285 — log-only formatting site
- internal/combat/concentration.go — `MaybeCreateConcentrationSaveOnDamage` (the only writer of pending_saves today)

## Spec / phase-doc anchors
- docs/phases.md — Phase 59 ("Spell Casting — AoE & Saves"); "Apply damage/effects once all saves resolved"

## Acceptance criteria (test-checkable)
- [ ] AoE `/cast` persists one `pending_saves` row per affected combatant with DC + cover bonus
- [ ] Affected player combatants are pinged with a `/save` prompt
- [ ] DM-rolled enemy saves resolve back to the spell's pending saves and trigger damage/effects once all resolved
- [ ] Test in `internal/discord/cast_handler_test.go` (or combat aoe-handler test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-66b-cast-extended-flag (cast_handler / commands.go touch points)
- E-67-zone-* tasks (cast_handler also creates spell zones on AoE)

## Notes
PendingSaves struct already exists on the result; the gap is the persistence + prompt layer plus the resolution-completion hook.
