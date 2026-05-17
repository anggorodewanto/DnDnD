finding_id: F-H01
severity: High
title: No light-source dim radius — 5e torches grant 20ft bright + 20ft dim
location: /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:907-927
spec_ref: spec §Vision sources & modifiers lines 2206-2207; phases §Phase 68
problem: |
  lightRadiusForItem/lightRadiusForSpell return a single RangeTiles and the FoW union
  promotes everything in that radius to Visible. No tiles get demoted to "dim light"
  obscurement. Players carrying a torch in pitch-black always see crisp 20ft bright
  and no dim halo.
suggested_fix: |
  Add DimRangeTiles to LightSource. In the visibility computation, mark tiles between
  bright range and dim range as Explored (dim/greyed) rather than Visible.
acceptance_criterion: |
  A LightSource with RangeTiles=4 and DimRangeTiles=8 marks tiles at distance 5-8
  as Explored (not Visible). A test demonstrates this.
