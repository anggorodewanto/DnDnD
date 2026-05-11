---
id: E-67-zone-anchor-follow
group: E
phase: 67
severity: HIGH
status: in_review
owner: opus 4.7 (1M) — ZONES-impl
reviewer:
last_update: 2026-05-11
---

# Combatant-anchored zones don't follow the caster on /move

## Finding
`Service.UpdateZoneAnchor` has no production callers. Spirit Guardians (`AnchorMode:"combatant"`) won't follow the caster on `/move`. `UpdateCombatantPosition` does not invoke `UpdateZoneAnchor`.

## Code paths cited
- internal/combat/zone.go — `Service.UpdateZoneAnchor` (no production caller)
- internal/combat/zone_definitions.go — Spirit Guardians `AnchorMode:"combatant"`
- internal/combat — `UpdateCombatantPosition` (movement path; missing anchor sync)

## Spec / phase-doc anchors
- docs/phases.md — Phase 67 ("Spell Effect Zones"); combatant-anchored zones move with caster

## Acceptance criteria (test-checkable)
- [ ] Moving a caster with an active combatant-anchored zone updates the zone's anchor position
- [ ] Spirit Guardians area follows the caster across `/move` operations
- [ ] Test in `internal/combat/zone_test.go` (or movement test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-silence-zone-type, E-67-zone-render-on-map, E-67-zone-triggers, E-67-zone-cleanup
- B-26b-* and any tasks touching `UpdateCombatantPosition`

## Notes
Service method exists; need only the caller wiring.
