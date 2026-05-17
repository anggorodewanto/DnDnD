finding_id: B-H04
severity: High
title: Map renderer never composites the uploaded background image
location: /home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go
spec_ref: phases §Phase 21b ("Background renders beneath terrain layer", with opacity)
problem: |
  The server-side PNG renderer has no awareness of maps.background_image_id at all.
  ParseTiledJSON parses only terrain, walls, lighting, elevation, and spawn zones.
  Phase 21b only delivers the background-image rendering inside the Svelte editor preview.
  Any map that uses a battle-map image as backdrop will render as plain beige terrain in Discord.
suggested_fix: |
  Draw the bg image (looked up via AssetStore) in RenderMap before terrain, with an opacity
  slider value plumbed from the Tiled JSON or a dedicated MapData.BackgroundOpacity field.
acceptance_criterion: |
  When MapData includes a BackgroundImage ([]byte PNG) and BackgroundOpacity (float64),
  RenderMap composites the image beneath the terrain layer at the specified opacity.
  A test demonstrates a non-white pixel at a position where the background image has color.
