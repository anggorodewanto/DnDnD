# Group B: Dice / Maps / Encounters (Phases 18–25) — Correctness Review

Scope: `internal/dice`, `internal/gamemap`, `internal/asset`, `internal/encounter`,
`internal/combat` (initiative bits), dashboard Svelte map-editor / encounter-builder,
and the `maps` / `assets` / `encounter_templates` / `encounters` / `combatants` /
`turns` migrations.

---

## [Critical] `ParseExpression` mangles modifiers with multiple `+`/`-` operators

- **Location:** `/home/ab/projects/DnDnD/internal/dice/dice.go:46-58`
- **Spec/Phase ref:** phases §Phase 18 ("parse dice expressions (NdM+K)", "modifier
  stacking")
- **Problem:** The parser strips every dice group then strips every `+`, leaving
  the residue to `Atoi`. As a result:
  - `"1d20+5+5"` parses as modifier `+55` (not `+10`).
  - `"1d20-2+3"` parses as modifier `-23` (instead of `+1`).
  - `"1d20-2-3"` returns an error ("invalid syntax") even though it is a valid
    expression meaning `−5`.
  - `"1d4+1d6+2+3"` would similarly collapse to `+23`.
  This breaks "modifier stacking" called out as a Phase 18 success criterion and
  silently corrupts spell damage like `"2d8+1d6+3"` rolled with feature bonuses
  appended to the expression string.
- **Suggested fix:** Walk the post-dice residue token-by-token, summing each
  signed integer, or replace the regex hack with a proper expression tokenizer
  that accepts `\s*[+-]\s*\d+` repeatedly.

## [Critical] `cryptoRand` / `RollD20` panic on degenerate dice (`Nd0`)

- **Location:** `/home/ab/projects/DnDnD/internal/dice/roller.go:48-54`,
  `/home/ab/projects/DnDnD/internal/dice/dice.go:23` (regex accepts `1d0`).
- **Spec/Phase ref:** phases §Phase 18.
- **Problem:** `ParseExpression` accepts `1d0`, `5d0`, etc. (`\d+d\d+` does not
  exclude zero). `rollGroups` then calls `r.randFn(0)`, which is `cryptoRand(0)`,
  which calls `rand.Int(rand.Reader, big.NewInt(0))` — that panics with
  "crypto/rand.Int argument must be > 0". Any user-supplied `/roll 1d0` (or any
  dashboard handler that forwards a malformed string) crashes the request /
  goroutine. Roll-history logging never gets a chance to bound the input.
- **Suggested fix:** Validate `Count >= 1 && Sides >= 1` in `ParseExpression`
  (and clamp/error in `rollGroups`) before any dice are drawn.

## [High] Map size limits not enforced when rendering, only at create-time

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:12-16`;
  no hard-limit check.
- **Spec/Phase ref:** spec §Map Size Limits ("rejected: >200 in either dimension").
- **Problem:** `RenderMap` only checks `>100` to downscale tile size, but accepts
  arbitrarily large `Width`/`Height` and tries to allocate a `(Width*TileSize) ×
  (Height*TileSize)` canvas. A bug or malicious template that gets past the
  service-layer validator (e.g. a stale stored map, or any caller building
  `MapData` directly from Tiled JSON without going through `service.CreateMap`)
  will allocate gigabytes and may OOM the bot. The DB CHECK in
  `20260310120009_create_maps.sql:6-7` only enforces `>= 1`, not the
  spec's 200-cell hard ceiling — so a map exceeding 200x200 inserted by hand or
  via `UpdateMap` from a buggy client will pass through fine.
- **Suggested fix:** Add `if md.Width > HardLimitDimension || md.Height > HardLimitDimension`
  early-return in `RenderMap`, mirror the limit as a DB CHECK constraint, and
  cap `tilesetMap`/canvas allocation by an absolute pixel ceiling.

## [High] `RenderMap` mutates caller-supplied `MapData.TileSize`

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:13-16`
  (`md.TileSize = 32`).
- **Spec/Phase ref:** spec §Map Size Limits (auto-downscale rule); phases §Phase 22.
- **Problem:** `RenderMap` overwrites `md.TileSize` in place. The same `*MapData`
  pointer is stored in `RenderQueue.latest` (queue.go:38-55) and is therefore
  visible to any future enqueue. A subsequent caller that *also* mutates the
  same struct (e.g. updates combatant positions and re-enqueues) will continue
  to see TileSize=32, and any "view" of the map cached elsewhere (encounter
  builder preview, fog precomputation) is silently corrupted. It also throws
  away an explicit caller-chosen TileSize for small maps with custom rendering
  needs.
- **Suggested fix:** Treat `MapData` as read-only inside the renderer: compute
  a local `tileSize := md.TileSize; if md.Width > 100 || md.Height > 100 { tileSize = 32 }`
  and pass that everywhere instead of mutating.

## [High] Asset upload accepts arbitrary MIME types (XSS / file-type abuse risk)

- **Location:** `/home/ab/projects/DnDnD/internal/asset/handler.go:36-83`,
  `/home/ab/projects/DnDnD/internal/asset/service.go:121-135`.
- **Spec/Phase ref:** phases §Phase 20, spec §Asset Storage ("`mime_type`...
  serves dashboard via `/api/assets/{id}`").
- **Problem:** `UploadAsset` trusts the multipart `Content-Type` header
  verbatim. `validateUpload` only checks that mime_type is non-empty; it never
  asserts the file is a PNG/JPG/TMJ. A DM (or an attacker who has DM auth) can
  upload an HTML/JS/SVG file as a "map_background" — `ServeAsset` then sets
  `Content-Type: text/html` and serves it on the same origin as the dashboard,
  enabling stored XSS. Also no per-type extension/size split (10 MB ceiling
  applies to all kinds including tileset JSON).
- **Suggested fix:** Maintain an allowlist per `AssetType`
  (`map_background`/`token` → `image/png|image/jpeg|image/webp`, `tileset` →
  `application/json`, etc.), reject everything else at the handler, and
  sniff the first 512 bytes with `net/http.DetectContentType` instead of
  trusting the client.

## [High] Map renderer never composites the uploaded background image

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go`
  (no background draw); `/home/ab/projects/DnDnD/internal/gamemap/renderer/types.go`
  (no `BackgroundImageID`/`BackgroundOpacity` fields on `MapData`).
- **Spec/Phase ref:** phases §Phase 21b ("Background renders beneath terrain
  layer", with opacity).
- **Problem:** The server-side PNG renderer has no awareness of
  `maps.background_image_id` at all — `ParseTiledJSON` parses only terrain,
  walls, lighting, elevation, and spawn zones. Phase 21b only delivers the
  background-image rendering inside the Svelte editor preview. Any map that
  uses a battle-map image as backdrop will render as plain beige terrain in
  Discord. Worse, since asset uploads succeed and the editor lets the DM set
  opacity, the system gives the false impression that backgrounds work.
- **Suggested fix:** Either (a) draw the bg image (looked up via `AssetStore`)
  in `RenderMap` before terrain, with an opacity slider value plumbed from the
  Tiled JSON or a dedicated `MapData.BackgroundOpacity` field, or (b)
  explicitly mark this as future work in the phases doc.

## [High] `TilesetRefs` request field silently dropped by HTTP handler

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/handler.go:65-82,
  198-218, 302-322`.
- **Spec/Phase ref:** spec §Asset Storage / Tiled-import compatibility (Phase
  19 schema includes `tileset_refs`).
- **Problem:** `createMapRequest` and `updateMapRequest` define no
  `tileset_refs` field, and the handler never passes one to
  `CreateMapInput`/`UpdateMapInput`. The DB column exists, the service layer
  marshals it, but no API caller can ever populate it. Tileset support
  (Phase 2 of the map system per spec) cannot be exercised through the REST
  surface — only via direct SQL or the `/api/maps/import` path.
- **Suggested fix:** Add `TilesetRefs []TilesetRef` to both request structs,
  wire through to `CreateMapInput.TilesetRefs` / `UpdateMapInput.TilesetRefs`,
  and add a service-layer round-trip test.

## [High] DM-view fog-of-war ignores `MapData.DMSeesAll` when caller pre-computed fog *without* setting the flag on `FogOfWar`

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:33-47`,
  `fog.go:14-20`, `fog_types.go:32-40`.
- **Spec/Phase ref:** spec §Dynamic Fog of War ("DM vs. player visibility:
  zone boundaries are rendered as colored overlays in the DM's Combat Manager
  view").
- **Problem:** `RenderMap` only propagates `md.DMSeesAll` into
  `md.FogOfWar.DMSeesAll` when fog is non-nil at call time. But the caller
  that *creates* `MapData` from `ParseTiledJSON` and *then* pre-computes
  `FogOfWar` themselves before calling `RenderMap` (a documented path — see
  the renderer_test.go pre-computed-fog tests) will end up with fog rendered
  for the DM too unless they remembered to set `FogOfWar.DMSeesAll`
  themselves. The contract is fragile and the parallel
  `filterCombatantsForFog` (`fog.go:52-78`) checks only `fow.DMSeesAll`,
  never `md.DMSeesAll`. Result: enemies on Unexplored tiles disappear from the
  DM's map even with `MapData.DMSeesAll = true`.
- **Suggested fix:** Read `md.DMSeesAll || (md.FogOfWar != nil && md.FogOfWar.DMSeesAll)`
  in both `DrawFogOfWar` and `filterCombatantsForFog`, or have `RenderMap`
  unconditionally copy the flag into a fresh `FogOfWar` (lazily allocating
  one when nil).

## [High] Fog renderer does not preserve "previously seen" cells across renders

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:68-93`
  (no merge with prior `explored_cells`).
- **Spec/Phase ref:** spec §Dynamic Fog of War step 4: "Previously seen but
  currently out-of-range cells rendered as dim/greyed out (explored but not
  active)".
- **Problem:** `ComputeVisibilityWithZones` builds a fresh `FogOfWar` per call
  with all states zero-initialised (Unexplored), then sets currently-visible
  tiles to `Visible`. It never marks the just-left tiles as `Explored`. The
  migration `20260513120001_add_encounter_explored_cells.sql` adds a column,
  and `refdata` reads/writes it, but no renderer or combat code merges it into
  the per-render fog. Consequence: when a player walks back out of a room,
  the room snaps to fully black on the next render instead of dim-grey — the
  spec's "explored but not active" tier is functionally unreachable.
- **Suggested fix:** Have the encounter render path read `encounters.explored_cells`,
  bit-OR the just-computed `Visible` set into it, persist the union back, and
  pass the union into the fog as `Explored` for any cell that is Explored but
  not currently `Visible`.

## [Medium] Initiative DEX-tie alphabetical sort is byte-wise, not D&D-aware

- **Location:** `/home/ab/projects/DnDnD/internal/combat/initiative.go:166-177`
  (`SortByInitiative`).
- **Spec/Phase ref:** spec §Initiative Tiebreaking ("alphabetical by display
  name"); phases §Phase 25.
- **Problem:** The sort uses raw Go `<` on `DisplayName`, which is byte-wise
  Unicode comparison and case-sensitive. So `"aria"` sorts AFTER `"Zara"`
  (lowercase code points are higher), and accented characters (`Élise`) sort
  *after* every ASCII name. For an inviting global player base this produces
  inconsistent (and arguably broken) tiebreaks. The test
  `TestSortByInitiative_TieAlphabetical` only exercises ASCII same-case
  inputs.
- **Suggested fix:** Use `strings.ToLower` (or `golang.org/x/text/collate`)
  before comparing; add a test for `"aria"` vs `"Zara"`.

## [Medium] PNG renderer renders zero-cost / oversized canvas for invalid `MapData`

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:18-30`.
- **Spec/Phase ref:** spec §Map Rendering ("regenerated from scratch each
  time"); phases §Phase 22.
- **Problem:** `RenderMap` doesn't guard against `Width <= 0`, `Height <= 0`,
  or `TileSize <= 0`. With `Width=0, Height=0, TileSize=0` the canvas is
  `(0+margin) × (0+margin+legend)` and the PNG encodes successfully as a 28×28
  blank image. With `TileSize=0` and `Width=50` the canvas is 28×28 again —
  silently wrong rather than failing fast. Worse, with `Width=200, Height=200,
  TileSize=200` (a buggy caller that didn't honor the soft-limit downscale)
  the renderer allocates a 40000×40000 RGBA buffer (≈6.4 GB).
- **Suggested fix:** Reject (`return nil, error`) when any of Width/Height/
  TileSize is non-positive or when `Width*TileSize*Height*TileSize` exceeds a
  hard ceiling (e.g. ~50M pixels).

## [Medium] Encounter map_id is nullable in the schema but Phase 22/23/26 assume it

- **Location:** `/home/ab/projects/DnDnD/db/migrations/20260312120001_create_encounter_templates.sql:5`,
  `20260312120002_create_encounters.sql:6`.
- **Spec/Phase ref:** spec §Encounter & Combat Tables (`map_id UUID FK → maps`,
  written without a NULL allowance for templates and "nullable for ad-hoc"
  only for encounters).
- **Problem:** Both `encounter_templates.map_id` and `encounters.map_id` are
  nullable. The template service `Create`/`Update` happily accept a
  null `MapID`. But the server-side PNG renderer (Phase 22), encounter builder
  preview (Phase 23) and combat lifecycle (Phase 26a) all assume a map
  exists, and `CreateEncounterFromTemplate` copies whatever the template has
  (`service.go:877`) — so a template saved without a map fans out into an
  encounter with no map, which then breaks `/start-combat`'s "post map image
  to Discord" step. There is no validation in `encounter.Service.Create`
  enforcing the template's map_id, despite the spec showing it as required for
  templates.
- **Suggested fix:** Either make `encounter_templates.map_id NOT NULL` (with a
  service-level validator), or document the spec change explicitly and add a
  preflight in `StartCombat`.

## [Medium] Encounter template `Duplicate` does not generate fresh `short_id`s

- **Location:** `/home/ab/projects/DnDnD/internal/encounter/service.go:115-139`.
- **Spec/Phase ref:** phases §Phase 23 ("save/edit/duplicate/delete
  templates"); spec §Combatant Targeting (short_id stable per encounter).
- **Problem:** `Duplicate` deep-copies `original.Creatures` JSON verbatim,
  including each entry's `short_id`. That's fine while the duplicate is a
  template, but as soon as both templates are used to instantiate two
  simultaneous encounters in the same campaign, two combatants might share
  the same short_id (e.g. `"G1"`) and the targeting layer's
  `ShortIDFromName` lookup becomes ambiguous in shared Discord channels.
- **Suggested fix:** Regenerate short_ids on duplicate (or ensure short_id
  uniqueness is scoped per encounter at the combatant layer — it is today,
  but only because instantiation appends `1`, `2`, … for Quantity>1, not
  across distinct creature_ref entries).

## [Medium] No validation that `position_col` / `position_row` are inside the map bounds

- **Location:** `/home/ab/projects/DnDnD/internal/combat/service.go:863-913`
  (`CreateEncounterFromTemplate`); no bounds check anywhere.
- **Spec/Phase ref:** phases §Phase 23 ("drag-drop creature token placement on
  map"), §Phase 24 ("combatants instantiated with correct stats").
- **Problem:** Template creatures store `position_col`/`position_row`, but the
  encounter service never compares them to `map.width_squares` /
  `height_squares`. A template authored against a 30×30 map and then
  re-pointed to a 10×10 map (or a buggy editor that emits col `"ZZ"`) silently
  produces combatants placed off-grid, breaking `/move`, fog-of-war, and the
  PNG renderer's token loop (which only checks `idx < 0`).
- **Suggested fix:** Validate placement against the encounter's map in
  `CreateEncounterFromTemplate` and surface a friendly error to the DM dialog.

## [Medium] DB does not enforce the map-dimension hard limit

- **Location:** `/home/ab/projects/DnDnD/db/migrations/20260310120009_create_maps.sql:6-7`.
- **Spec/Phase ref:** spec §Map Size Limits.
- **Problem:** The CHECK only requires `>= 1`. The hard limit (200) lives
  only in Go code (`gamemap.HardLimitDimension`). Any path that bypasses
  `Service.CreateMap` / `Service.UpdateMap` (data migration scripts, future
  endpoints, raw psql) can persist a 9999×9999 map and the renderer will then
  OOM. Defence-in-depth recommends the limit at both layers.
- **Suggested fix:** `CHECK (width_squares BETWEEN 1 AND 200 AND height_squares BETWEEN 1 AND 200)`.

## [Medium] Tiled import accepts a `width=0, height=0` map if it's not `infinite`

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/import.go:88-103`.
- **Spec/Phase ref:** spec §Tiled import compatibility ("hard rejection:
  infinite maps, non-orthogonal orientations, maps exceeding system maximum").
- **Problem:** `intField(doc, "width")` returns 0 if missing or zero. The
  check is `< 1` so this is rejected — good. But if the JSON has
  `"width": "30"` (string), `intField` returns 0 again and the import is
  rejected with the generic "got 0x0" message rather than a clear "non-numeric
  width" error. Lower-impact, but the importer also doesn't reject a Tiled
  map whose tilelayer `Data` length disagrees with `width*height`, which
  later trips the renderer's `idx >= len(grid)` guard silently (terrain
  defaults to OpenGround instead of erroring).
- **Suggested fix:** Validate `width`/`height` parsed as JSON numbers
  (`json.Number`), and assert layer-data length matches width×height up-front.

## [Medium] Stacked-token offset can place tokens outside their grid cell

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/token.go:28-36`.
- **Spec/Phase ref:** spec §Map Rendering ("stacked tokens: ... offset
  diagonally").
- **Problem:** `offset := float64(i) * ts * 0.2` shifts every subsequent
  stacked token by 0.2 × tile size diagonally. With four flying tokens
  stacked, the topmost token is centered at `(col*ts + ts/2 + 0.6*ts, row*ts +
  ts/2 - 0.6*ts)`, i.e. well *outside* the originating tile and overlapping
  neighbour tiles. There's no cap or wrap. With altitude badges this also
  starts to clip the canvas if the stack is at the top-right corner.
- **Suggested fix:** Clamp the diagonal offset (e.g. `min(i*0.2, 0.45)`) or
  switch to a small-arc layout (radial fan) for stacks of >3.

## [Medium] `/api/assets/upload` response sets headers after potentially writing body

- **Location:** `/home/ab/projects/DnDnD/internal/asset/handler.go:103-111`
  (`ServeAsset`).
- **Spec/Phase ref:** phases §Phase 20 (asset HTTP endpoint).
- **Problem:** `ServeAsset` writes `Content-Type` and `Content-Length` headers
  *and then* does `io.Copy(w, rc)`. If `OpenFile` succeeded but the file is
  truncated/short-read, the `Content-Length` header lies, and the proxy/CDN
  may serve garbage. There's also no fallback for a partial read mid-stream
  (the comment "best-effort copy" acknowledges this). For small images this
  is harmless, but for tilesets it could corrupt the dashboard.
- **Suggested fix:** Stat the file size before serving (or stream into a
  `bytes.Buffer` first if assets stay <10 MB), then set Content-Length to the
  actual byte count.

## [Medium] Local asset storage path-traversal: filename is discarded, but campaign UUID isn't validated

- **Location:** `/home/ab/projects/DnDnD/internal/asset/local_store.go:32-46`.
- **Spec/Phase ref:** spec §Asset Storage (`data/assets/{campaign_id}/{type}/`).
- **Problem:** `Put` joins `s.baseDir + campaignID.String() + dir + uuid.New().String()`.
  `uuid.UUID.String()` is safe, but the upload handler parses campaign_id via
  `uuid.Parse`, which accepts lowercase, uppercase, and braced forms.
  `uuid.UUID.String()` normalises, so this is OK today — but the handler also
  never confirms the caller is allowed to write to that campaign. Any
  authenticated DM (or any callsite that calls Service.Upload directly) can
  pass an arbitrary campaign UUID, polluting another campaign's asset tree.
- **Suggested fix:** Verify in the handler that `campaign_id` matches the
  authenticated session's active campaign (the same lookup that
  `dashboard/svelte/src/App.svelte` does via `/api/me`).

## [Medium] Map `UpdateMap` allows shrinking width/height without re-clipping `tiled_json`

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/service.go:124-151`.
- **Spec/Phase ref:** phases §Phase 19 ("Tiled-compatible JSON storage
  format").
- **Problem:** `UpdateMap` validates that the new width/height are within
  bounds, but doesn't compare against the existing `tiled_json` payload's
  layer data length. A DM that shrinks 30×30 to 10×10 (or just edits the
  outer integer fields without touching the JSON) saves a record where
  `tiled_json.width=10` but each layer's `data` array is still 900 entries
  long. The renderer then reads `row*width + col` indices into a buffer that
  no longer matches the declared shape, and out-of-bounds combatants pass
  silently.
- **Suggested fix:** Reject the update when `width_squares`/`height_squares`
  don't match `tiled_json.width`/`height` AND/or re-validate layer lengths
  inside `validateMapFields`.

## [Medium] `RenderQueue` never times-out or drops requests on render failure

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/queue.go:76-93`.
- **Spec/Phase ref:** spec §Map Rendering ("per-encounter render queue with
  debouncing", "command response posted ... immediately without waiting").
- **Problem:** `executeRender` runs `q.renderFn(md)` without a deadline. A
  200×200 map render (40 000 tiles + thousands of FoW rays) on a busy server
  could block the queue's `time.AfterFunc` goroutine indefinitely while a
  fresh `Enqueue` comes in — and since the new request just resets a timer
  that fires in a fresh `time.AfterFunc` goroutine, you can stack up
  unbounded goroutines all racing to render the same encounter. Stop()
  stops timers but does not cancel an in-flight `renderFn`.
- **Suggested fix:** Run `renderFn` under a `context.WithTimeout` (e.g. 5 s)
  and serialize executions per encounter through a single worker goroutine.

## [Low] `RollDamage` does not double the modifier-side dice for spells that allow it (e.g. Sneak Attack crit)

- **Location:** `/home/ab/projects/DnDnD/internal/dice/roller.go:100-124`.
- **Spec/Phase ref:** phases §Phase 18 (crit detection).
- **Problem:** `RollDamage` doubles `Count` for every dice group when
  `critical=true` and applies the static modifier once — correct for
  weapon-base damage per 5e PHB. But Sneak Attack, Divine Smite extra dice,
  and other rider damage rolls go through this same function in calling
  code, meaning all rider dice are also doubled. That matches RAW. However,
  there is no way to call the function with "double main, don't double
  rider" semantics, and no documentation of the assumption. For a
  multi-source crit (Smite + weapon + Hex), callers must build a single
  combined expression — which is exactly what `1d8+1d6+2d8+5` in
  Multi-group-crit tests confirm. So technically correct, but fragile.
- **Suggested fix:** Add a `RollDamageOptions{DoubleDice bool}` per-group or a
  dedicated `RollCritDamage(weapon, riders)` helper to make intent explicit.

## [Low] D20 result `Total` ignores the "min 1 / max 20" sometimes referenced for nat-1 crits with negative DEX

- **Location:** `/home/ab/projects/DnDnD/internal/dice/d20.go:75-82`.
- **Spec/Phase ref:** phases §Phase 18.
- **Problem:** Nothing wrong with the math itself (Total = roll + mod), but
  the comment hints that nat-1 yields "Critical Fail". For an initiative roll
  with a heavily-negative DEX (e.g. exhausted DEX 1 → mod −5) on a nat-1, the
  recorded `Total` is `−4`. The tiebreaker then orders that combatant after
  every other combatant with Total ≥ −4, which is the desired RAW outcome,
  but `initiative_roll` is an `INTEGER` column — negative values are stored
  and used in display strings (e.g. "Initiative −4"). Confirm this is the
  intended UX before locking it in via Phase 25 acceptance.
- **Suggested fix:** Either document that negative initiative totals are
  allowed (RAW), or clamp display to "0" while keeping the underlying total
  for sort purposes.

## [Low] `ColumnLabel` past column 701 (`ZZ`) becomes 3-letter (`AAA`)

- **Location:** `/home/ab/projects/DnDnD/internal/gamemap/renderer/grid.go:65-75`.
- **Spec/Phase ref:** spec §Map Rendering ("coordinate labels (A-Z, AA-AZ,
  etc.)").
- **Problem:** Implementation is correct (`AAA` after `ZZ`), but the
  HardLimitDimension is 200, so the max column is `GR` — well inside the
  2-letter range. No bug, just confirming the implementation matches the
  spec's "A-Z, AA-AZ, etc." idiom; tests cover up to `AAA`.
- **Suggested fix:** None; this is a positive finding noted for completeness.

## [Low] Encounter `display_name` not validated against length / control characters

- **Location:** `/home/ab/projects/DnDnD/internal/encounter/service.go:44-62`.
- **Spec/Phase ref:** spec §Simultaneous Encounters ("display name visible to
  players in Discord").
- **Problem:** Any DM can set `display_name` to a 4096-char string or include
  control characters / Discord markdown injection (`@everyone`, code-block
  closures). `FormatInitiativeTracker` just `Sprintf`s it into messages. The
  combat workspace eventually trims via Discord's API, but the system would
  benefit from a sanitiser at the service boundary.
- **Suggested fix:** Trim/limit to ~64 chars and strip `@`, backticks,
  newlines before persisting.

## [Low] Asset uploads have no per-campaign quota or count cap

- **Location:** `/home/ab/projects/DnDnD/internal/asset/handler.go:31-83`,
  `/home/ab/projects/DnDnD/internal/asset/local_store.go:38-60`.
- **Spec/Phase ref:** phases §Phase 20.
- **Problem:** A DM can upload an unbounded number of 10 MB PNGs per
  campaign; nothing prunes orphans (assets referenced by deleted maps stay on
  disk and in the DB). Fly volumes are finite.
- **Suggested fix:** Add a per-campaign byte/count quota check before `Put`,
  and a soft-delete sweep that removes orphaned `assets` rows / files weekly.

## [Low] `extractRegion` clipping silently truncates non-aligned wall objects

- **Location:** `/home/ab/projects/DnDnD/dashboard/svelte/src/lib/mapdata.js:193-201,
  453-456`.
- **Spec/Phase ref:** phases §Phase 21d (region select + copy/paste).
- **Problem:** `extractObjectsInBounds` only retains an object if its entire
  bounding box is *inside* the selection rectangle. Walls that straddle the
  selection edge are dropped wholesale rather than clipped, so copying a
  3×3 region that includes a wall on its right edge loses that wall on paste.
  This makes "tile corridors" (a stated use case in the spec) brittle.
- **Suggested fix:** Either clip the wall segment at the bounds (preferred,
  matches users' mental model) or document that boundary objects are
  excluded.

## [Low] `UndoStack` push happens *after* mutation, but does not capture the post-paste state for redo correctly

- **Location:** `/home/ab/projects/DnDnD/dashboard/svelte/src/lib/mapdata.js:516-557`,
  `MapEditor.svelte:271-290,302-309`.
- **Spec/Phase ref:** phases §Phase 21d.
- **Problem:** `pushUndo()` clones the *current* (pre-mutation) map and clears
  the redo stack. After `pasteSelection`, the redo stack is wiped — meaning
  a Ctrl+Z after a paste discards the paste, but a Ctrl+Y can't restore it.
  The implementation matches typical undo semantics, but the redo stack is
  cleared on push, so the "undo paste then redo" round-trip is impossible.
- **Suggested fix:** Acceptable for MVP, but document in the user-facing
  hint.

---

## Phase status (Phases 18–25)

- **Phase 18 (Dice Engine):** Issues — see Critical findings on modifier
  parsing and `Nd0` panic; otherwise crit/advantage/disadvantage logic and
  cancellation are correct, and breakdown formatting is sound.
- **Phase 19 (Maps Table & Storage):** Issues — DB doesn't enforce hard
  limit; `tileset_refs` orphaned from HTTP surface; UpdateMap allows
  width/height/JSON divergence.
- **Phase 20 (Assets):** Issues — no MIME allowlist, no campaign auth on
  upload, no quota.
- **Phase 21a (Map Editor — Grid/Terrain/Walls/Save):** OK (no findings).
- **Phase 21b (Map Editor — Image Import & Opacity):** Issue — server PNG
  renderer never composites the background image.
- **Phase 21c (Lighting / Elevation / Spawn Zones):** OK on the JSON write
  path; Magical-darkness tile parsing is wired into FoW. Other lighting
  types (dim, fog, light obscurement) are parsed but only consumed
  elsewhere — verify the spec's "auto-applied combat modifiers" path is hit
  by the obscurement module (out of scope for this review).
- **Phase 21d (Undo / Region / Copy-paste / Duplicate):** Minor — walls at
  selection edges are dropped; redo-after-paste is impossible.
- **Phase 22 (PNG Renderer):** Issues — `TileSize` mutation side effect, no
  hard-limit guard, fog "previously seen" tier missing, DM-sees-all flag
  propagation fragile, stacked-token offset can escape the cell.
- **Phase 23 (Encounter Builder):** Issue — duplicate retains source's
  short_ids, no map_id validation, no position bounds checks.
- **Phase 24 (Encounters & Combatants Tables):** OK on schema; spec-aligned.
- **Phase 25 (Initiative System):** Issue — alphabetical tiebreak is
  case-sensitive byte-wise. Round counter + surprise skip otherwise
  correctly implemented and well-tested.
