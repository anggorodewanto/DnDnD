# Batch 03: Maps, assets, editor, rendering, encounter builder (Phases 19–23)

## Summary

Phases 19–23 are largely **delivered with one significant gap**: the server-side
PNG renderer (Phase 22) only parses `terrain` and `walls` from the Tiled JSON.
Lighting zones, elevation, and spawn-zone overlays painted in the Map Editor
(Phase 21c) are persisted in `tiled_json` but are **not honored by the renderer
or fog-of-war pipeline**. Spawn zones are at least consumed by
`internal/exploration/spawn.go`; lighting and elevation appear to be
write-only data today.

Map size limits (soft 100, hard 200) are enforced server-side and surfaced in
the editor (with a one-time downscale warning). The AssetStore interface is
clean, returns a stable `/api/assets/{id}` URL, and the Fly Volume mount is
configured. Tiled `.tmj` import scaffolding is in place with hard-rejection
sentinels and a skipped-features tracker, matching the spec's three-tier
validation. Encounter Builder is real and includes drag-drop placement, short
ID generation, duplicate, and a creature library selector.

No golden-PNG fixtures exist for the renderer — tests are structural (decode
succeeds, dimensions match formula).

## Per-phase findings

### Phase 19 — Maps Table & Map Storage
- Status: **MATCHES**.
- Key files:
  - `db/migrations/20260310120009_create_maps.sql`
  - `internal/gamemap/service.go` (constants `SoftLimitDimension=100`,
    `HardLimitDimension=200`, `StandardTileSize=48`, `LargeTileSize=32`)
  - `internal/gamemap/handler.go`
- Findings:
  - Migration matches spec: `id, campaign_id, name, width_squares,
    height_squares, tiled_json, background_image_id, tileset_refs` plus
    timestamps and CHECK >= 1.
  - `validateDimensions` rejects width/height > 200; `classifySize` flags
    `large` over 100. Both invariants are unit-tested.
  - `tileset_refs` stored as JSONB; service marshals `[]TilesetRef` with
    `pqtype.NullRawMessage` (correct, but not exposed on `mapResponse` JSON —
    minor divergence; not blocking since Phase 2 tilesets are deferred).
  - `TiledJSON` round-trips intact; sqlc-generated CRUD via `refdata.*MapParams`.

### Phase 20 — Assets Table & AssetStore Interface
- Status: **MATCHES**.
- Key files:
  - `db/migrations/20260310120010_create_assets.sql` (adds FK from
    `maps.background_image_id` to `assets.id`)
  - `internal/asset/store.go` — `Store` interface: `Put / Get / Delete / URL`
  - `internal/asset/local_store.go` — UUID filenames, layout
    `{baseDir}/{campaign_id}/{typeDir}/{uuid}` matching spec
    `data/assets/{campaign_id}/{type}/`
  - `internal/asset/service.go`, `internal/asset/handler.go` — wraps DB and
    file store, exposes `POST /api/assets/upload` and `GET /api/assets/{id}`
  - `fly.toml` — `[mounts] destination = "/data"`
- Findings:
  - Interface matches spec exactly. URL returns stable
    `"/api/assets/" + assetID` — independent of storage backend.
  - `Upload` does best-effort cleanup of the stored file if the DB insert
    fails (good).
  - AssetType enum covers `map_background, token, tileset, narration` —
    note: spec also names `generated`, which is intentionally absent because
    rendered map PNGs are ephemeral (Discord-attached, not persisted) — spec
    line 2364 confirms this is correct.
  - `original_name` is recorded but the local store ignores it on disk; UUID
    filenames avoid collisions per spec.

### Phase 21a — Map Editor: Grid, Terrain, Walls, Save/Load
- Status: **MATCHES**.
- Key files:
  - `dashboard/svelte/src/MapEditor.svelte` (~1247 lines)
  - `dashboard/svelte/src/MapList.svelte`
  - `dashboard/svelte/src/lib/mapdata.js` (canonical Tiled-format model)
  - `dashboard/svelte/src/lib/api.js`
- Findings:
  - Editor specifies dimensions, generates a blank Tiled-JSON map with the
    five canonical layers (`terrain`, `walls`, `lighting`, `elevation`,
    `spawn_zones`).
  - Terrain brush covers all 5 spec types (open ground, difficult, water,
    lava, pit) with matching GID assignments.
  - Wall tool draws along tile edges as `objectgroup` rectangles (width=tile
    when horizontal, height=tile when vertical) — matches Tiled convention.
  - Save/load via `POST/PUT /api/maps` + `GET /api/maps/{id}`.
  - `/api/me` wires campaign ID (per the phases.md note); no placeholder.

### Phase 21b — Image Import & Opacity
- Status: **MATCHES**.
- Key files: `dashboard/svelte/src/MapEditor.svelte` lines 40-43, 115-118,
  154-155, 178-182, 200-206, 240-243, 259, 357-363.
- Findings:
  - Background image uploaded via `/api/assets/upload` with
    `type=map_background`; returned URL stored as `backgroundImageId` on the
    map and round-tripped through `GET /api/maps/{id}`.
  - Opacity slider (`backgroundOpacity`, default 0.5) feeds
    `ctx.globalAlpha` during canvas draw, with the grid rendered at full
    opacity on top.

### Phase 21c — Lighting, Elevation, Spawn Zones
- Status: **PARTIAL / DIVERGENT (write-only on server)**.
- Key files:
  - `dashboard/svelte/src/lib/mapdata.js` — `LIGHTING_TYPES`, `setLighting`,
    `setElevation` (clamped to `ELEVATION_MAX=10`), `addSpawnZone`,
    `getSpawnZones`.
  - `dashboard/svelte/src/MapEditor.svelte` — brush UI for all three layers.
  - `internal/exploration/spawn.go` — reads `spawn_zones` from Tiled JSON
    for encounter start placement.
- Findings:
  - Editor produces and persists all three layers per spec; data round-trips
    through `tiled_json`.
  - **Spawn zones** are read by the exploration spawn placement code — OK.
  - **Lighting brush data is not consumed anywhere on the server.** The
    `renderer/` package parses only `terrain` and `walls` layers
    (`renderer/parse.go` `parseTerrainLayer`/`parseWallsLayer`). The spec's
    "Static environmental lighting baked into the map data" (spec line 2216)
    is not wired into the fog-of-war or zone-overlay rendering. Magical
    darkness is honored only when supplied as a runtime `encounter_zones`
    entry, not from baked-in map lighting.
  - **Elevation per tile** is similarly stored but never read by the server
    — `Combatant.AltitudeFt` in the renderer comes from combatant state, not
    from the tile under the token.
  - This is the largest divergence in the batch. It does not block Phase 23
    (encounter builder), but it does mean the spec's lighting-from-map-data
    promise is unsatisfied at the renderer/FoW level.

### Phase 21d — Undo/Redo, Region Select, Copy/Paste, Duplicate
- Status: **MATCHES** (real, not stubs).
- Key files:
  - `dashboard/svelte/src/lib/mapdata.js` — `UndoStack` (maxSize 50,
    push/undo/redo/clear), `extractRegion`, `pasteRegion`, `cloneMap`,
    `duplicateMap`.
  - `dashboard/svelte/src/MapEditor.svelte` lines 78-93, 265-340 — keyboard
    handlers Ctrl+Z, Ctrl+Shift+Z, Ctrl+C, Ctrl+V; rectangular region drag,
    paste preview, duplicate-map button.
  - `dashboard/svelte/src/lib/mapdata.test.js`
- Findings:
  - UndoStack stores full map snapshots — simple but correct; redo stack
    is cleared on new push (standard behavior).
  - `extractRegion`/`pasteRegion` cover terrain + lighting + elevation tile
    layers AND walls + spawn_zones object layers with coordinate clipping
    and object-id renumbering on paste.
  - `duplicateMap` is a deep `JSON.parse(JSON.stringify(...))` clone; the
    editor appends " (copy)" to the name and clears the undo stack on
    duplicate — matches spec.

### Phase 22 — Map Rendering Engine (Server-Side PNG)
- Status: **PARTIAL** (core rendering present; lighting/elevation gap;
  no golden fixtures).
- Key files:
  - `internal/gamemap/renderer/renderer.go` — `RenderMap` orchestrator.
  - `internal/gamemap/renderer/parse.go` — Tiled JSON -> `MapData`.
  - `internal/gamemap/renderer/terrain.go`, `grid.go`, `wall.go`,
    `token.go`, `legend.go`, `zone.go`, `fog.go`, `fow.go`, `queue.go`.
  - `internal/gamemap/renderer/types.go` — `HealthTier` ladder (Uninjured /
    Scratched / Bloodied / Critical / Dying / Dead / Stable) with
    `TierColor` (dual-channel: color + symbol).
- Findings:
  - Tile size auto-downscales to 32px when width>100 or height>100 inside
    `RenderMap` itself (spec lines 2185, 2322).
  - Coordinate labels follow A-Z, AA-AZ, ... pattern in `grid.go`
    (`TestParseCoordinate_RoundTrip` covers this).
  - Stacked-token altitude offset is implemented and tested
    (`TestRenderMap_StackedTokens`).
  - Unified legend (terrain key + active effects) toggles on/off via
    `LegendHeight` — empty map with no effects skips legend (matches spec
    line 2190).
  - Fog of war: symmetric shadowcasting in `fow.go`; magical-darkness
    demotion is computed from runtime zones only, not from baked-in
    lighting tiles (see Phase 21c).
  - **Render queue** (`queue.go`) implements per-encounter debouncing via a
    `time.AfterFunc` timer; new `Enqueue` calls reset the timer and replace
    `latest[encounterID]`, so intermediate states are discarded (matches
    spec line 2180). Tests `TestRenderQueue_DebounceCoalescesMultipleRequests`
    and `TestRenderQueue_DifferentEncountersRenderedSeparately` cover this.
  - **No golden PNG fixtures.** All renderer tests verify shape only
    (decode succeeds, expected pixel dimensions match formula). Visual
    regressions could slip through. Spec doesn't mandate goldens but if
    we add them later they belong in `internal/gamemap/renderer/testdata/`.
  - **Health tier indicator dual-channel:** `types.go` defines the colors;
    `token.go` adds symbol overlays — needs sub-audit if colorblind
    accessibility is being checked explicitly.

### Phase 23 — Encounter Templates & Encounter Builder
- Status: **MATCHES**.
- Key files:
  - `db/migrations/20260312120001_create_encounter_templates.sql`
  - `internal/encounter/service.go`, `handler.go`
  - `dashboard/svelte/src/EncounterBuilder.svelte` (876 lines)
  - `dashboard/svelte/src/EncounterList.svelte`
- Findings:
  - Migration has `id, campaign_id, map_id (FK), name, display_name,
    creatures jsonb` — matches spec (display name + internal name distinct).
  - Service CRUD + `Duplicate` (appends " (copy)" to name and display
    name); `Delete`, `Update`, `ListByCampaignID`. `ListCreatures` exposes
    the stat block library for the builder.
  - Builder UI: name + display name fields, map selector, creature search
    panel feeding off `GET /api/creatures`, quantity per creature,
    `generateShortId` that scans existing short_ids and increments
    (`G1, G2, ...`) — matches spec's auto-generation.
  - Drag-drop creature placement on canvas: `startDragCreature`,
    `handleCanvasDragOver`, `dragPreviewPos`, with token preview rendered
    over the map. Placement writes `position_col`/`position_row` into the
    creature entry.
  - "Saved Encounters" list is the `EncounterList.svelte` component
    (Campaign Home requirement satisfied).
  - Encounter mode column added in a later migration
    (`20260415120000_add_encounter_mode.sql`); not in scope for Phase 23
    but worth noting templates have evolved.

## Cross-cutting concerns

- **Editor → renderer data parity gap:** the Svelte editor produces 5 canonical
  layers (terrain, walls, lighting, elevation, spawn_zones); the server
  renderer reads 2 (terrain, walls). Spawn zones are read elsewhere
  (exploration); lighting and elevation are write-only. This is the only
  cross-cutting divergence in the batch.
- **No golden PNGs:** `internal/gamemap/renderer/` has 23+ unit tests but no
  visual fixtures. Tests verify decode and dimensions; pixel-level
  regressions in terrain colors, legend layout, or token glyphs are not
  caught.
- **Asset URL stability:** `LocalStore.URL` returns `/api/assets/{id}`, which
  is decoupled from on-disk path — swapping to S3 only requires changing
  the `Store` impl. Good abstraction hygiene.
- **`generateDefaultTiledJSON` (server)** in `internal/gamemap/handler.go`
  produces only `terrain` + `walls` layers — diverges from the Svelte
  `generateBlankMap` which produces all 5. Maps created via raw API without
  pre-built Tiled JSON will be missing the lighting/elevation/spawn_zones
  layers, though this is moot until the renderer reads them.
- **Tiled import** (`internal/gamemap/import.go`): hard rejections for
  infinite maps, non-orthogonal orientation, oversize dimensions; soft
  rejections track tile animations, image layers, parallax, group layers
  (flattened), text/point objects, wang sets. Matches spec lines 2334-2354.
  Phase 116 will close the loop on full tileset support — this scaffolding
  is sufficient.

## Critical items

1. **Phase 21c lighting + elevation are write-only on the server.**
   `internal/gamemap/renderer/parse.go` does not parse the `lighting` or
   `elevation` Tiled layers; the spec's "baked-in static lighting" promise
   (spec line 2216) is unfulfilled at the renderer/FoW level. Recommended:
   extend `ParseTiledJSON` to populate a tile-indexed lighting grid and
   pipe it into `ComputeVisibilityWithZones` / a new draw helper that
   tints unlit tiles. Elevation should feed token altitude defaults or be
   explicitly deferred to a later phase.

2. **`generateDefaultTiledJSON` (server) vs `generateBlankMap` (client)
   drift.** Server-side default produces only 2 layers; client produces 5.
   Once Critical Item 1 is fixed, server-side default must match — or the
   server should reject API-created maps that lack the canonical layer
   set.

3. **No golden PNG fixtures for the renderer.** Visual regressions
   (terrain colors, legend layout, token glyph alignment) are not caught
   by current tests. Low priority but worth adding when lighting/elevation
   rendering lands so the new layers are pinned.
