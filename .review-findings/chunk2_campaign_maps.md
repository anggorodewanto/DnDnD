# Chunk 2 Findings — Phases 11–22 (Campaign + Channels + Commands + Registration + Dashboard skeleton + Char Cards + Dice + Maps + Assets + Editor + PNG Renderer)

Reviewed against `docs/phases.md` (lines 60–134) and `docs/dnd-async-discord-spec.md`.

## Summary

Code-level implementation of every Phase 11–22 unit is present and tested in isolation: the `campaign`, `discord/setup`, `registration`, `charactercard`, `dice`, `gamemap`, `asset`, dashboard scaffolding, and `gamemap/renderer` packages all carry meaningful logic with broad unit coverage. The serious problems live in the **production wiring layer** (`cmd/dndnd/main.go`): the `/setup` slash command is registered but has **no handler attached** in production (`NewCommandRouter(bot, nil, …)`), so the very first user-facing step of Phase 12 returns "Unknown command"; `RollHistoryLogger`, `MapRegenerator`, `PlayerNotifier`, and `OnCharacterUpdated` are all defined in the domain packages but **never instantiated for production**, leaving Phase 17/18/22 partially live. The Phase 21a placeholder campaign id (`00000000-0000-0000-0000-000000000001` in `App.svelte:24`) is still hard-coded as flagged in `docs/phases.md:114`. Asset storage location (`data/assets`) does not match the fly volume mount at `/data` (Phase 20). Several fields on character cards (conditions, concentration, exhaustion) are intentionally deferred — already documented in MEMORY.

## Per-phase findings

### Phase 11 — Campaign CRUD & Multi-Tenant Scoping  ⚠️
- ✅ Service `CreateCampaign / Get* / Update* / Pause / Resume / Archive` with status transition guard at `internal/campaign/service.go:80-184`.
- ✅ Settings JSONB struct (`turn_timeout_hours`, `diagonal_rule`, `open5e_sources`, `channel_ids`, …) at `internal/campaign/service.go:21-36`.
- ✅ Spec announcement strings copied verbatim (`internal/campaign/service.go:127-129`).
- ✅ Pause/Resume HTTP endpoints at `internal/campaign/handler.go:26-59`; resume re-pings current-turn player via `TurnPinger` (Phase 115 hookup at line 145).
- ✅ Test coverage: service_test, handler_test, integration_test all present.
- ⚠️ **No production code path calls `Service.CreateCampaign`.** `grep -r CreateCampaign cmd/` only finds the handler signature and tests. The Phase 11 done-when bullet "Campaign created on `/setup`" is NOT met: the `/setup` handler at `internal/discord/setup.go:199` calls `GetCampaignForSetup(guildID)` and **errors out** ("no campaign found for this server. Create a campaign first.") if no campaign row exists. There is no other production entry point that creates one — `docs/playtest-quickstart.md:114-124` only mentions `/setup` for channel creation, not campaign creation, leaving a gap.

### Phase 12 — `/setup` Channel Structure  ❌
- ✅ Channel structure matches spec lines 131–151: SYSTEM (`initiative-tracker`, `combat-log`, `roll-history`), NARRATION (`the-story`, `in-character`, `player-chat`), COMBAT (`combat-map`, `your-turn`), REFERENCE (`character-cards`, `dm-queue`) at `internal/discord/setup.go:25-58`.
- ✅ Permission overrides correct: `the-story` DM-write-only (`theStoryPerms`), `combat-map` bot-write-only (`combatMapPerms`), `dm-queue` DM/bot-only viewable (`dmQueuePerms`).
- ✅ Skip-existing logic via the `existingChannels` map at `internal/discord/setup.go:118-130`.
- ✅ Channel IDs persisted via `SaveChannelIDs` (handler line 211).
- ❌ **Production never wires `SetupHandler`.** `cmd/dndnd/main.go:741` calls `discord.NewCommandRouter(bot, nil, regDeps)` — first arg is the setup handler. With it `nil`, `internal/discord/router.go:237-239` does NOT register `setup`, and `setup` is also absent from the `gameCommands` slice (router.go:198-204), so `/setup` falls through to `respondEphemeral("Unknown command: /setup")` at router.go:265. **This is the largest single blocker for an end-to-end playtest** — it directly contradicts the Phase 12 done-when bullet.

### Phase 13 — Slash Command Registration (Player Commands)  ✅
- ✅ All 33 spec'd commands present in `CommandDefinitions()` at `internal/discord/commands.go:13-519` (verified via name extraction; matches the spec list at phases.md line 71 verbatim).
- ✅ `RegisterCommands` handles bulk overwrite + stale deletion (commands.go:523-554) per spec lines 174-181.
- ✅ Stub handler returns "/<name> is not yet implemented." (`internal/discord/router.go:489-497`).
- ✅ `setup` command included with `DefaultMemberPermissions = ManageChannels` (commands.go:514-518).
- Minor: `setup` is not part of the spec-named "player commands" list but is required for Phase 12; correctly registered.

### Phase 14 — `/register`, `/import`, `/create-character`  ⚠️
- ✅ `/register` exact + Levenshtein fuzzy at `internal/registration/service.go:51-104` and `fuzzy.go`. Matches spec lines 45-49 (suggestions, threshold).
- ✅ Ephemeral confirmations and `#dm-queue` notification at `internal/discord/registration_handler.go:91-105, 211-215, 269-272`.
- ✅ `/import` placeholder path creates pending row from URL (registration_handler.go:197-215). DDB importer is plugged via `WithDDBImporter` opt — production does NOT pass it (intentional per Phase 14 description: "stub — accepts URL").
- ✅ Status-aware game-command stubs via `StatusAwareStubHandler` (registration_handler.go:389-420), wired in `NewCommandRouter` (router.go:213-217).
- ✅ `StatusCheckResponse` covers pending/changes_requested/rejected per spec line 54.
- ⚠️ `/create-character` portal link is **fake** in production. `cmd/dndnd/main.go:730-732` defines `TokenFunc` as `return "e2e-token", nil` — the comment at lines 726-729 explicitly defers the real portal-token issuer to "Phase 14 follow-up." Phase 14's done-when bullet "/create-character returns one-time portal link" is met in code path but the link is a hardcoded placeholder. The `RegisterHandler` would otherwise panic without the func (router.go:222-224).
- ⚠️ Welcome DM (spec lines 183-202) has no implementation; the bot has `welcome.go` but that is GUILD_MEMBER_ADD pre-registration scaffolding, not the `/help` follow-up.

### Phase 15 — Dashboard Skeleton & Campaign Home  ⚠️
- ✅ Sidebar nav with 11 entries (`internal/dashboard/handler.go:33-46`) — covers Campaign Home, Approval, Encounter Builder, Stat Block Library, Asset Library, Map Editor, Exploration, Character Overview, Create Character, Errors. Matches spec line 2794.
- ✅ Campaign Home template renders DMQueueCount / PendingApprovals / ActiveEncounters / SavedEncounters cards + quick-action buttons (`handler.go:202-232`).
- ✅ WebSocket route + reconnect with exponential backoff (1s→30s) using `nhooyr/websocket` at `internal/dashboard/ws.go:111-160` and the inline JS at `handler.go:236-264`.
- ✅ Svelte SPA stub embedded via `embed.FS` (`internal/dashboard/embed.go`, asset dir at `internal/dashboard/assets/`).
- ⚠️ `DMQueueCount` and `PendingApprovals` are **hardcoded to 0** in `ServeDashboard` (`handler.go:149-152`). Phase 15 done-when allows "placeholder data," but Phase 16 is checked off and provides `ListPendingApprovals` — so the count should be live by now.
- ⚠️ `ActiveEncounters` / `SavedEncounters` are empty slices despite Phase 23 (encounter list) being done — no integration glue.

### Phase 16 — Character Approval Queue  ⚠️
- ✅ Approval store + handler with full CRUD: `ListPendingApprovals`, `GetApprovalDetail`, `Approve`, `RequestChanges`, `Reject`, `RetireCharacter` (`internal/dashboard/approval.go`, `approval_handler.go`, `approval_store.go`).
- ✅ Page template + JS at `approval_handler.go:351-549` — list, detail, feedback panel, approve/changes/reject buttons, WS-driven refresh.
- ✅ Per-DM campaign resolution via `CampaignLookup` (approval_handler.go:154-177); production wires `dashboardCampaignLookup{queries: queries}` at `cmd/dndnd/main.go:588`.
- ✅ Card poster wired: `cardPoster = charactercard.NewService(...)` at `cmd/dndnd/main.go:585`; on approve calls `PostCharacterCard`, on retire calls `UpdateCardRetired` (approval_handler.go:271-282).
- ❌ **Player notification not wired in production.** `cmd/dndnd/main.go:580-587` constructs `NewApprovalHandler(... nil ...)` for the `notifier` arg with the explicit comment "Discord DMs on approve/reject are a follow-up." Phase 16 done-when says "player notification" is part of the approve flow — **missing**. Spec lines 53 and 41 require a Discord DM ping with the outcome.
- ⚠️ `/retire` is a Phase-13 stub, so the queue cannot actually surface created_via=retire items end-to-end. The handler treats them correctly when present (approval_handler.go:248-262) but no production code path inserts them.

### Phase 17 — Character Cards (`#character-cards`)  ⚠️
- ✅ Full format at `internal/charactercard/format.go:44-114` — header, HP+temp, AC, speed, abilities, equipped main/off, spell slots with ordinals, conditions, concentration, exhaustion, gold, languages. Matches spec lines 219-228.
- ✅ Spell counts (DDB + portal formats) at `service.go:241-279`.
- ✅ Short ID generator with deterministic ordering at `service.go:156-178`.
- ✅ `PostCharacterCard` / `UpdateCardRetired` invoked from approval flow (above).
- ⚠️ **`OnCharacterUpdated` is never called from production.** `grep` for OnCharacterUpdated shows only the implementation site (`charactercard/service.go:120`) and the dashboard interface (`approval.go:49`). Damage / equip / level-up / condition mutations across `internal/combat`, `internal/levelup`, `internal/equipment` etc. don't drive card edits. Phase 17 done-when "auto-updates on HP/equipment/condition/level changes" is **unmet**.
- ⚠️ `buildCardData` (service.go:203-238) does NOT populate `Conditions`, `Concentration`, or `Exhaustion` — already documented in user MEMORY (`project_character_card_deferred_fields.md`) as intentional deferral to phases 39, 42, and spellcasting.

### Phase 18 — Dice Rolling Engine  ⚠️
- ✅ Expression parser supporting NdM[+K] with multiple groups + signed modifier at `internal/dice/dice.go`.
- ✅ Advantage / Disadvantage / cancellation via `RollD20` at `internal/dice/d20.go:47-82`; `CombineRollModes` covers cancel-out logic at lines 86-97.
- ✅ Critical hit/fail flags on chosen die (d20.go:76-77).
- ✅ `RollDamage(critical)` doubles dice counts but not modifiers (`roller.go:106-110`) — matches 5e crit rule.
- ✅ Comprehensive unit tests at `dice_test.go` (~25 funcs covering parse, advantage, disadvantage, crits, multiple groups, negative modifier).
- ✅ `RollLogEntry` and `RollHistoryLogger` interface at `internal/dice/log.go`.
- ❌ **Production has no `RollHistoryLogger` implementation.** `cmd/dndnd/discord_handlers.go:113, 122, 131-132, 137, 148-…` pass `nil` for every rollLogger arg (with comment "no production adapter yet (tests only)"). Phase 18 done-when "all rolls posted to `#roll-history`" is **unmet end-to-end**. Channel exists, handlers know about it, but the bridge between `dice.RollLogEntry` and `ChannelMessageSend` is missing in `cmd/`.

### Phase 19 — Maps Table & Storage  ✅
- ✅ Migration `db/migrations/20260310120009_create_maps.sql` with width/height >= 1 CHECK, JSONB `tiled_json`, optional `background_image_id`, `tileset_refs` JSONB.
- ✅ Service `CreateMap / GetByID / ListByCampaignID / UpdateMap / DeleteMap` at `internal/gamemap/service.go:73-153`.
- ✅ Soft (100) / hard (200) limit constants with `validateDimensions` and `classifySize` at service.go:178-194.
- ✅ Standard 48px / Large 32px tile constants at service.go:21-23.
- ✅ Tiled-compatible JSON validated via `import.go` ImportTiledJSON (orthogonal-only, infinite rejected, drops unsupported features with `SkippedFeature` audit trail).
- ✅ HTTP handler exposes `/api/maps` CRUD at `internal/gamemap/handler.go:26-35`. Note: registered without auth middleware in `cmd/dndnd/main.go:398` — see cross-cutting risks.

### Phase 20 — Assets Table & AssetStore  ⚠️
- ✅ Migration `20260310120010_create_assets.sql` with FK from `maps.background_image_id → assets.id` (lines 14-16).
- ✅ `Store` interface (`Put`, `Get`, `Delete`, `URL`) at `internal/asset/store.go:32-44`.
- ✅ `LocalStore` writes to `{baseDir}/{campaign_id}/{type_dir}/{uuid}` with UUID filenames at `local_store.go:32-60`. Type-dir map: `map_background→maps`, `token→tokens`, `tileset→tilesets`, `narration→narration`.
- ✅ `Service.Upload` validates type, writes file, creates DB row, cleans up file on DB error (`asset/service.go:44-76`).
- ✅ `/api/assets/upload` (POST) and `/api/assets/{id}` (GET) at `asset/handler.go:26-29`.
- ⚠️ **Fly volume mount mismatch.** `fly.toml:23-25` mounts `dndnd_data` at `/data`, but `cmd/dndnd/main.go:407-409` defaults `assetDataDir` to `data/assets` (relative). Production needs `ASSET_DATA_DIR=/data/assets` to actually persist assets across deploys, but neither `fly.toml`, `Dockerfile`, nor `docs/playtest-quickstart.md` documents this required override. Files written to `./data/assets` inside the container will be lost on every restart.
- ⚠️ Asset API endpoints at `cmd/dndnd/main.go:413` are mounted **without auth middleware**. Public PNGs are tolerable, but cross-campaign isolation is not enforced at the handler layer.

### Phase 21a — Map Editor: Grid, Terrain, Walls, Save/Load  ⚠️
- ✅ `dashboard/svelte/src/MapEditor.svelte` (1137 lines) with `mapdata.js` (576 lines).
- ✅ `generateBlankMap`, `setTerrain`, `addWall/removeWall/getWalls`, `validateDimensions` in `mapdata.js:72-…`. Five terrain types match spec (`open_ground`, `difficult_terrain`, `water`, `lava`, `pit`) at `mapdata.js:5-11`.
- ✅ Save/load via `createMap`/`updateMap`/`getMap` API calls (MapEditor.svelte:100-167).
- ✅ Tiled-compatible JSON with terrain / walls / lighting / elevation / spawn_zones layers at `mapdata.js:86-…`.
- ❌ **Campaign ID still hard-coded to placeholder UUID** at `dashboard/svelte/src/App.svelte:24`: `let campaignId = $state('00000000-0000-0000-0000-000000000001');`. Already flagged in `docs/phases.md:114`. App.svelte passes this string to MapEditor / EncounterBuilder / CombatManager / NarratePanel / etc. — every dashboard panel that needs campaign context. Until OAuth-derived campaign lookup wires through, every production write goes to the same fake campaign UUID and any real campaign breaks. Should have been resolved by Phase 23 per the original note.

### Phase 21b — Image Import & Opacity  ✅
- ✅ Background upload via multipart `uploadAsset` → `/api/assets/upload` (MapEditor.svelte:182-209).
- ✅ Opacity slider (0–1, 5% step) at MapEditor.svelte:825-836; render uses `globalAlpha = backgroundOpacity` then drops terrain to 0.4 alpha when bg present (lines 311-322).
- ✅ Asset stored via `AssetStore`; reload reconstructs `/api/assets/{id}` URL (lines 110-114).

### Phase 21c — Lighting, Elevation, Spawn Zones  ✅
- ✅ Lighting brush: 6 types (normal, dim_light, darkness, magical_darkness, fog, light_obscurement) at `mapdata.js:17-24`. Drawn as 0.4 alpha overlay (MapEditor.svelte:346-360).
- ✅ Elevation per-tile (0–10, ELEVATION_MAX=10) at `mapdata.js:55`; numeric label rendered per tile.
- ✅ Spawn zones with player/enemy types and rectangle-drag UI (MapEditor.svelte:386-419); persisted into Tiled `spawn_zones` object layer.

### Phase 21d — Undo/Redo, Region Select, Copy/Paste, Duplicate  ✅
- ✅ `UndoStack` class consumed via `pushUndo` before each mutation (MapEditor.svelte:216-237). Bound to Ctrl+Z / Ctrl+Shift+Z (lines 270-296).
- ✅ Rectangular selection drag with cyan dashed preview (lines 439-463).
- ✅ Copy/paste via `extractRegion` / `pasteRegion` and Ctrl+C / Ctrl+V handlers; paste preview overlay (lines 239-257, 465-476).
- ✅ `duplicateMap` clones state, resets `savedMapId`, appends "(copy)" to name (lines 260-268).

### Phase 22 — Server-Side PNG Renderer  ⚠️
- ✅ `RenderMap` at `internal/gamemap/renderer/renderer.go:12-84` orchestrates terrain → zone overlays → fog → grid → walls → tokens → coordinate labels → legend, encodes via `image/png`.
- ✅ Auto-downscale 48px→32px for >100 dim (renderer.go:14-16); standalone constants in service.go.
- ✅ Spreadsheet-style column labels (A→Z→AA→AB) with `ColumnLabel` at `grid.go:65-75`; `ParseCoordinate` is the inverse.
- ✅ Token rendering with health-tier color + border style (uninjured solid, scratched arc nick, bloodied dashed) at `token.go:59-90`. Tier icon overlays (warning triangle, heartbeat, X, bandage cross) at `token.go:127-179` — covers the dual-channel accessibility spec lines 270-279.
- ✅ Stacked tokens offset by altitude with `↑Nft` badge at `token.go:32-50, 107-124`.
- ✅ Legend with terrain key + active-effects sections, omitted when only open ground and no effects (`legend.go:18-20`). Hatching for difficult terrain (legend.go:83-85).
- ✅ Per-encounter `RenderQueue` with debounce; only the latest enqueued state renders (queue.go:38-93). Matches spec line 2180.
- ✅ Tests: `renderer_test.go` (266 lines), `token_test.go`, `legend_test.go`, `queue_test.go`, `parse_test.go` — broad coverage of the renderer surface.
- ❌ **Renderer is never invoked in production.** `cmd/dndnd/discord_handlers.go:34` declares `mapRegenerator discord.MapRegenerator` on `discordHandlerDeps` but it is **never set** — production constructs `discordHandlerDeps{...}` (`cmd/dndnd/main.go` ≈ line 691) without the `mapRegenerator` field. Inside `done_handler.go:438-440`, `if mr == nil { return }`, so `PostCombatMap` is a silent no-op. `enemy_turn_notifier.go:69` calls the same. Phase 22 done-when "PNG generated from map JSON + combatant positions" is satisfied at the package layer, but **no production caller drives it** — `#combat-map` never receives images.
- ⚠️ Render queue is wired but no caller `Enqueue`s anything.

## Cross-cutting risks

1. **Production wiring gap is the systemic theme of this chunk.** Domain packages are well-built and tested; integration in `cmd/dndnd/main.go` and `cmd/dndnd/discord_handlers.go` skips several connectors:
   - `setupHandler` — `nil` → `/setup` returns "Unknown command" (Phase 12).
   - `RollHistoryLogger` — `nil` everywhere → no `#roll-history` posts (Phase 18).
   - `MapRegenerator` — never set → no `#combat-map` PNGs (Phase 22).
   - `PlayerNotifier` — `nil` → no Discord DM on approve/reject (Phase 16).
   - `OnCharacterUpdated` — never called → cards stale (Phase 17).
   - `TokenFunc` — fake string → portal links broken (Phase 14).
   The aggregate effect: an end-to-end playtest (which 121.x is supposed to enable) hits multiple silent-no-op surfaces on the very first turn.

2. **Asset volume path mismatch (Phase 20).** Fly mount `/data` vs default `data/assets` — undocumented requirement to set `ASSET_DATA_DIR` will cause data loss on the first deploy.

3. **`/api/maps`, `/api/assets/upload` mount with no auth** (`cmd/dndnd/main.go:398, 413`). Cross-tenant isolation is enforced only by `campaign_id` validity on inputs; any authenticated DM (or none, for upload) could write into another campaign's namespace. Worth a deliberate decision before first live multi-tenant use.

4. **Campaign Home placeholder counts (Phase 15).** With Phase 16 already shipping a real approval store, the dashboard should show non-zero counts. Today the cards always read zero, masking real DM-queue / approval pressure.

5. **App.svelte campaign placeholder UUID (Phase 21a).** All Svelte panels still pass the same hard-coded placeholder UUID in App.svelte:24. Already flagged but unfixed; will produce silent cross-tenancy bugs the moment a real second campaign exists in DB.

6. **Welcome-DM flow not implemented.** Spec lines 183-202 ("Welcome DM on guild member join") has no production code — only the player-onboarding text exists in spec form. Not strictly listed as a Phase 11–22 done-when, but cross-cuts Phase 14.

## Recommended follow-ups

1. **Wire `SetupHandler` in `cmd/dndnd/main.go`.** Highest priority: pass a real handler into `discord.NewCommandRouter`. Decide whether `/setup` also auto-creates the campaign row when none exists for the guild (closes the Phase 11 done-when gap).
2. **Wire `RollHistoryLogger`** as a thin adapter that maps `dice.RollLogEntry → ChannelMessageSend` against the configured `roll-history` channel id; drop the four `nil` args in `cmd/dndnd/discord_handlers.go`.
3. **Wire `MapRegenerator`** so `done_handler.PostCombatMap` and `enemy_turn_notifier.NotifyEnemyTurnExecuted` actually produce PNGs (build map data via `gamemap.Service.GetByID` + `combat.ListCombatantsByEncounterID` → `renderer.RenderMap`, debounced via `renderer.NewRenderQueue`). Set the field on `discordHandlerDeps` and ensure both notifier paths use it.
4. **Wire `PlayerNotifier`** in `dashboard.NewApprovalHandler` (cmd/dndnd/main.go:587) using the existing `direct_messenger` for the three notify methods.
5. **Drive `OnCharacterUpdated`** from the mutation surfaces created in Phases 39 (conditions), 42 (damage), 75a (equipment), 88 (level-up). Cross-reference the MEMORY note `project_character_card_deferred_fields.md`.
6. **Replace the `TokenFunc` placeholder** (cmd/dndnd/main.go:730-732) with the real portal-token issuer once Phase 14's portal lands. Keep the panic guard or default to a clear `errors.New("portal not configured")` to avoid silently issuing fake links.
7. **Default `ASSET_DATA_DIR=/data/assets`** when `FLY_APP_NAME` is set, or document the override loudly in `fly.toml` and Dockerfile. Add an integration smoke test asserting writes survive process restart in the fly mount.
8. **Resolve the App.svelte placeholder UUID** by piping `currentCampaignID` from the OAuth session (the dashboard auth middleware already injects `discord_user_id`; add a campaign lookup → expose it server-side as a template variable or `/api/me` endpoint consumed by the SPA on boot).
9. **Make Campaign Home counts live** — `Handler.ServeDashboard` already has `campaignLookup`; reuse it to call `ListPendingApprovals` and a future `dmqueue.CountPending`.
10. **Auth-gate `/api/maps` and `/api/assets/*`** under `authMw` at `cmd/dndnd/main.go:398, 413`, matching the pattern used at line 589 for approval routes.
11. **Auto-attach `OnCharacterUpdated`** to the encounter event bus (Phase 27) once Phase 17 is revisited.
