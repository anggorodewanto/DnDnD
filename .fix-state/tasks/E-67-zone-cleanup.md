---
id: E-67-zone-cleanup
group: E
phase: 67
severity: HIGH
status: in_review
owner: opus 4.7 (1M) — ZONES-impl
reviewer:
last_update: 2026-05-11
---

# Zone duration expiry and encounter-end cleanup not wired

## Finding
Zone auto-creation in `maybeCreateSpellZone` and AoE handler never set `ExpiresAtRound`. `CleanupExpiredZones` has no callers. `CleanupEncounterZones` has no callers. Zones therefore never expire by duration and persist across encounters.

## Code paths cited
- internal/combat/spellcasting.go:711-741 — `maybeCreateSpellZone` (never sets `ExpiresAtRound`)
- internal/combat/aoe.go:484-508 — AoE auto-create (never sets `ExpiresAtRound`)
- internal/combat/zone.go — `CleanupExpiredZones` (no production caller)
- internal/combat/zone.go — `CleanupEncounterZones` (no production caller)

## Spec / phase-doc anchors
- docs/phases.md — Phase 67 ("Spell Effect Zones"); duration expiry and encounter-end cleanup

## Acceptance criteria (test-checkable)
- [ ] Zone auto-creation sets `ExpiresAtRound` based on the spell's duration
- [ ] `CleanupExpiredZones` is invoked on the round-advance / turn-advance path
- [ ] Zones with elapsed duration are removed and surfaced to logs/render
- [ ] `CleanupEncounterZones` is invoked on encounter end
- [ ] Test in `internal/combat/zone_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-silence-zone-type, E-67-zone-render-on-map, E-67-zone-anchor-follow, E-67-zone-triggers
- BreakConcentrationFully zone-removal path (already wired) — ensure no double-cleanup regressions

## Notes
Three related cleanup wirings rolled into one task per the splitting guidance.
