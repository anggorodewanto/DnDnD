---
id: E-67-silence-zone-type
group: E
phase: 67
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Silence spell uses wrong ZoneType (silence-enforcement code can't see it)

## Finding
`zone_definitions.go:102` sets `ZoneType:"control"` for the Silence spell, but `combatantInSilenceZone` and `CheckSilenceBreaksConcentration` filter on `ZoneType == "silence"`. The Silence spell creates a zone the silence-enforcement code cannot see, so casting Silence will not silence anyone or break V/S concentration. Cross-impacts Phase 61.

## Code paths cited
- internal/combat/zone_definitions.go:102 — Silence definition with `ZoneType:"control"`
- internal/combat/concentration.go — `ValidateSilenceZone`, `combatantInSilenceZone`, `CheckSilenceBreaksConcentration` (filter on `"silence"`)

## Spec / phase-doc anchors
- docs/phases.md — Phase 67 ("Spell Effect Zones") + Phase 61 ("Concentration … silence-zone V/S blocks")

## Acceptance criteria (test-checkable)
- [ ] Silence spell creates a zone whose ZoneType is recognized by silence-enforcement code
- [ ] Casting V/S spells inside the Silence zone is blocked
- [ ] Damage taken by a concentrating caster inside Silence zone correctly triggers V/S concentration-break path
- [ ] Test in `internal/combat/concentration_test.go` or `zone_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-zone-render-on-map, E-67-zone-anchor-follow, E-67-zone-triggers, E-67-zone-cleanup (same zone subsystem)

## Notes
Either change Silence's ZoneType to "silence" or expand the filters to accept the control+silence pairing — service-side decision; both have ripple effects on definitions and filters.
