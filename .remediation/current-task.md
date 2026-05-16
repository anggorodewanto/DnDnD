finding_id: B-H05
severity: High
title: TilesetRefs request field silently dropped by HTTP handler
location: internal/gamemap/handler.go:65-82, 198-218, 302-322
spec_ref: spec §Asset Storage / Tiled-import; Phase 19
problem: |
  createMapRequest and updateMapRequest define no tileset_refs field. The DB column exists but no API caller can populate it.
suggested_fix: |
  Add TilesetRefs []TilesetRef to both request structs, wire through to CreateMapInput/UpdateMapInput.
acceptance_criterion: |
  A create-map request with tileset_refs populates the field on the stored map. A test demonstrates this.
