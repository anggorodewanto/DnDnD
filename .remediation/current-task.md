finding_id: B-H06
severity: High
title: DM-view fog-of-war ignores MapData.DMSeesAll when caller pre-computed fog
location: internal/gamemap/renderer/renderer.go:33-47, fog.go:14-20, fog_types.go:32-40
spec_ref: spec §Dynamic Fog of War; Phase 22
problem: |
  RenderMap only propagates md.DMSeesAll into md.FogOfWar.DMSeesAll when fog is non-nil at call time. filterCombatantsForFog checks only fow.DMSeesAll, never md.DMSeesAll. Result: enemies on Unexplored tiles disappear from the DM's map even with MapData.DMSeesAll = true.
suggested_fix: |
  Read md.DMSeesAll || (md.FogOfWar != nil && md.FogOfWar.DMSeesAll) in both DrawFogOfWar and filterCombatantsForFog.
acceptance_criterion: |
  When md.DMSeesAll=true but FogOfWar.DMSeesAll=false, the DM still sees all combatants. A test demonstrates this.
