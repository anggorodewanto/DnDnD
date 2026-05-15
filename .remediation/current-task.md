finding_id: F-C03
severity: Critical
title: Devil's Sight never wired into player vision pipeline
location: cmd/dndnd/discord_adapters.go:755-806
spec_ref: spec §Dynamic Fog of War lines 2204-2208; phases §Phase 68
problem: |
  renderer.VisionSource exposes HasDevilsSight, the FoW math honors it, and the combat obscurement engine honors it — but buildVisionSources never sets the flag from PC race/class/feature data. A Warlock in Darkness sees only the origin tile.
suggested_fix: |
  Inspect the character's features JSON for "Devil's Sight" and set src.HasDevilsSight = true in buildVisionSources.
acceptance_criterion: |
  A character with "Devil's Sight" in their features has HasDevilsSight=true in their VisionSource. A test demonstrates this.
