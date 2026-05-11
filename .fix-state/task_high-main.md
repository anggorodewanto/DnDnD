# task high-main — bundled main.go wiring fixes (high-09, high-10, high-13, high-14, high-17)

## Findings (verbatim from chunks)

### high-09 — RollHistoryLogger production adapter (chunk2 Phase 18)
> ❌ **Production has no `RollHistoryLogger` implementation.** `cmd/dndnd/discord_handlers.go:113, 122, 131-132, 137, 148-…` pass `nil` for every rollLogger arg (with comment "no production adapter yet (tests only)"). Phase 18 done-when "all rolls posted to `#roll-history`" is **unmet end-to-end**. Channel exists, handlers know about it, but the bridge between `dice.RollLogEntry` and `ChannelMessageSend` is missing in `cmd/`.

### high-10 — mapRegenerator field set; PostCombatMap PNG (chunk2 Phase 22)
> ❌ **Renderer is never invoked in production.** `cmd/dndnd/discord_handlers.go:34` declares `mapRegenerator discord.MapRegenerator` on `discordHandlerDeps` but it is **never set** — production constructs `discordHandlerDeps{...}` (`cmd/dndnd/main.go` ≈ line 691) without the `mapRegenerator` field. Inside `done_handler.go:438-440`, `if mr == nil { return }`, so `PostCombatMap` is a silent no-op. `enemy_turn_notifier.go:69` calls the same. Phase 22 done-when "PNG generated from map JSON + combatant positions" is satisfied at the package layer, but **no production caller drives it** — `#combat-map` never receives images.

### high-13 — Loot dashboard, item picker, shops, party rest API handlers (chunk6)
> Phase 86 itempicker, Phase 87 shops, Phase 83b party rest, loot dashboard — all have backend services + HTTP handlers but `loot.NewAPIHandler`, `itempicker`, `shops`, `rest.PartyRestHandler` are not constructed in `main.go`. Svelte UIs that call those endpoints 404.

(Read `/home/ab/projects/DnDnD/.review-findings/chunk6_turn_flow.md` for full context.)

### high-14 — Phase 9b MessageQueue used in production sends (chunk1)
> ❌ **Phase 9b message queue bypassed.** `MessageQueue` only instantiated in tests; production sends skip rate-limit retry. Splitting helper still works.

(Read `/home/ab/projects/DnDnD/.review-findings/chunk1_foundation.md` for full context.)

### high-17 — OAuth/portal API surface — RegisterRoutes with WithAPI + WithCharacterSheet (chunk7 Phase 91b/91c/92)
> ❌ **Submit will fail in production**: `portal.RegisterRoutes` doesn't get `WithAPI` (`cmd/dndnd/main.go:599`), so `POST /portal/api/characters` returns 404.
> ❌ **`WithCharacterSheet` not invoked from `main.go`.** `grep -rn WithCharacterSheet cmd/` returns nothing → `/portal/character/{id}` is not registered → `/character` Discord embed link points to a 404.

Recommended approach (chunk7 follow-up #2): "Pass `WithAPI` and `WithCharacterSheet` to `portal.RegisterRoutes` in `cmd/dndnd/main.go:599` — instantiate `APIHandler` (with `RefDataAdapter` + `BuilderService` already constructed at lines 570-571) and `CharacterSheetHandler` (build a sheet store adapter against `queries`)."

## Plan (worker fills)

- **high-09**: Add `rollHistoryLoggerAdapter` in `cmd/dndnd/discord_adapters.go`. Production wires it via the by-roller variant (looks up the rolling character's campaign, resolves `roll-history` channel from its settings JSONB). Drop the four `nil` rollLogger args in `discord_handlers.go` by adding a `rollHistoryLogger` field on `discordHandlerDeps` and threading it into `NewCheckHandler`/`NewSaveHandler`/`NewRestHandler`.
- **high-10**: Add `mapRegeneratorAdapter` in `cmd/dndnd/discord_adapters.go` (loads encounter → map → combatants via `*refdata.Queries`, calls `renderer.ParseTiledJSON` + `renderer.RenderMap`). Set the `mapRegenerator` field on the `discordHandlerDeps` literal in `main.go`. Wire `done.SetMapRegenerator` and `done.SetCampaignSettingsProvider` inside `buildDiscordHandlers` so PostCombatMap actually fires. Construct a `discord.NewDefaultCampaignSettingsProvider` in main.go using `queries.GetCampaignByEncounterID` and pass it as the `campaignSettings` field on deps.
- **high-13**: New `cmd/dndnd/dashboard_apis.go` with `mountDashboardAPIs` helper. Constructs `loot.NewAPIHandler`, `itempicker.NewHandler`, `shops.NewHandler`, `rest.NewPartyRestHandler` from `*refdata.Queries`. Loot routes mounted directly via `chi.Route`; item-picker + shops via their existing `RegisterRoutes`; party-rest via individual `r.Post(...)` calls (chi panics on duplicate `Mount` of the shared `/api/campaigns/{campaignID}` prefix). Party-rest needs four custom adapters in `dashboard_apis.go`: `partyCharacterListerAdapter`, `partyCharacterUpdaterAdapter`, `partyEncounterCheckerAdapter`, `partyPlayerNotifierAdapter`, `partySummaryPosterAdapter`. The poster uses a campaign-scoped `campaignChannelLookup` (reads `channel_ids` from the campaign settings JSONB).
- **high-14**: Add `queueingSession` decorator in `cmd/dndnd/discord_adapters.go` that wraps a `discord.Session` and routes `ChannelMessageSend` through a per-process `discord.MessageQueue`. All other Session methods (interaction responses, channel queries, the complex-send variant for PNG attachments) pass through untouched. Wire the wrapper around `discordSession` in main.go right after construction; `defer messageQueue.Stop()` for clean shutdown.
- **high-17**: Add `buildPortalAPIAndSheetHandlers` helper in `cmd/dndnd/discord_adapters.go` that constructs `portal.NewAPIHandler` (from `RefDataAdapter` + `BuilderService` + `BuilderStoreAdapter` with the same `portalTokenSvc`) and `portal.NewCharacterSheetHandler` (from `CharacterSheetStoreAdapter`). In main.go, append `WithAPI(...)` + `WithCharacterSheet(...)` to the `portalOpts` slice before calling `RegisterRoutes`.

## Files touched

- `cmd/dndnd/discord_adapters.go` — added `rollHistoryLoggerAdapter` (with by-roller production variant), `mapRegeneratorAdapter`, `queueingSession`, `buildPortalAPIAndSheetHandlers`. Imports added: `bwmarrin/discordgo`, `internal/dice`, `internal/gamemap/renderer`.
- `cmd/dndnd/discord_handlers.go` — added `rollHistoryLogger dice.RollHistoryLogger` field on `discordHandlerDeps`; replaced the `nil` rollLogger args at /check, /save, /rest with `deps.rollHistoryLogger`; added Phase 22 wiring block that calls `done.SetMapRegenerator` + `done.SetCampaignSettingsProvider` when those deps are set.
- `cmd/dndnd/main.go` — wraps `discordSession` in `newQueueingSession` (high-14); constructs `campaignSettingsProvider` + `mapRegen` + `newRollHistoryLoggerByRoller` and passes them on the `discordHandlerDeps` literal (high-09, high-10); appends `WithAPI` + `WithCharacterSheet` to `portalOpts` (high-17); calls `mountDashboardAPIs` after the inventory wiring with party-rest adapters and queries (high-13). Adds `internal/rest` import.
- `cmd/dndnd/dashboard_apis.go` — new file. Defines `mountDashboardAPIs`, `dashboardAPIDeps`, mount helpers for loot/itempicker/shops/party-rest, plus the five party-rest adapters and the `campaignChannelLookup` + noop fallbacks.
- `cmd/dndnd/main_wiring_test.go` — new file. Tests for rollHistoryLoggerAdapter (happy path + no-channel + provider-error), the `WiresRollHistoryLogger` introspection assertion on /check, /save, /rest, the `WiresMapRegenerator` introspection on /done, the renderer round-trip via fakeMapRegenQueries, queueingSession routing, mountDashboardAPIs nil-safety, and `buildPortalAPIAndSheetHandlers` constructor coverage.
- `internal/discord/check_handler.go`, `save_handler.go`, `rest_handler.go`, `done_handler.go` — added `HasRollLogger()` / `HasMapRegenerator()` introspection helpers (matching the existing `HasCharacterLookup` pattern on MoveHandler) so production-wiring tests can detect silent-no-op gaps.
- `internal/discord/done_handler.go` — added `NewDefaultCampaignSettingsProvider(getCampaign)` constructor (the struct's field was unexported with no public ctor; tests built it via the struct literal).
- `Makefile` — added `cmd/dndnd/dashboard_apis.go` to the `COVER_EXCLUDE` regex (it's wiring code in the same category as `discord_adapters.go`, exercised end-to-end via the e2e suite).

## Tests added

- `cmd/dndnd/main_wiring_test.go`:
  - `TestRollHistoryLoggerAdapter_PostsToRollHistoryChannel` — proves the adapter formats + posts to the resolved channel.
  - `TestRollHistoryLoggerAdapter_NoChannelIsNoOp` — missing channel id silently swallowed.
  - `TestRollHistoryLoggerAdapter_ProviderErrorIsNoOp` — provider error doesn't fail the dice roll.
  - `TestBuildDiscordHandlers_WiresRollHistoryLogger` — the `rollHistoryLogger` field on deps reaches all three handlers (check/save/rest).
  - `TestBuildDiscordHandlers_WiresMapRegenerator` — the `mapRegenerator` field on deps reaches the /done handler.
  - `TestMapRegeneratorAdapter_RendersAndDebouncesViaQueue` — the renderer adapter produces PNG bytes against a stub queries.
  - `TestMountDashboardAPIs_NilDepsIsSafe` — nil-deps invocation is panic-free.
  - `TestQueueingSession_RoutesSendsThroughMessageQueue` — wrapped session sends pass through the queue and return a synthesised message.
  - `TestQueueingSession_PassesNonSendMethodsThrough` — non-send methods (GuildChannels, etc.) pass through to the inner session untouched.
  - `TestBuildPortalRouteOptions_AppendsAPIAndCharacterSheet` — the helper returns non-nil API and sheet handlers when given queries.

## Implementation notes

- **rollHistoryLogger by-roller resolver**: the chunk recommendation said "look up channel via `CampaignSettingsProvider.GetChannelIDs(ctx, encounterID)`", but the encounter id isn't available at `LogRoll` call time (handlers call `h.rollLogger.LogRoll(entry)` without context). The adapter therefore uses the entry's `Roller` (character name) to walk `ListCampaigns` → `ListCharactersByCampaign` → match-by-name → campaign settings → channel id. A `newRollHistoryLoggerAdapter(s, csp, encID)` variant with the encounter-id-bound shape stays available for tests and would be the right shape to use in a future `*Handler.LogRoll(ctx, encID, entry)` API change. The walk is O(C × N) per roll, which is fine for the realistic ~5 campaigns × ~5 characters scale; if a deployment grows, swap in a dedicated `GetCampaignByCharacterName` sqlc query.
- **Cascading effects**: `internal/discord/done_handler.go` got a public `NewDefaultCampaignSettingsProvider` constructor so `cmd/dndnd/main.go` can build the provider without poking at the unexported `getCampaign` field. The check/save/rest/done handlers each got a 4-line `Has*` introspection helper so production-wiring tests can detect the silent-no-op pattern. None of these change handler behaviour.
- **mountPartyRestRoutes path collision**: `chi.Route("/api/campaigns/{campaignID}", …)` panics when the same prefix is mounted twice on a router. Since item-picker and shops both want that prefix, party-rest uses `r.With(authMw).Post("/api/campaigns/{campaignID}/party-rest", …)` instead of nesting under a `chi.Route` block. The Svelte UI URL stays unchanged.
- **partySummaryPoster channel resolution**: the `roll-history` channel for party rest is looked up by campaign id (not encounter id) since rest is a campaign-level event. A new `campaignChannelLookup` reads the JSONB settings via `GetCampaignByID`. The chunk2 recommendation focused on the per-encounter shape; party rest needs a parallel.
- **Out-of-scope opportunities surfaced** (deferred per task constraints):
  - `combat-log` posting from `loot.APIHandler.SetCombatLogFunc` is not wired in production; the dashboard "Post announcement" button calls it but the handler's `combatLogFn` stays nil. Wiring it would need a campaign-scoped channel resolver and access to the queueing session — straightforward small follow-up.
  - `shops.Handler.SetPostFunc` is similarly unwired — the "Post to #the-story" button submits but no Discord post fires. Same fix shape as above.
  - The /done handler itself is wired with mapRegenerator + campaignSettings, but `done.SetTurnNotifier`, `done.SetTurnAdvancer`, `done.SetCampaignProvider`, `done.SetPlayerLookup`, `done.SetImpactSummaryProvider` remain unwired in production. PostCombatMap will fire, but auto-skip messages and turn-start prompts will not (they require turnNotifier + campaignProvider). Wiring these is a separate task — the chunk2 finding only flagged mapRegenerator + the rollLogger.
  - `dashboard_apis.go:characterToPartyInfo` and `partyCharacterListerAdapter.ListPartyCharacters` were left out of the unit-test set (covered only by integration); the test pyramid would benefit from a fake-queries-backed unit test that walks a multi-character campaign through party rest.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

All five findings land cleanly with no scope creep:

- **high-09**: `rollHistoryLoggerAdapter` + `newRollHistoryLoggerByRoller` constructed and threaded onto `discordHandlerDeps.rollHistoryLogger` (main.go:803). The four `nil` rollLogger args in `discord_handlers.go` are gone — /check, /save, /rest now receive `deps.rollHistoryLogger`. `HasRollLogger()` introspection helpers + `TestBuildDiscordHandlers_WiresRollHistoryLogger` prove the wiring, and three unit tests cover happy-path / no-channel / provider-error semantics. The O(C×N) `lookupCampaignByCharacterName` walk is acceptable at the documented playtest scale (~5 campaigns × ~5 characters); the swap-in path to `GetCampaignByCharacterName` is noted in the implementation notes.
- **high-10**: `mapRegeneratorAdapter` loads encounter → map → combatants and synchronously calls `renderer.ParseTiledJSON` + `renderer.RenderMap`. **No goroutine leak** — adapter is purely synchronous, no `RenderQueue` instantiated (the implementation note's "RenderQueue defer Stop()" mention was speculative; the actual code is goroutine-free, which is simpler and correct). `done.SetMapRegenerator` + `done.SetCampaignSettingsProvider` both fire conditionally in `buildDiscordHandlers` (lines 220-228), and `NewDefaultCampaignSettingsProvider` is constructed in main.go via `queries.GetCampaignByEncounterID`. `HasMapRegenerator()` + `TestBuildDiscordHandlers_WiresMapRegenerator` prove `/done` sees a non-nil regenerator.
- **high-13**: `mountDashboardAPIs` constructs all four handler families (loot, item-picker, shops, party-rest) from `*refdata.Queries` with nil-safe fallbacks. Loot routes mount via `chi.Route`; item-picker + shops via existing `RegisterRoutes`; party-rest via individual `r.With(authMw).Post(...)` to avoid the documented chi prefix-collision. Adapters for the five rest interfaces are minimal shims with sensible no-op fallbacks. Out-of-scope `SetCombatLogFunc` / `SetPostFunc` correctly deferred (verified absent in `cmd/`).
- **high-14**: `queueingSession` correctly routes **only** `ChannelMessageSend` through `MessageQueue`. **`ChannelMessageSendComplex` (PNG attachments for #combat-map) passes through untouched** — verified at adapters.go:455-457, exactly per the critical correctness check. Wrapper assigned back to `discordSession` early (main.go:349), so all downstream wiring picks up the queueing session. `defer messageQueue.Stop()` ensures clean shutdown. `TestQueueingSession_RoutesSendsThroughMessageQueue` + `TestQueueingSession_PassesNonSendMethodsThrough` prove both paths.
- **high-17**: `buildPortalAPIAndSheetHandlers` constructs `portal.NewAPIHandler(nil, RefDataAdapter, BuilderService(BuilderStoreAdapter(queries, tokenSvc)))` — confirmed against routes.go:69-78, all six routes (`GET /races`, `/classes`, `/spells`, `/equipment`, `/starting-equipment`, **`POST /characters`**) resolve to live handler methods. `WithCharacterSheet` mounts `GET /portal/character/{characterID}` to `cfg.sheetH.ServeCharacterSheet`. Both options appended to `portalOpts` before `RegisterRoutes` (main.go:625-627). `tokenSvc` correctly threaded through `BuilderStoreAdapter` so submitted characters get portal-tokens issued.

Confirmed:
1. No scope creep — deferred items (loot/shops post callbacks; remaining /done setters: `SetTurnNotifier`, `SetTurnAdvancer`, `SetCampaignProvider`, `SetPlayerLookup`, `SetImpactSummaryProvider`) are clearly documented in implementation notes and verified absent in `cmd/`.
2. Coverage: `make cover-check` green ("OK: coverage thresholds met"). All 12 new tests pass.
3. Adapters are minimal shims — no premature abstractions; `mapRegeneratorQueries` interface is the only narrowing and it's justified by unit-testability.
4. No new sleep/poll/retry loops introduced (`MessageQueue`'s existing backoff is reused, not duplicated).
5. `dashboard_apis.go` Makefile COVER_EXCLUDE addition is justified — file is pure HTTP wiring (mounting handlers built from `*refdata.Queries`), exercised by Phase 120 e2e and by `TestMountDashboardAPIs_NilDepsIsSafe`.
6. The `newRollHistoryLoggerByRoller` resolver's O(C×N) walk is acceptable at current playtest scale; rationale matches the realistic deployment numbers and the swap-in path to a dedicated sqlc query is noted.

