---
id: E-67-zone-triggers
group: E
phase: 67
severity: HIGH
status: in_review
owner: opus 4.7 (1M) — ZONES-impl
reviewer:
last_update: 2026-05-11
---

# Zone enter / start-of-turn triggers never fire (and round-reset never invoked)

## Finding
`Service.CheckZoneTriggers` has no production callers, so damage-on-enter and damage-at-turn-start zone effects defined in zone_definitions.go (Spirit Guardians, Wall of Fire, Cloud of Daggers, Moonbeam, Stinking Cloud) never actually fire. `ResetZoneTriggersForRound` also has no callers — even if triggers fired, the per-round dedupe map (`triggered_this_round`) never resets.

## Code paths cited
- internal/combat/zone.go — `Service.CheckZoneTriggers` (no production caller)
- internal/combat/zone.go — `ResetZoneTriggersForRound` (no production caller)
- internal/combat/zone_definitions.go — Spirit Guardians, Wall of Fire, Cloud of Daggers, Moonbeam, Stinking Cloud trigger definitions

## Spec / phase-doc anchors
- docs/phases.md — Phase 67 ("Spell Effect Zones"); enter and start-of-turn damage triggers

## Acceptance criteria (test-checkable)
- [ ] Movement into a damaging zone invokes `CheckZoneTriggers` and applies the defined effect
- [ ] Start-of-turn triggers fire for combatants inside damaging zones
- [ ] Per-round dedupe map is reset at the start of each round via `ResetZoneTriggersForRound`
- [ ] A combatant cannot be hit twice in the same round by the same enter-trigger
- [ ] Test in `internal/combat/zone_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-silence-zone-type, E-67-zone-render-on-map, E-67-zone-anchor-follow, E-67-zone-cleanup
- E-71-readied-action-expiry (both touch turn-start/round-start initiative hooks)

## Notes
Wire `CheckZoneTriggers` into the movement path and turn-start path; wire `ResetZoneTriggersForRound` into the round-advance path.
