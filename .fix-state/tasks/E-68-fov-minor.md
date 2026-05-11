---
id: E-68-fov-minor
group: E
phase: 68
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# FoW: algorithm name, Devil's Sight, magical-darkness override, DM-sees-all

## Finding
Several Phase 68 sub-items are partial:
- Algorithm is center-to-center raycasting (symmetric by construction) rather than Albert Ford's named symmetric shadowcasting.
- `VisionSource` lacks `HasDevilsSight bool`; FoW vision/light union does not consider Devil's Sight despite obscurement.go being aware of it.
- `ComputeVisibilityWithLights` takes max(base, darkvision, blindsight, truesight) without a zone-level pass demoting darkvision through magical darkness. Override is only enforced at attack/check time via `EffectiveObscurement`. Spec explicitly calls this out as part of FoW.
- DM-sees-all toggle: not visible in FoW path; renderer always uses the union for whatever vision sources are passed in. Caller must omit FoW for DM rendering.

## Code paths cited
- internal/gamemap/renderer/fow.go — `shadowcast`, `rayBlockedByWalls`
- internal/gamemap/renderer/fog_types.go — `VisibilityState`, `FogOfWar`, `ComputeVisibility`, `ComputeVisibilityWithLights`
- internal/gamemap/renderer/fog.go — `DrawFogOfWar`, `filterCombatantsForFog`
- cmd/dndnd/discord_adapters.go:411-477 — explored history persistence
- internal/combat/obscurement.go — Devil's Sight + magical-darkness awareness

## Spec / phase-doc anchors
- docs/phases.md — Phase 68 ("Dynamic Fog of War"); "symmetric shadowcasting (Albert Ford's algorithm)" and "Magical darkness ignores darkvision"

## Acceptance criteria (test-checkable)
- [ ] FoW algorithm implements (or is renamed to faithfully describe) Albert Ford's symmetric shadowcasting
- [ ] `VisionSource` carries a `HasDevilsSight` flag and the FoW union honors Devil's Sight against magical darkness
- [ ] Magical-darkness zones demote darkvision inside `ComputeVisibilityWithLights` (not just at attack/check time)
- [ ] DM-rendering path explicitly bypasses FoW (or an explicit DM-sees-all toggle is honored)
- [ ] Test in `internal/gamemap/renderer/fog_test.go` (or fow_test.go) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-69-obscurement-misc (shared obscurement.go usage)
- E-67-zone-render-on-map (zone-to-render plumbing)

## Notes
Four small sub-items grouped per task-id guidance; consider splitting only if implementation pressure dictates.
