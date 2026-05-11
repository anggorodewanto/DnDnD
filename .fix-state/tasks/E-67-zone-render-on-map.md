---
id: E-67-zone-render-on-map
group: E
phase: 67
severity: HIGH
status: in_review
owner: opus 4.7 (1M) — ZONES-impl
reviewer:
last_update: 2026-05-11
---

# Spell zones never appear on rendered maps (ZoneOverlays unpopulated)

## Finding
Renderer's `DrawZoneOverlays` requires `MapData.ZoneOverlays` to be populated; no production code populates it. `cmd/dndnd/discord_adapters.go:428` calls `ParseTiledJSON(..., nil)` with nil effects. Spell zones do not appear on rendered maps.

## Code paths cited
- internal/gamemap/renderer/zone.go — `DrawZoneOverlays` (requires `MapData.ZoneOverlays`)
- cmd/dndnd/discord_adapters.go:428 — `ParseTiledJSON(..., nil)` passing nil effects
- internal/combat/zone.go — `Service` zone CRUD (source of zone data)

## Spec / phase-doc anchors
- docs/phases.md — Phase 67 ("Spell Effect Zones"); zones must render on maps

## Acceptance criteria (test-checkable)
- [ ] Active encounter zones are loaded and passed into `MapData.ZoneOverlays` for rendering
- [ ] Rendered map images show overlays for at least Fog Cloud, Wall of Fire, Darkness, and Spirit Guardians
- [ ] Test in `internal/gamemap/renderer/zone_test.go` (or discord_adapters test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-67-silence-zone-type, E-67-zone-anchor-follow, E-67-zone-triggers, E-67-zone-cleanup

## Notes
Includes plumbing zone records from `combat.Service` through the discord adapter into `ParseTiledJSON` (or the map-regenerator path).
