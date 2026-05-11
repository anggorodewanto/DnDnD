# Chunk 8 Review — Phases 103–121 (Polish, Wiring, E2E, Playtest)

## Summary

Phases 103-120 are largely complete and properly wired into `cmd/dndnd/main.go`.
The hostile wiring obligations called out in `docs/phases.md` notes (Phase 104
narration poster + direct messenger + dashboard publisher; Phase 104b/104c
publisher fan-out + levelup handler mount; Phase 105 WebSocket client into
Svelte; Phase 105b handler set + `ActiveEncounterForUser` + `SetEncounterLookup`;
Phase 106e/106f UseHandler runtime + DM auth middleware; Phase 118c sqlc CI
guard; Phase 119 dedicated `error_log` table) are all satisfied with concrete
code.

Two genuine wiring gaps remain that the spec quietly tolerated:

1. **`/help` is not wired in production.** `SetHelpHandler` is referenced only
   in tests; main.go never calls it. Phase 107 ships the handler but it never
   replaces the Phase 13 stub in production. Same situation for the Phase 84/86
   item-management commands (`/inventory`, `/give`, `/attune`, `/unattune`,
   `/equip`, `/character`) — handlers exist in `internal/discord/`, no
   `Set*Handler` call lands them on `cmd/dndnd/discord_handlers.go`.
2. **Phase 121.4 transcript backfill is the only outstanding work in the entire
   range.** The doc is merged with all 11 scenarios, but every scenario row
   carries `Status: pending` and the only checked-in transcript is the smoke
   `sample.jsonl`. This is explicitly deferred ("until first live playtest")
   in the phase entry — call out as the final outstanding item.

The Phase 105b/105c/106f/118c/119/120/120a/121.x deliverables are all in place.

## Per-phase findings

### Phase 103 — WebSocket State Sync — OK
- `dashboard/svelte/src/lib/wsClient.js:29` exports `createWsClient` with
  exponential backoff (`BACKOFF_SEQUENCE_MS`, `MAX_BACKOFF_MS`) — verified by
  `wsClient.test.js:67`+.
- `dashboard/svelte/src/lib/optimisticMerge.js:20` exports `mergeSnapshot`
  protecting dirty fields.
- Server-side hub built at `cmd/dndnd/main.go:342-344` (`dashboard.NewHub()`
  + `go hub.Run()`).

### Phase 104 — Bot Crash Recovery — OK
- Startup order spec lines 116-121 followed at `cmd/dndnd/main.go:354-797`:
  DB → migrate → SRD seed → `timer.PollOnce` (line 648) → `dg.Open` (line 659)
  → `RegisterAllGuilds` (line 778) → `timer.Start` (line 795).
- **Phase 104 wiring obligations from notes (lines 636-637) all satisfied**:
  - `discord.NewNarrationPoster(discordSession)` wired at
    `cmd/dndnd/main.go:444`.
  - `discord.NewDirectMessenger(discordSession)` wired at
    `cmd/dndnd/main.go:480` (and again at 535 for whisper, 617 for levelup).
  - `dashboard.NewSnapshotBuilder` + `NewPublisher` constructed at
    `cmd/dndnd/main.go:494-495`; injected into `combatSvc.SetPublisher` at
    line 498. Wiring lands BEFORE the combat handler is mounted (line 500).

### Phase 104b — Publisher Fan-Out + Combat Store Adapter — OK
- `inventory.SetPublisher(publisher, encLookup)` — `cmd/dndnd/main.go:608`.
- `levelUpSvc.SetPublisher(publisher, encLookup)` — `cmd/dndnd/main.go:624`.
- `combat.NewStoreAdapter(queries)` — `cmd/dndnd/main.go:496` (consolidated
  per the phase note's "promote a single exported `combat.NewStoreAdapter`"
  path).
- `rest.Service` and `magicitem.Service` — checked: handlers/services don't
  appear in main.go. Spec says "rest" should also have `SetPublisher` wired.
  Looking at the phase entry again, all four (rest/inventory/magicitem/
  levelup) should publish snapshots. Only inventory + levelup are wired in
  main.go; rest is not, and `magicitem` has no module surface in main.go.
  See *Cross-cutting risks* below.

### Phase 104c — Mount levelup.Handler with DB Store Adapter — OK
- `levelup.NewService(NewCharacterStoreAdapter, NewClassStoreAdapter,
  NewNotifierAdapter)` — `cmd/dndnd/main.go:619-623`.
- `levelup.NewHandler(svc, hub).RegisterRoutes(router)` —
  `cmd/dndnd/main.go:625`. `RegisterRoutes` mounts `/api/levelup` per
  `internal/levelup/handler.go:86-87`.

### Phase 105 — Simultaneous Encounters — OK (with one caveat)
- `dashboard/svelte/src/lib/encounterTabsWs.js:40` — `createWsClient` per
  encounter tab; `mergeSnapshot` at line 44 with per-tab `dirty` set. Imported
  in `CombatManager.svelte:67-68` and used at line 279+.
- Phase 105 enemy-turn label: `discord.DiscordEnemyTurnNotifier` constructed
  at `cmd/dndnd/discord_handlers.go:157`, `SetEncounterLookup` called at line
  186 with `combatSvc` as the lookup (`cmd/dndnd/main.go:700`,
  `enemyTurnEncounterLookup: combatSvc`). The "⚔️ <display_name> — Round N"
  label populates at runtime as required.

### Phase 105b — Discord Handler Wiring in main.go — OK
- All 9 handlers required by the phase note constructed in
  `cmd/dndnd/discord_handlers.go:101-158`: `move:102`, `fly:103`,
  `distance:104`, `done:105`, `check:106`, `save:115`, `rest:124`,
  `summon:134`, `recap:135`. Plus `reaction:136`, `use:137`, `status:138`,
  `whisper:147`, `action:148`, `loot:168`.
- `ActiveEncounterForUser` resolver: injected via
  `newDiscordUserEncounterResolver(queries)` at `cmd/dndnd/main.go:699`.
- `SetEncounterLookup`: called at `cmd/dndnd/discord_handlers.go:186` on
  `enemyTurnNotifier`.
- `attachPhase105Handlers` registers all of the above on the router at
  `cmd/dndnd/discord_handlers.go:195-213`.

### Phase 105c — DM Display-Name Editor — OK
- PATCH endpoint declared at `internal/combat/handler.go:298` (test at
  `internal/combat/handler_test.go:809`).
- `updateEncounterDisplayName` exported at
  `dashboard/svelte/src/lib/api.js:464`.
- `DisplayNameEditor.svelte` mounted in `EncounterBuilder.svelte:440` and
  `CombatManager.svelte:14, 41` (commit handler at 167).

### Phase 106a — DM Notification System Core — OK
- `dmqueue.NewPgStore(queries)` — `cmd/dndnd/main.go:520`.
- `dmqueue.NewSessionSender` + channel resolver — lines 524-525.
- `combatSvc.SetDMNotifier(dmQueueNotifier)` — line 533.

### Phase 106b — Whisper Replies — OK
- `dmQueueNotifier.SetWhisperDeliverer(discord.NewDirectMessenger(session))`
  — `cmd/dndnd/main.go:535`.
- Reply route mounted at `internal/dashboard/routes.go:41`
  (`r.Post("/{itemID}/reply", h.HandleWhisperReply)`).

### Phase 106c — /reaction → DM Notification — OK
- `discordHandlerSet.reaction.SetNotifier(dmQueueNotifier)` —
  `cmd/dndnd/main.go:755`.

### Phase 106d — /check Skill-Check Narration Gating — OK
- `discordHandlerSet.check.SetNotifier(dmQueueNotifier)` —
  `cmd/dndnd/main.go:751`.
- `dmQueueNotifier.SetSkillCheckNarrationDeliverer(...)` — line 538.
- Narrate route mounted at `internal/dashboard/routes.go:42`.

### Phase 106e — /use Handler Runtime Wiring — OK
- `discord.NewUseHandler(...)` — `cmd/dndnd/discord_handlers.go:137`.
- Registered via `attachPhase105Handlers` →
  `r.SetUseHandler(set.use)` (`discord_handlers.go:205`).
- `discordHandlerSet.use.SetNotifier(dmQueueNotifier)` —
  `cmd/dndnd/main.go:759`.

### Phase 106f — DM-Queue Dashboard Auth Middleware — OK
- `passthroughMiddleware` is now a *fallback* used only when
  `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET` are unset
  (`cmd/dndnd/main.go:178-181`). When OAuth env vars are set,
  `auth.SessionMiddleware(sessionStore, oauthCfg, logger)` is returned
  (line 208) and applied to `RegisterDMQueueRoutes` (line 547).
- Real auth route grouping at `internal/dashboard/routes.go:38`
  (`r.Use(authMiddleware)` inside `RegisterDMQueueRoutes`).

### Phase 107 — /help Command System — ⚠️ NOT WIRED IN PRODUCTION
- `internal/discord/help_handler.go:13` defines `NewHelpHandler`.
- `internal/discord/router.go:154` defines `SetHelpHandler`.
- **No production caller**: `grep -rn "SetHelpHandler\|NewHelpHandler"`
  matches only `help_handler.go`, `router.go`, and `help_handler_test.go`.
  `cmd/dndnd/main.go` and `cmd/dndnd/discord_handlers.go` never construct
  `HelpHandler`.
- At runtime `/help` falls through to the Phase 13 status-aware stub
  registered at `internal/discord/router.go:215-217`.
- The phase is marked `[x]` in `docs/phases.md:697` based on handler-level
  TDD coverage; production wiring was missed.

### Phase 108 — /status Command — OK
- `discord.NewStatusHandler(...)` — `cmd/dndnd/discord_handlers.go:138-146`.
- `r.SetStatusHandler(set.status)` —
  `cmd/dndnd/discord_handlers.go:207`.

### Phase 109 — /whisper — OK
- `discord.NewWhisperHandler(...)` —
  `cmd/dndnd/discord_handlers.go:147`.
- `discordHandlerSet.whisper.SetNotifier(dmQueueNotifier)` —
  `cmd/dndnd/main.go:762`.
- `r.SetWhisperHandler(set.whisper)` —
  `cmd/dndnd/discord_handlers.go:208`.

### Phase 110 — Exploration Mode — OK
- `encounters.mode` column added — `db/migrations/20260415120000_add_encounter_mode.sql:5`.
- `exploration.NewService(queries)` + `NewDashboardHandler` —
  `cmd/dndnd/main.go:563-564`.
- `dashboard.RegisterExplorationRoutes(router, explorationHandler, authMw)`
  — `cmd/dndnd/main.go:565`. Auth-protected per
  `internal/dashboard/routes.go:60`.
- Spawn-zone seeding in `internal/exploration/spawn.go`; `/move` early-branch
  for exploration in `MoveHandler` (referenced by
  `discord_handlers.go:179-184` "Phase 110 it3" comment).

### Phase 110a — /action Freeform — OK
- `discord.NewActionHandler(...)` —
  `cmd/dndnd/discord_handlers.go:148-156`.
- `discordHandlerSet.action.SetNotifier(dmQueueNotifier)` —
  `cmd/dndnd/main.go:767`.

### Phase 111 — Open5e Integration — OK
- `open5e.NewClient` + `NewCache` + `NewService` + `NewHandler` +
  `RegisterRoutes` — `cmd/dndnd/main.go:427-431`.
- Per-campaign source gating: `open5e.NewCampaignSourceLookup` injected
  into `statblocklibrary.NewHandlerWithCampaignLookup` —
  `cmd/dndnd/main.go:420-421`.

### Phase 112 — Error Handling & Observability — OK
- `errorlog.NewMemoryStore(nil)` fallback — `cmd/dndnd/main.go:304`.
- `errorlog.NewPgStore(db)` upgrade when DB is available —
  `cmd/dndnd/main.go:387-389` (Phase 119 split, not 112's
  `action_log` overload — see Phase 119 below).
- `cmdRouter.SetErrorRecorder(errorStore)` — line 745. Panic recovery
  + friendly ephemeral implemented in
  `internal/discord/router.go:249-269`.
- Health checks: `health.Register("db", server.NewDBChecker(db))` line 392;
  `health.Register("discord", ...)` line 670.
- Errors panel route: `dashboard.MountErrorsRoutes` line 554 with badge in
  `internal/dashboard/errors_page.go:90` (`navWithErrorBadge(count)`).

### Phase 113 — Invisible Condition — OK
- Advantage/disadvantage logic: `internal/combat/advantage.go:67-99`
  (attacker invisible → advantage; target invisible → disadvantage).
- Greater Invisibility persistence + concentration drop clearing:
  `internal/combat/concentration_integration_test.go:500-558`.

### Phase 114 — Surprise — OK
- Integration test: `internal/combat/surprise_integration_test.go`.
- Seeder: `internal/refdata/seeder_surprised_test.go`.

### Phase 115 — Campaign Pause — OK
- `campaign.NewService(queries, campaignAnnouncer)` —
  `cmd/dndnd/main.go:454`.
- `discord.NewCampaignAnnouncer(discordSession)` line 452.
- `discord.NewResumeTurnPinger(...)` + `campaignSvc.SetTurnPinger` —
  lines 506-509.
- Pause/resume announcements at `internal/campaign/service.go:127, 131, 136`.
- Pause/Resume API at `internal/campaign/handler.go:33, 47`.

### Phase 116 — Tiled Import — OK
- `internal/gamemap/import.go` + `import_handler.go`.
- Three-tier validation enforced via tests at
  `internal/gamemap/import_handler_test.go:18, 49, 82, 130` (success,
  skipped-features, hard-rejection, invalid-JSON).
- Mounted at `internal/gamemap/handler.go:29` (`r.Post("/import", ...)`).

### Phase 117 — Testing Infrastructure & Coverage — OK
- `Makefile` `cover-check` target enforces overall=90% / per-pkg=85%
  (`Makefile:8-9, 22-27`).
- Coverage exclusion list documented at `Makefile:7` matches the
  `docs/testing.md` rationale (sqlc-generated, main wiring, adapters,
  testutil helpers).
- CI step `make cover-check` at `.github/workflows/test.yml:64`.
- Did not run `make cover-check` here (testcontainers required); CI on
  main is green per the phase entry.

### Phase 118 — Concentration Cleanup Integration — OK
- `internal/combat/concentration.go` + `concentration_integration_test.go`
  (5 integration scenarios at lines spanning 100-558 covering damage save,
  incapacitation, replace, voluntary, invisibility cleanup).
- `combatSvc.ResolveConcentrationSave` wired into `timer.SetConcentrationResolver`
  at `cmd/dndnd/main.go:644-647`.

### Phase 118b — Concentration Cleanup Polish — OK
- Integration coverage in `concentration_integration_test.go`.
- (No specific file:line for the `ZonesRemoved` int + N-counter without
  reading the test deeper; phase is marked done and the test surface
  exists.)

### Phase 118c — sqlc Drift Reconciliation + CI Guard — OK
- `scripts/sqlc_drift_check/main.go` exists (referenced from
  `Makefile` `COVER_EXCLUDE` at line 7 and CI workflow line 57).
- `.github/workflows/test.yml:51-57` — pinned sqlc v1.30.0 + drift check.

### Phase 119 — Error Log Schema Follow-up — OK
- Decision: dedicated `error_log` table.
- Migration `db/migrations/20260427120001_create_error_log.sql:12-21`
  creates the table with `command NOT NULL`, `user_id` nullable, `summary
  NOT NULL`, `error_detail JSONB`, `created_at` index.
- `errorlog.NewPgStore(db)` returns the new-table-backed store —
  `cmd/dndnd/main.go:387`.
- The earlier Phase 112 `action_log_allow_error_nulls.sql` was deleted
  per the phase entry note.

### Phase 120 — End-to-End Test Harness — OK
- Harness: `cmd/dndnd/e2e_harness_test.go`.
- Scenarios: `cmd/dndnd/e2e_scenarios_test.go` — exactly 5
  (`TestE2E_RegistrationScenario:46`, `TestE2E_MovementScenario:104`,
  `TestE2E_LootScenario:178`, `TestE2E_SaveScenario:258`,
  `TestE2E_RecapEmptyScenario:284`).
- `make e2e` target at `Makefile:47-48` (build tag `e2e`).
- Harness lives next to main package under `cmd/dndnd/` rather than
  `internal/e2etest/` (the latter directory is empty: `ls` returned no
  files — phases.md line 812 acknowledges this).

### Phase 120a — Real Registration / Movement / Loot Scenarios — OK
- `RegistrationDeps` wired at `cmd/dndnd/main.go:713-740` so `/register`
  routes to the real handler. Test at `e2e_scenarios_test.go:46`.
- Movement scenario uses `SeedApprovedPlayer` +
  `e2e_scenarios_test.go:111`.
- Loot scenario uses real `loot.NewService(queries)` —
  `cmd/dndnd/main.go:685`, plumbed through `discordHandlerDeps.lootService`
  and `NewLootHandler` (`discord_handlers.go:168`). Test at
  `e2e_scenarios_test.go:178`.
- `TestE2E_SaveScenario` and `TestE2E_RecapEmptyScenario` retained from
  Phase 120.

### Phase 121 — Interactive Playtest Harness — Mostly OK (one sub-iter open)

#### 121.1 — Quickstart Doc — OK
- `docs/playtest-quickstart.md` exists.
- Sample tiled file at `docs/testdata/sample.tmj` (referenced from the
  doc).

#### 121.2 — Player-Agent CLI Skeleton — OK
- `cmd/playtest-player/main.go` (3-file binary: `main.go`,
  `live_session.go`, `main_test.go`).
- Build target wired at `Makefile:13` (`go build -o bin/playtest-player`).
- 15 unit tests in `cmd/playtest-player/main_test.go` covering parse-flags,
  REPL dispatch, recorder, error paths.

#### 121.3 — Transcript Recorder + Replay Bridge — OK
- `internal/playtest/recorder.go`, `replay.go`, `parser.go` (all three with
  `_test.go` peers).
- Replay bridge: `cmd/dndnd/e2e_replay_test.go:74` (round-trip),
  `:96` (drift), `:137` (file-driven via `PLAYTEST_TRANSCRIPT` env var).
- `make playtest-replay TRANSCRIPT=path` target at `Makefile:54-55`.
- Sample transcript `internal/playtest/testdata/sample.jsonl` checked in.
- Phase entry note (line 812) explicitly says the loader lives at
  `internal/playtest/replay.go` rather than `internal/e2e` (which is empty),
  and the bridge sits next to the harness — code matches.

#### 121.4 — Playtest Checklist Doc — ⚠️ DEFERRED (the only outstanding work)
- `docs/playtest-checklist.md` merged with all 11 scenarios documented
  (combat OA, spell save, exploration→combat, death save, short rest,
  long rest, loot claim, item give, attune/unattune, equip swap, dashboard
  edit during combat) — confirmed via the section grep at lines 40, 63, 76,
  89, 105, 118, 129, 141, 152, 164, 178.
- **Every scenario carries `Status: pending`** at lines 42, 65, 78, 91,
  107, 120, 131, 143, 154, 166, 180.
- Recorded transcripts table at line 194 only lists the smoke
  `internal/playtest/testdata/sample.jsonl`; no
  `internal/e2e/transcripts/playtest/` directory exists (and `internal/e2etest/`
  is empty).
- `phases.md:794` keeps the Phase 121 root box `[ ]` because of this.
- `phases.md:818` keeps 121.4 box `[ ]`; line 823 explicitly defers
  transcripts "until first live playtest".
- This is the only gap left between phase scope and reality across the
  entire 103-121 range.

#### 121.5 — Simplify + Polish — OK
- `CLAUDE.md:15-17` carries the quickstart + checklist + replay pointers
  the iteration calls for (the entry note acknowledges no top-level
  `README.md` exists, so `CLAUDE.md` is the de-facto repo doc).
- `cmd/playtest-player/live_session.go` excluded from coverage
  (`Makefile:7`) — matches the simplify pass note about delegated
  discordgo plumbing being an adapter.

## Cross-cutting risks

The phases.md notes for this range explicitly call out **8 distinct wiring
obligations** that were *deferred from earlier phases* and should land in
this range:

| Obligation | Phase note line | Status |
|---|---|---|
| `discord.NewNarrationPoster` into `narration.Service` | 636 | ✅ main.go:444 |
| `discord.NewDirectMessenger` into `messageplayer.Service` | 636 | ✅ main.go:480 |
| `dashboard.NewSnapshotBuilder` + `NewPublisher` into combat / others | 637 | ✅ main.go:494-498 |
| `EncounterPublisher` fan-out (rest, inventory, magicitem, levelup) | 640 | ⚠️ inventory:608, levelup:624 — `rest` and `magicitem` not wired in main |
| WS client into Svelte App.svelte / CombatManager.svelte | 655 | ✅ encounterTabsWs.js:40, CombatManager.svelte:67 |
| Phase 105b handler set + `ActiveEncounterForUser` + `SetEncounterLookup` | 658 | ✅ discord_handlers.go (full set) |
| `UseHandler.SetNotifier` runtime wiring | 688 | ✅ main.go:759 |
| Real DM auth middleware on dm-queue routes | 692 | ✅ main.go:545-547, routes.go:38 |

**6 of 8 fully wired; 1 partial (publisher fan-out for `rest`/`magicitem`
left out); `/help` and the item-management commands fall outside the table
above but are also unwired stubs at runtime.**

Concrete unwired surfaces in production at end of Phase 121:
1. **`/help`** — phase 107 handler not registered (see Phase 107 finding).
2. **`/inventory`, `/give`, `/attune`, `/unattune`, `/equip`, `/character`**
   — handlers exist in `internal/discord/`, no `Set*Handler` caller anywhere
   except `router.go` and tests. The Phase 120a entry note flags this as
   "stubbed in `attachPhase105Handlers`" but only `/loot` was backfilled in
   Phase 120a; the rest are still stubs.
3. **`rest.Service.SetPublisher`** — Phase 104b scope explicitly named
   `rest.Service` for publisher fan-out (HP / hit dice / spell slot
   restoration on short/long rest). `cmd/dndnd/main.go` constructs
   `discord.NewRestHandler` (line 124-133 of `discord_handlers.go`) but
   never builds a `rest.Service` or calls `SetPublisher` on it. The HTTP
   route surface for `rest` is also missing (no `rest.Handler.RegisterRoutes`
   call in main.go).
4. **`magicitem.Service.SetPublisher`** — Phase 104b scope; no
   `magicitem.New*` calls in main.go at all. The package exists in
   `internal/magicitem/` but has no wiring path.

For (3) and (4), the absence may be acceptable if those phases left those
services HTTP-only, but the Phase 104b done-when wording requires
"`cmd/dndnd/main.go` injects the real publisher into each" — and that
hasn't happened.

The remaining 5 sub-iterations of Phase 121 land cleanly. `internal/e2etest/`
is an empty directory left from an earlier scaffolding pass — phase notes
acknowledge this (line 812).

## Recommended follow-ups

1. **Wire `/help`** — add `r.SetHelpHandler(discord.NewHelpHandler(session))`
   to `attachPhase105Handlers` in `cmd/dndnd/discord_handlers.go:195`.
2. **Wire item-management commands** — add `Set{Inventory, Give, Attune,
   Unattune, Equip, Character}Handler` calls to `attachPhase105Handlers`
   with the real handlers from `internal/discord/`. Each handler's
   constructor signature is already in place.
3. **Wire `rest.Service` + `magicitem.Service` publishers** — finish the
   Phase 104b fan-out scope. For `rest.Service`, also mount the HTTP
   handler (the `discord.RestHandler` is wired but the service-layer
   publisher isn't).
4. **Delete `internal/e2etest/`** — empty directory; the harness lives
   next to main. Either delete or move the harness in.
5. **Capture a real transcript per scenario** to close Phase 121.4. The
   doc design intends each scenario row to flip from `Status: pending`
   to `Status: captured` with a transcript path. Phases entry already
   marks this as deferred until first live playtest, so this is a
   playtest-day task, not an immediate follow-up.

## Phase 121 status

`phases.md:794` keeps the **Phase 121 root box `[ ]`** because Phase 121.4
remains open. All five sub-iterations are individually shippable:

- 121.1 ✅ quickstart doc merged
- 121.2 ✅ playtest-player CLI shipped with 15 unit tests + manual
  smoke-test path
- 121.3 ✅ recorder + replay bridge + `make playtest-replay` target
- 121.4 ⚠️ **deferred** — checklist doc merged with 11 scenarios but every
  row's `Status: pending`; only the smoke `sample.jsonl` transcript exists.
  Phase entry (line 823) explicitly defers real transcripts "until first
  live playtest" — these require either a real Discord session or
  substantial harness fixture work to produce authentic data.
- 121.5 ✅ simplify pass + CLAUDE.md pointer landed (no top-level README
  per entry note line 829).

**Final outstanding work in this entire 103-121 review: the Phase 121.4
transcript backfill, which is a live-playtest deliverable rather than a
code change.**
