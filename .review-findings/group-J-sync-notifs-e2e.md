# Group J Review — Phases 103-120a (Sync + Wiring + Notifications + Misc)

Scope: WebSocket state sync, crash recovery, encounter publisher fan-out, simultaneous encounters, slash-command wiring, DM notifications, exploration, freeform /action, Open5e, error handling, invisible/surprise, campaign pause, Tiled import, test infra, E2E harness, concentration cleanup follow-ups, error_log schema.

Severity guide: Critical = data loss / auth bypass / cross-tenant leak; High = wrong behaviour vs spec impacting a documented user flow; Medium = missing functionality / fragility / spec divergence; Low = polish / cleanup.

---

## [Critical] WebSocket subscribes to any encounter without campaign-ownership check
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:135
- **Spec/Phase ref:** Phase 103 (WS state sync), spec §DM Dashboard / Combat Workspace; F-2 (per-request DM verification)
- **Problem:** `ServeWebSocket` takes `encounter_id` straight from the query string and stuffs it into `client.EncounterID` for `BroadcastEncounter` fan-out. The `/dashboard/ws` route is mounted under `RequireDM` (any-campaign DM), not `RequireCampaignDM`. A DM of campaign A who knows an encounter UUID from campaign B can connect with `?encounter_id=<B-uuid>` and stream full snapshots (HP, hidden-NPC stats, positions) for an encounter they do not own. Multi-tenant scope is therefore enforced only by UUID-guessability, not authorization.
- **Suggested fix:** In `ServeWebSocket`, parse the UUID, load the encounter, resolve its campaign, and verify the authenticated user is that campaign's DM (use the existing `DMVerifier.IsCampaignDM`) before registering the client. Reject mismatch with 403.

## [Critical] Open5e public search endpoint bypasses per-campaign source gating
- **Location:** /home/ab/projects/DnDnD/internal/open5e/handler.go:37 (`RegisterPublicRoutes`); main.go:848
- **Spec/Phase ref:** spec §Extended Content (lines 2541-2546), Phase 111; SR-001 / F-14
- **Problem:** `/api/open5e/monsters` and `/api/open5e/spells` are mounted on the bare router with no auth and no `campaign_id` filter. The spec says "DM enables/disables third-party sources per campaign", but the live search proxy returns the full upstream catalog including books the DM has disabled — players (and anonymous users) can pull arbitrary monsters/spells before they hit the campaign-filtered statblock library. Combined with the cache-on-fetch behaviour in `Service.SearchAndCacheMonster`, anyone can also write to `creatures` table via the protected POST after just learning a slug from this search.
- **Suggested fix:** Either (a) gate the search GETs behind DM auth so only the DM proxies Open5e, or (b) require `?campaign_id=` and filter results through `CampaignSourceLookup` before returning. The catalog list at `/api/open5e/sources` is safe; the per-doc search is the leak.

## [Critical] Open5e HTTP client has no timeout — upstream stall can hang any /search request
- **Location:** /home/ab/projects/DnDnD/internal/open5e/client.go:43
- **Spec/Phase ref:** Phase 111
- **Problem:** `NewClient` defaults `httpClient` to `http.DefaultClient`, which has zero timeout. A slow Open5e API call wedges the goroutine indefinitely. Combined with the un-auth'd public search above, an attacker can pile up goroutines holding DB pool connections through `SearchAndCacheSpell`. Even on the happy path, a degraded Open5e (common) will block dashboard search forever and starve other requests.
- **Suggested fix:** Construct a default `&http.Client{Timeout: 10*time.Second}` (or honour `context.Context` deadlines on each call via `http.NewRequestWithContext` plus a request-scoped timeout) and document the timeout knob.

---

## [High] Saved/Active encounter Campaign Home cards show player-facing display_name, not the spoilery internal name
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/main.go:243-246 and 261-265 (`encounterListerAdapter.{ListActiveEncounterNames,ListSavedEncounterNames}`)
- **Spec/Phase ref:** spec lines 1694, 2840, 3094-3095 ("internal name (DM-only, visible on dashboard)"; display_name = player-facing)
- **Problem:** Both adapters prefer `e.DisplayName` over `e.Name` and surface the result into the DM's Campaign Home cards. Spec explicitly says the internal name is the dashboard-only spoiler-safe name and the display name is the vague version the DM sets for players. The current code shows the vague player-facing string to the DM and hides the spoilery internal name — the inverse of the intended UX.
- **Suggested fix:** Return `e.Name` unconditionally for the dashboard cards (or surface both as separate fields and let the template show internal in primary position).

## [High] Reaction-declaration → dm-queue itemID mapping is in-memory only; lost on restart breaks /reaction cancel
- **Location:** /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:51-86 (`itemIDs map[uuid.UUID]stashedItem`)
- **Spec/Phase ref:** Phase 106c; spec §Reaction Declarations + Encounter End cleanup (line 2901-2906)
- **Problem:** The `declarationID → dm_queue_item_id` map is in-memory. When the bot restarts (Phase 104 crash recovery), the map is empty but the underlying `reaction_declarations` rows and `dm_queue_items` rows survive. A subsequent `/reaction cancel <description>` then silently fails to strike-through the dm-queue message even though it does cancel the declaration. The handler's own comment acknowledges this is "tolerable" — but the spec's "Reaction queues rehydrated" requirement (review prompt #2) is unmet.
- **Suggested fix:** Add a `dm_queue_item_id` column to `reaction_declarations` (or a small mapping table) so the link survives restart. Hydrate on construction.

## [High] DM dashboard error panel cannot render stack trace / structured detail — error_detail column never written
- **Location:** /home/ab/projects/DnDnD/internal/errorlog/recorder.go:18-29 (`Entry`); /home/ab/projects/DnDnD/internal/errorlog/pgstore.go:79-83 (`buildInsertErrorQuery`)
- **Spec/Phase ref:** spec §Error Log (lines 3176-3194), Phase 119; review prompt §19
- **Problem:** The migration `20260427120001_create_error_log.sql` creates `error_detail JSONB` per spec ("optional structured detail (full error msg / stack / fields)"). But `errorlog.Entry` has no `Detail`/`Stack`/`Severity` field, and the INSERT only writes `command, user_id, summary`. The panic recovery middleware (`internal/server/middleware.go:29-41`) captures `debug.Stack()` and logs it but discards it before calling `recorder.Record`. The DM has no way to see what actually went wrong from the dashboard — only a one-line summary.
- **Suggested fix:** Add `Detail json.RawMessage` (and ideally `Severity string`) to `Entry`, populate from `debug.Stack()` + the underlying error in the panic middleware, and include it in `buildInsertErrorQuery`. Update `ListRecent` to surface it.

## [High] /help "Context Tips" shows hardcoded text, not actual remaining resources
- **Location:** /home/ab/projects/DnDnD/internal/discord/help_handler.go:80-96
- **Spec/Phase ref:** Phase 107; spec line 2939 / 2940 ("Context-specific tips (remaining attacks, available slots)")
- **Problem:** `combatContextTips` and `explorationContextTips` are static const strings ("Use /move to relocate (costs movement speed)"). Spec explicitly calls for live context — "remaining attacks, available slots" — derived from the player's current turn / character state. Today the tips give zero personalised info.
- **Suggested fix:** Pull current turn (attacks left, action/bonus used, movement_remaining_ft) and character (spell slot vector) and render those into the tips block. Use the existing `combat.FormatRemainingResources` formatter as the seed.

## [High] One character can be in two active encounters (no DB constraint; LIMIT 1 in query masks the bug)
- **Location:** /home/ab/projects/DnDnD/db/queries/encounters.sql:46-51 (`GetActiveEncounterIDByCharacterID`)
- **Spec/Phase ref:** Phase 105 ("Character limited to one active encounter")
- **Problem:** Spec forbids one character from being in two active encounters at once, but enforcement is purely application-side. The lookup uses `ORDER BY cb.created_at DESC LIMIT 1`, which silently picks the newest and hides any duplicates that slip past. A bug in `StartCombat` (or a race between two simultaneous Start clicks) would dual-list the PC without any visible signal.
- **Suggested fix:** Add a partial unique index `(character_id) WHERE encounter.status='active'` enforced via a CTE-backed insert, or at minimum make `GetActiveEncounterIDByCharacterID` return an error when more than one row matches so the bug surfaces.

## [High] /whisper accepts empty message and spams a dm-queue item
- **Location:** /home/ab/projects/DnDnD/internal/discord/whisper_handler.go:61-80
- **Spec/Phase ref:** Phase 109
- **Problem:** The handler reads `optionString(interaction, "message")` but never checks for empty/whitespace before posting. A player can issue `/whisper` with no text (or coerce a blank with a leading space) and produce blank dm-queue items. The dm-queue inbox then renders the player name with an empty summary, and `PostWhisper` happily marshals an empty `Summary` field that PgStore stores as `''`.
- **Suggested fix:** Reject `strings.TrimSpace(message) == ""` with an ephemeral hint, mirroring how `/action` validates `rawAction`.

## [High] dm-queue Sender bypasses the per-channel MessageQueue (rate-limit ordering)
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/sender.go:19-41
- **Spec/Phase ref:** Phase 9b §rate-limit batching; Phase 106a
- **Problem:** `SessionSender.Send`/`Edit` calls `session.ChannelMessageSend` directly on the raw `discordgo.Session`. The Phase 9b MessageQueue that wraps the session for FIFO + 429 backoff (`main.go:733-736`) is bypassed for every dm-queue event. A busy combat session that emits many `KindEnemyTurnReady` posts can trigger Discord 429s and starve other channels because dm-queue posts skip the queue. The sender comment acknowledges this is "acceptable for Phase 106a" but the wiring has now shipped through 120a without revisiting.
- **Suggested fix:** Route `SessionSender` through the same MessageQueue wrapper used by `discordSession` in main.go, or add a Sender variant that calls into `MessageQueue.Enqueue`.

## [High] WS reader/writer can race on slow-client drop
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:77-101 (`Hub.Run` broadcast block) + 141-167 (writer + reader)
- **Spec/Phase ref:** Phase 103
- **Problem:** When the hub's broadcast block decides a client is slow it does `delete(h.clients, client); close(client.Send)`. The writer goroutine then exits cleanly. But the reader goroutine (line 159-166) is still running and on the next read error sends to `h.hub.Unregister <- client` — a **second** unregister of the same already-dropped client. Hub.Run's unregister branch does `if _, ok := h.clients[client]; ok` and `close(client.Send)` so it skips the close, but the writer also did `defer conn.Close(...)` which is fine. The more concerning race: `h.hub.Register <- client` and `h.hub.Unregister <- client` are unbuffered. If the hub is busy fanning out to N clients (synchronous loop), incoming Register on a new connection blocks the request goroutine; the auth-passed upgrade response can stall arbitrarily under load.
- **Suggested fix:** Buffer the Register/Unregister channels (e.g. 64) so request goroutines never block on hub housekeeping, and guard against double-unregister with a `client.unregistered` boolean.

## [High] Encounter snapshot publisher does NOT trigger on /move position writes
- **Location:** /home/ab/projects/DnDnD/internal/discord/move_handler.go:686-735; combat.Service publish hook
- **Spec/Phase ref:** Phase 104b ("every other service whose mutations can affect an active encounter's combatant state")
- **Problem:** `HandleMoveConfirm` calls `turnProvider.UpdateTurnActions` and `combatService.UpdateCombatantPosition` directly via the move-service adapter (`moveServiceAdapter.UpdateCombatantPosition` → `refdata.Queries.UpdateCombatantPosition`). Neither call goes through a service that has been wired with `SetPublisher`. After a player confirms a move, the dashboard does not receive a fresh snapshot — DMs see stale token positions until something else mutates the encounter. Phase 104b enumerates other services that should publish but skipped the move flow.
- **Suggested fix:** Make `combatService.UpdateCombatantPosition` route through `combat.Service` rather than `*refdata.Queries`, and have the service call `publish(ctx, encounterID)` after commit; alternatively, drop an explicit `publisher.PublishEncounterSnapshot(ctx, turn.EncounterID)` inside `HandleMoveConfirm`'s post-write path.

---

## [Medium] Open5e service caches into globally-visible rows (`campaign_id NULL`) on any auth'd POST
- **Location:** /home/ab/projects/DnDnD/internal/open5e/cache.go:54-66 (`CacheMonster` uses `CampaignID: uuid.NullUUID{Valid: false}`)
- **Spec/Phase ref:** spec §Extended Content + §Homebrew Content (lines 2548-2555); Phase 111
- **Problem:** Cached Open5e creatures/spells write `campaign_id NULL` so a fetch from campaign A also fills campaign B's library. Visibility is enforced only by the statblocklibrary filter checking `open5e_sources` per campaign — but campaign B sees the row exists in the table and can race-enable the source to access it before the DM intends. Spec implies per-campaign caches; the comment at the top of `client.go` acknowledges the global cache as a deliberate choice but it's at odds with the per-campaign source toggle UX.
- **Suggested fix:** Either stash `campaign_id` per cache entry (and key the upsert on (id, campaign_id)), or document the global-cache + per-campaign-filter contract clearly in the dashboard help text so DMs don't think the toggle is private.

## [Medium] WS Hub.Register / Unregister channels can deadlock under sustained slow-client traffic
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:42-51 (unbuffered channels)
- **Spec/Phase ref:** Phase 103
- **Problem:** Both `Register` and `Unregister` are `make(chan *Client)` (unbuffered). `ServeWebSocket` sends to `Register` synchronously, and the reader sends to `Unregister` synchronously. If Hub.Run is stalled on a slow client fan-out (because dozens of subscribers are getting back-pressured non-deterministically), a request goroutine accepting a new WS upgrade waits on `Register <- client` indefinitely.
- **Suggested fix:** Buffer the channels (cap 64+) so housekeeping never blocks request handlers.

## [Medium] Crash-recovery loses in-memory once-per-turn slot tracker (Sneak Attack double-use risk)
- **Location:** /home/ab/projects/DnDnD/internal/combat/service.go:323-324 (`usedEffects map`)
- **Spec/Phase ref:** Phase 104 (crash recovery), spec line 116-121 ("In-flight commands rejected")
- **Problem:** `Service.usedEffects` is purely in-memory and reset on every bot restart. If a Rogue's player uses Sneak Attack on their turn, then the bot restarts before the turn ends, the player can /attack again and trigger Sneak Attack a second time within the same turn. Spec says turn timers are derived from DB fields (not in-memory) — but once-per-turn FES effects are not.
- **Suggested fix:** Persist used FES effect slots on the `combatants` row (or a side table) keyed by (turn_id, combatant_id, effect_type) and read it back in `usedEffectsSnapshot`.

## [Medium] Hub broadcast drops slow clients but never closes their writer goroutine's conn
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:77-101
- **Spec/Phase ref:** Phase 103
- **Problem:** When the hub deletes a slow client and `close(client.Send)`, the writer goroutine exits (range over Send breaks) and `defer conn.Close` fires. The reader, however, is still parked on `conn.Read(r.Context())`. `conn.Close` from the writer will unblock the reader with an err, and reader will attempt `h.hub.Unregister <- client` — but the client isn't in the map anymore (already deleted), so it's a no-op. Net result: a dropped slow client briefly holds two goroutines until the reader fails its next read. Not a leak in the steady state, but the reader has no read-deadline so a half-closed TCP connection can keep the reader alive indefinitely.
- **Suggested fix:** Set a per-read deadline (e.g. `conn.SetReadLimit` + periodic ping/pong) so detection is bounded.

## [Medium] Campaign #the-story announcer resolves channel by name on every announce (drift if renamed)
- **Location:** /home/ab/projects/DnDnD/internal/discord/narration_poster.go:71-82 (`resolveStoryChannel`)
- **Spec/Phase ref:** Phase 115; spec §Tech Stack + spec line 2914
- **Problem:** Pause/resume announcements iterate every guild channel looking for `name == "the-story"`. If the DM renames the channel (or runs `/setup` after one exists), pause/resume silently fails. The campaign settings JSONB already stashes a `channel_ids` map for the COMBAT cluster — the announcer should reuse that.
- **Suggested fix:** Have `CampaignAnnouncer` consume `CampaignSettingsProvider` and pull `channel_ids["the_story"]` first, falling back to the name scan only when settings have no entry.

## [Medium] `LIMIT 1` masks duplicate active-encounter rows for a character (no error)
- **Location:** /home/ab/projects/DnDnD/db/queries/encounters.sql:46-51 (same query)
- **Spec/Phase ref:** Phase 105
- **Problem:** When `GetActiveEncounterIDByCharacterID` is called and the character somehow ended up in two active encounters, the query returns the newest without warning. Combined with the per-user resolver (`discord_adapters.go:74`), the player is routed to the most recently created encounter — silently dropping their commands against the other encounter. There is no observability for the corrupt state.
- **Suggested fix:** Change to `:many`, surface duplicates as an `ErrAmbiguousEncounter`, and let callers either prompt the player to specify or page the DM.

## [Medium] Tiled import accepts massive maps up to `HardLimitDimension` with no tile-count guard
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/import.go:88-103 (`checkHardRejections`)
- **Spec/Phase ref:** Phase 116; spec §Tiled Import
- **Problem:** Only width/height are bounded; total tile count (`width × height × layers`) is unconstrained. A 500×500 map with 8 tile layers is 2 M cells of int32 data per layer load — and the renderer parses every cell. Spec says "hard rejection: infinite maps, non-orthogonal, **too large**". "Too large" should bound the cell budget too, not just one dimension.
- **Suggested fix:** Reject when `width*height > HardLimitCells` (e.g. 100k) or compute a layer-aware budget.

## [Medium] Tiled `version`/`tiledversion` not validated
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/import.go:55-84 (`ImportTiledJSON`)
- **Spec/Phase ref:** Phase 116
- **Problem:** No assertion that the payload is actually a Tiled `.tmj`. A `{"width":10,"height":10,"orientation":"orthogonal","layers":[]}` blob is accepted as valid — no schema-version gate. Future Tiled changes (e.g. ChunkData byte arrays) will silently corrupt imports.
- **Suggested fix:** Require `type == "map"` and `version` matching a known set; emit a warning when `tiledversion` doesn't match the bundled tileset version.

## [Medium] Open5e cache silently rewrites partial monster data with defaults instead of skipping
- **Location:** /home/ab/projects/DnDnD/internal/open5e/cache.go:110-156 (`monsterToParams`)
- **Spec/Phase ref:** Phase 111
- **Problem:** Missing size, type, CR, speed, and ability scores are filled with `Medium`, `beast`, `0`, `{walk:30}`, `{all:10}` defaults — logged at Warn but still persisted. SRD seeding has the opposite policy ("Records that fail validation are skipped"). A homebrew/non-conforming Open5e doc therefore lands a row that *looks* SRD-valid in the library, with no `homebrew=true` flag (the row sets `Homebrew: false`) and no UI hint that the row was reconstructed.
- **Suggested fix:** Either reject rows missing required fields (matching SRD policy) or mark partial-default rows with `source = "open5e:<slug>:partial"` and add a `data_completeness` indicator so the DM can spot incomplete entries.

## [Medium] dm-queue Post stores `messageID = itemID` placeholder on Send failure → Resolve/Cancel later 404s
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/notifier.go:163-186 (`Post`) + 200-218 (`Cancel`/`Resolve` use `item.MessageID`)
- **Spec/Phase ref:** Phase 106a (SR-002 insert-then-send)
- **Problem:** SR-002 ordering is correct (Insert before Send), but when `Sender.Send` fails, the row keeps the placeholder `messageID = itemID`. Later when the DM resolves from the dashboard, `Notifier.Resolve` calls `Sender.Edit(channelID, itemID, …)` and Discord returns 404 (no such message) — the resolve still succeeds in the DB but never delivers the strike-through. Worse: the dashboard considers the resolve "done" and the DM has no follow-up signal.
- **Suggested fix:** When `MessageID` still equals `ItemID` (placeholder), have `Cancel`/`Resolve` re-issue `Send` to recover, or surface the Edit error so the dashboard knows to re-send.

## [Medium] Tiled import silently strips group layers but doesn't preserve child-layer property inheritance
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/import.go:115-119 (group flattening)
- **Spec/Phase ref:** Phase 116; spec §Tiled Import skip-with-warning list
- **Problem:** `sanitizeLayers` flattens `group` layers into the root list and discards group-level metadata (offsets, opacity, visibility, custom properties). If the DM modelled walls as a group with `opacity:0.5`, child layers lose that, and any per-group custom properties (occasionally used to mark "elevation" zones) are dropped silently. The summary reports "group_layer" once but not which child lost what.
- **Suggested fix:** Either reject groups (force the DM to flatten in Tiled) or copy group props/opacity onto each child before flattening.

## [Medium] Action handler exploration cancel doesn't strike-through dm-queue item
- **Location:** /home/ab/projects/DnDnD/internal/discord/action_handler.go:495-509 + `performExplorationCancel`
- **Spec/Phase ref:** Phase 110a; spec dm-queue table ("Action cancelled" row)
- **Problem:** When the player runs `/action <text>` in exploration the handler `postExplorationDMQueue`s an itemID but never stashes it linked to the `pending_actions` row. A subsequent `/action cancel` calls `CancelExplorationFreeformAction(ctx, combatantID)` which deletes the pending_actions row but has no way to find the matching dm_queue_items row to call `Notifier.Cancel` on. The "Cancelled by player" strike-through that the spec advertises never lands.
- **Suggested fix:** Persist `dm_queue_item_id` on `pending_actions` (or a side mapping) so the cancel path can call `notifier.Cancel(itemID, "Cancelled by player")`.

## [Medium] Action handler combat cancel allows incapacitated combatants but block path treats them equally
- **Location:** /home/ab/projects/DnDnD/internal/discord/action_handler.go:368-380
- **Spec/Phase ref:** Phase 110a, C-43 (dying combatant block)
- **Problem:** The `if !isCancel { incapacitatedRejection(...) }` lets a dying combatant cancel a pending action — but the cancel still runs through `combat.Service.CancelFreeformAction` which expects a valid `turn`. If the cancel happens after the dying combatant's turn ended, `combatantBelongsToUser` already passed (their character_id matches), but the current `turn.CombatantID` is someone else's, leading to a spurious "not your turn" message when the player tries to withdraw their action.
- **Suggested fix:** Resolve the pending_actions row by combatant_id rather than by current turn, so cancel works regardless of turn state.

## [Medium] Registration dm-queue posts bypass the unified `dmqueue.Notifier`
- **Location:** /home/ab/projects/DnDnD/internal/discord/registration_handler.go:312 (`postDMQueueNotification` directly calls `session.ChannelMessageSend`)
- **Spec/Phase ref:** Phase 106a + spec §DM Notification System ("DM's single notification hub")
- **Problem:** /register, /import, and /create-character post their approval requests by raw `session.ChannelMessageSend`, NOT through `dmqueue.Notifier.Post`. The spec mandates `#dm-queue` is the single hub with structured Item rows and dashboard "Resolve →" links. These approval requests are missing from `dm_queue_items` (and thus from the unified dashboard inbox/Action Resolver list), so the DM has to switch surfaces between #dm-queue + the approval queue.
- **Suggested fix:** Add a `KindCharacterApproval` event kind and route the registration handlers through `dmqueue.Notifier.Post`, deep-linking back to the approval dashboard page.

## [Medium] Reaction-handler stash leaks: declarations that get DM-resolved don't clear itemIDs map
- **Location:** /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:178-185 (stash on Post); no removal path on DM-side resolve
- **Spec/Phase ref:** Phase 106c
- **Problem:** When the DM resolves a reaction declaration from the dashboard, the corresponding entry in `h.itemIDs` is never deleted. Across a long combat (10+ declarations per round × many rounds) the map grows unbounded. Not a critical leak since the process turns over with the bot, but a long-running deploy will accumulate.
- **Suggested fix:** Periodically prune entries whose underlying declaration is no longer `status='active'`, or hook the dm-queue resolve callback to call `h.dropStashed(declID)`.

## [Medium] dashboardCampaignLookup `IsDM(true)` is not the same as `IsCampaignDM(specific id)` for WS gate
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/dm_middleware.go:48-75 (`RequireDM`); routes.go:21 (WS mounted under `RequireDM`)
- **Spec/Phase ref:** F-2 spec line 65
- **Problem:** Same root cause as the Critical finding above, recorded separately as a gap in the middleware coverage. `RequireDM` returns true if the user is the DM of *any* campaign; `RequireCampaignDM` would scope to the specific encounter's campaign. The WS route uses only `RequireDM`, so cross-campaign reads are permitted.
- **Suggested fix:** Add a campaign-scoped WS auth helper that derives campaign_id from encounter_id before allowing register.

## [Medium] /action freeform combat-mode skips turnGate when no gate is wired (tests-only path leaks to prod)
- **Location:** /home/ab/projects/DnDnD/internal/discord/action_handler.go:300-309 (the `else if err := doCombatAction(ctx)` branch)
- **Spec/Phase ref:** Phase 110a, F-4 turn-gate
- **Problem:** If `SetTurnGate` is not called, the handler still runs `doCombatAction` directly with no advisory-lock protection. In tests this is fine; in production a wiring bug (turnGate stays nil) silently downgrades to the unlocked path with no warning. There is no `logger.Warn` or runtime assertion that the gate is set.
- **Suggested fix:** Log Warn (or fail fast) when `turnGate == nil` and the combat path is exercised in a non-test build; require explicit opt-in for "no gate".

## [Medium] /distance handler not visible in scope but several Phase-105 handler tests use a typed-nil interface trap
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go:186-200 (the `if deps.queries != nil` guard comment explicitly calls out the typed-nil trap)
- **Spec/Phase ref:** Phase 105b
- **Problem:** The wiring code carefully guards against typed-nil interfaces. The same defensive pattern is NOT applied to `discordHandlerSet.attune.SetPublisher(magicItemSvc)` in main.go:1434 — if `magicItemSvc` were ever nil-able (it isn't today, but the API surface allows it), the publisher would silently store a typed-nil. Low-immediacy risk but a code-smell that has caused at least one similar bug in this repo.
- **Suggested fix:** Either move the nil check into `SetPublisher` or add a `// invariant: must be non-nil` comment so the guard is obvious next time.

---

## [Low] Open5e cache `idPrefix` (`open5e_`) is checked in only one place; no enforcement on stat-block readers
- **Location:** /home/ab/projects/DnDnD/internal/open5e/cache.go:51
- **Spec/Phase ref:** Phase 111
- **Problem:** The disjoint-namespace claim is documented but not enforced. A future bug that lets users pass arbitrary `id` to `creatures` writes would let an Open5e payload squat an SRD slug.
- **Suggested fix:** Add a check in `UpsertCreatureParams` validation that rejects manually-passed IDs starting with `open5e_` outside the cache path.

## [Low] EncounterListerAdapter returns `[]string{}` instead of nil for "no campaign", masking the no-active-campaign distinction
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/main.go:248-251 + 268-272
- **Spec/Phase ref:** Campaign Home spec
- **Problem:** Both lookups always return an empty slice on the "no campaign id" path. The Campaign Home template treats empty-slice the same as "no encounters" — the DM can't distinguish "no campaign" from "campaign with no encounters", which matters when the DM expects to see a campaign they just created.
- **Suggested fix:** Return a sentinel (e.g. nil + non-nil error sentinel for "no campaign") so the template can render "No campaign — run /setup".

## [Low] WS client EncounterID query param is not UUID-validated
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:135
- **Spec/Phase ref:** Phase 103
- **Problem:** Any string is accepted as `EncounterID` and used as a fan-out key. Garbage strings just yield no messages, but a misconfigured client could subscribe with an unparseable value and silently fail. Combined with the Critical finding above, validating + comparing to authorized encounters would close both gaps.
- **Suggested fix:** Reject with 400 when `encounter_id` is non-empty and not a valid UUID.

## [Low] dm-queue PgStore.ListPending uses `context.Background()` from `Notifier.ListPending`/`Get`
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/notifier.go:222-229, 320-327
- **Spec/Phase ref:** Phase 106a
- **Problem:** `Get` and `ListPending` consume `context.Background()` and swallow store errors (return `false`/`nil`). The dashboard inbox cannot distinguish "no pending items" from "DB unreachable" and the DM has no way to know the panel is stale.
- **Suggested fix:** Accept a ctx parameter on these methods and surface errors to the caller.

## [Low] E2E `SeedDMApproval` bypasses dashboard approval HTTP endpoint
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/e2e_harness_test.go:168-184
- **Spec/Phase ref:** Phase 120a §"replace `TestE2E_StatusScenario` with `TestE2E_RegistrationScenario` that drives `/register → DM approve → welcome…`"
- **Problem:** The harness calls `registration.Service.Approve` directly rather than hitting the dashboard's approval mutation. The Phase 120a scope says "extend the e2e harness with a `SeedDMApproval` helper (issues the dashboard's approval mutation directly)" — currently the helper bypasses the HTTP route entirely. The /register and welcome DM legs are covered, but the dashboard's approval endpoint stays untested end-to-end.
- **Suggested fix:** Have `SeedDMApproval` POST through `dashboard/approve/{id}` so the auth middleware + handler are exercised.

## [Low] Tiled import silently coerces `tilesets` field to `[]any` even when caller supplies an object
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/import.go:69-72
- **Spec/Phase ref:** Phase 116
- **Problem:** A malformed payload with `"tilesets": {}` (object instead of array) fails the type assertion and is just dropped, producing a map with no tilesets but no error. The DM thinks the import succeeded and only notices when rendering fails.
- **Suggested fix:** When the field is present and the wrong shape, return `ErrInvalidTiledJSON`.

## [Low] HelpHandler topic table is case-sensitive but spec advertises class names "rogue", "cleric"
- **Location:** /home/ab/projects/DnDnD/internal/discord/help_handler.go:44-48 (`helpTopics[topic]`)
- **Spec/Phase ref:** Phase 107
- **Problem:** Discord application command option choices are case-preserving. If the DM registered the option as a free-form string, `/help Rogue` (capitalised) won't match the lowercase `helpTopics` keys and the player gets "Unknown help topic".
- **Suggested fix:** `topic = strings.ToLower(strings.TrimSpace(topic))` before the lookup.

## [Low] Exploration `EndExploration` doesn't notify dashboard / clear PCs
- **Location:** /home/ab/projects/DnDnD/internal/exploration/service.go:172-185
- **Spec/Phase ref:** Phase 110
- **Problem:** Marking an encounter status=completed via the exploration service flips the row but leaves all combatants intact. There's no concentration cleanup, no map state freeze announcement, no dashboard push. Compare to combat `EndCombat` which runs the full cleanup chain.
- **Suggested fix:** Either route `EndExploration` through a shared cleanup helper or document that exploration end is a no-op transition that requires the DM to manually clean up.

## [Low] Reaction handler doesn't return `ErrItemNotFound` from `cancelDMQueueItem` for missing entries
- **Location:** /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:257-269
- **Spec/Phase ref:** Phase 106c
- **Problem:** When the cancel path can't find the stashed item ID (e.g. across the restart-loss already filed), it silently no-ops without warning the player. The player sees "Cancelled reaction" but the dm-queue still shows the original (now-cancelled-in-DB) declaration as pending — confusing state.
- **Suggested fix:** When stash miss + bot has been up long enough that the declaration must be old, surface "Cancellation reached the database but the queue message may need manual cleanup" to the player.

## [Low] Combat enemy-turn notifier label fallback when SetEncounterLookup is unset
- **Location:** /home/ab/projects/DnDnD/internal/discord/enemy_turn_notifier.go (SetEncounterLookup wiring)
- **Spec/Phase ref:** Phase 105 / Phase 105b
- **Problem:** Phase 105b is wired correctly at main.go:421, but the comment at handler.go:182 still says "Phase 105 left this with an empty fallback when the lookup is nil". The code is fine; the doc-comment is stale and likely to mislead future readers searching for the bug.
- **Suggested fix:** Update the comment to reflect that the lookup is now mandatory wiring and the fallback is dead code (or remove the fallback).

## [Low] Health endpoint `/health` only reports two subsystems (db, discord); spec implies more
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/main.go:1336 (`health.Register("discord", …)`)
- **Spec/Phase ref:** spec §Monitoring & Observability (lines 2959-2964)
- **Problem:** Spec mentions PostgreSQL connectivity + Discord gateway. The current `/health` reports just those two, but spec also says "Application-level observability comes from structured logs" — health check is good. However, the response is `200 OK` if both pass, with no degradation detail when one is unset (`databaseURL == ""` path on main.go:1566-1567 registers "not configured" checkers but the spec's response examples include `uptime` which isn't populated).
- **Suggested fix:** Include `uptime` field in the JSON body per spec example.

## [Low] Errorlog Entry has no Severity field; spec asks for severity
- **Location:** /home/ab/projects/DnDnD/internal/errorlog/recorder.go:18-29
- **Spec/Phase ref:** review prompt §19 ("severity")
- **Problem:** The user's review explicitly listed `severity` in the schema fields. The Entry struct has Command/UserID/Summary/CreatedAt only. Migration doesn't create a severity column either. The dashboard panel cannot filter by ERROR vs WARN.
- **Suggested fix:** Add a `severity` column to error_log (default 'error') and an Entry field. Populate from the slog level at record time.

## [Low] `dmqueue.Sender` doesn't wrap message-too-long; long whispers will fail at Discord's 2000-char limit
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/sender.go (no splitting)
- **Spec/Phase ref:** spec §Testing Strategy line 3014 ("Message splitting when content exceeds Discord's 2000-character limit")
- **Problem:** Whisper replies use `WhisperReplyDeliverer.SendDirectMessage` which the DirectMessenger does split. But `Notifier.Resolve` posts the `outcome` text via `Sender.Edit`, and a long DM outcome edit can exceed Discord's 2000 char limit. There's no splitting at the dm-queue sender level.
- **Suggested fix:** Truncate or split outcome text before Edit.

## [Low] /action subcommand normalisation misses underscore variants
- **Location:** /home/ab/projects/DnDnD/internal/discord/action_handler.go:315-317 + 321-333
- **Spec/Phase ref:** Phase 110a
- **Problem:** `normalizeActionSubcommand` lower-cases but doesn't strip underscores; `isDispatchSubcommand` checks for `"action-surge"` and `"channel-divinity"` but not `"action_surge"` or `"channel_divinity"`. Discord command names often use snake_case in older clients; this means `/action action_surge` falls into the freeform path silently.
- **Suggested fix:** Strip both `-` and `_` in `normalizeActionSubcommand`.

## [Low] Snapshot.Build does GetEncounter + ListCombatants + GetTurn in three trips per publish
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/snapshot.go:52-83
- **Spec/Phase ref:** Phase 103
- **Problem:** Each PublishEncounterSnapshot makes 3 DB roundtrips. Under combat with 6 combatants and a 30s turn timer, this adds up. Not a correctness issue but a scalability one.
- **Suggested fix:** Add a single `:many`/CTE query that loads all three in one round-trip, or cache by encounterID with invalidation on publish.

## [Low] WS writer uses `r.Context()` for write deadlines, but request context is cancelled on disconnect
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/ws.go:146-148
- **Spec/Phase ref:** Phase 103
- **Problem:** `context.WithTimeout(r.Context(), 5*time.Second)` derives from the request context. When the WS handler returns, `r.Context()` is canceled — but the writer goroutine continues to use it. Every subsequent write times out instantly because the parent context is dead. The writer then errors and exits — which is also when the reader fails — but the writer's claimed "5-second write deadline" only holds on the *first* write after register.
- **Suggested fix:** Use `context.Background()` (or a hub-scoped context) for write deadlines so they actually have the configured duration.

## [Low] CampaignAnnouncer announcement is best-effort with no logging
- **Location:** /home/ab/projects/DnDnD/internal/campaign/service.go:192-195
- **Spec/Phase ref:** Phase 115
- **Problem:** `_ = s.announcer.AnnounceToStory(...)` silently swallows errors. When pause/resume announce fails (channel missing, bot offline, 429), the DM has no signal — the campaign status flips but the player never sees the announcement.
- **Suggested fix:** Log Warn at minimum (`s.logger` is not currently a Service field; add it).

## [Low] DM-queue inbox list doesn't paginate
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/pgstore.go:159-164 (ListAllPendingDMQueueItems)
- **Spec/Phase ref:** Phase 106a/106b
- **Problem:** `ListAllPendingDMQueueItems` returns every pending row across every campaign with no LIMIT. For a busy multi-campaign deployment the inbox could load thousands of rows on every dashboard refresh.
- **Suggested fix:** Add pagination params and a sane default LIMIT (50).

## [Low] dmqueueChannelResolver and channel_ids are not validated to be in the bot-accessible guild
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/main.go:302 (`newDMQueueChannelResolver`)
- **Spec/Phase ref:** Phase 106a
- **Problem:** The resolver scans guild channels by name (similar to story channel). A DM who renames `#dm-queue` after /setup breaks dm-queue posts silently — no health-check, no warning. Spec lines mention that `#dm-queue` is configured via `/setup` but the persistence is via the channel name not the stored channel id.
- **Suggested fix:** Wire through `channel_ids["dm_queue"]` from campaign settings (the /setup handler already persists it).

## [Low] Phase 118c CI guard comment present but no actual CI workflow file enforces it
- **Location:** /home/ab/projects/DnDnD/docs/phases.md (Phase 118c "load-bearing deliverable")
- **Spec/Phase ref:** Phase 118c
- **Problem:** Phase 118c calls the CI guard ("run sqlc generate && git diff --exit-code") the load-bearing deliverable. I found no `.github/workflows/*.yml` (and no Makefile target) that runs `sqlc generate` and fails on dirty diff. The Phase is marked done but the guard is missing.
- **Suggested fix:** Add a GitHub Actions step (or `make sqlc-check`) running `sqlc generate && git diff --exit-code`, and gate PRs on it.

## [Low] /reaction in exploration mode is allowed even though spec ties reactions to combat
- **Location:** /home/ab/projects/DnDnD/internal/discord/reaction_handler.go (no encounter mode check)
- **Spec/Phase ref:** Phase 106c; spec §Reaction Declarations
- **Problem:** `handleDeclare` resolves to an encounter regardless of `encounter.Mode`. In exploration mode there are no turns/reactions; the declaration row still lands and clutters the dm-queue.
- **Suggested fix:** Reject `/reaction` outside combat with "Reactions are only available during combat encounters."

## [Low] Open5e cache "manual" resolution default isn't surfaced to DM
- **Location:** /home/ab/projects/DnDnD/internal/open5e/cache.go:215-225
- **Spec/Phase ref:** Phase 111
- **Problem:** Every Open5e spell is cached with `ResolutionMode: "manual"` (no auto-resolve hook). The DM sees no indicator that the spell needs full adjudication versus the SRD versions; this is fine functionally but easy to miss.
- **Suggested fix:** Surface `resolution_mode` in the stat-block library UI so the DM knows what to expect.

## [Low] CampaignAnnouncer story-channel resolution uses Channel*Send rather than the resolved poster's split logic
- **Location:** /home/ab/projects/DnDnD/internal/discord/campaign_announcer.go:30
- **Spec/Phase ref:** Phase 115
- **Problem:** Cosmetic — pause/resume announcements bypass `NarrationPoster.Post` even though the spec's pause copy is short. Long-term, if announcement copy grows beyond 2000 chars it will fail.
- **Suggested fix:** Reuse NarrationPoster with no-attachment payload.

## [Low] Exploration spawn assignment is row-major; spec doesn't dictate ordering and may surprise DMs
- **Location:** /home/ab/projects/DnDnD/internal/exploration/spawn.go (AssignPCsToSpawnZones)
- **Spec/Phase ref:** Phase 110
- **Problem:** PCs are assigned to spawn tiles row-major in CharacterIDs order. The DM may expect alphabetical-by-character, or by initiative. Spec doesn't pin this so it's a documentation gap.
- **Suggested fix:** Document the deterministic ordering in the playtest doc.

## [Low] Phase 106f "remove passthroughMiddleware" — still defined and exported in main.go
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/main.go:319
- **Spec/Phase ref:** Phase 106f
- **Problem:** `passthroughMiddleware` is still present (defined and referenced from a few test mounts). Phase 106f says "Replace the `passthroughMiddleware` currently protecting `RegisterDMQueueRoutes`" — the dm-queue route IS now behind `dmAuthMw`, but the helper itself remains as a footgun: a future contributor could accidentally re-mount with passthrough.
- **Suggested fix:** Either move `passthroughMiddleware` into a `_test.go` file or rename it explicitly (e.g. `testPassthroughMiddleware`) so it can't be used in main wiring.

---

## Phase-by-phase summary

- Phase 103: WS state sync — Critical scope leak (cross-campaign WS subscribe) + medium concurrency concerns; otherwise functional.
- Phase 104: Bot crash recovery — Functional path is correct (stale-turn scan before gateway open); medium gap in once-per-turn FES persistence.
- Phase 104b: Publisher fan-out & store adapter cleanup — Adapter cleanup landed (`combat.NewStoreAdapter`). Fan-out misses /move position writes (High).
- Phase 104c: Mount `levelup.Handler` — OK.
- Phase 105: Simultaneous encounters — OK functional; medium concern around DB constraint for one-encounter-per-character.
- Phase 105b: Discord handler wiring — OK (every handler wired in main.go; SetEncounterLookup present).
- Phase 105c: DM display-name editor — OK.
- Phase 106a: dm-queue core — Medium issues with placeholder messageID + Sender bypassing MessageQueue.
- Phase 106b: dm-queue remaining events + whisper — OK; medium /whisper validation gap.
- Phase 106c: /reaction wire — High: in-memory itemID mapping loses cancel ability on restart.
- Phase 106d: /check narration gate — OK.
- Phase 106e: /use wiring — OK.
- Phase 106f: dm-queue auth — OK (dmAuthMw on routes), with low cleanup of `passthroughMiddleware`.
- Phase 107: /help — High: context tips are hardcoded text, not live state.
- Phase 108: /status — OK.
- Phase 109: /whisper — High: no empty-message validation.
- Phase 110: Exploration — Low gaps (EndExploration cleanup, spawn ordering doc).
- Phase 110a: /action freeform wiring — Medium gaps (cancel doesn't strike-through, no warning when turnGate nil, dying-cancel race).
- Phase 111: Open5e integration — **Three Criticals** (no auth on search, no timeout, global cache w/ source filter mismatch).
- Phase 112: Error handling & observability — High: error_detail/severity/stack never persisted.
- Phase 113: Invisible condition — OK (advantage/disadvantage paths present in advantage.go).
- Phase 114: Surprise — OK (integration tests cover skip, reaction availability post-turn).
- Phase 115: Campaign pause — Medium: story channel resolved by name, no channel_ids reuse.
- Phase 116: Tiled import — Medium gaps in size/version validation + group-layer flattening fidelity.
- Phase 117: Testing infra & coverage — OK.
- Phase 118: Concentration cleanup integration — OK.
- Phase 118b: Concentration cleanup polish — OK.
- Phase 118c: sqlc drift reconciliation — Low: CI guard appears missing.
- Phase 119: Error log schema — High: schema landed but Go side omits error_detail.
- Phase 120: E2E harness — OK.
- Phase 120a: E2E scenario backfill — Low: SeedDMApproval bypasses dashboard HTTP route.
