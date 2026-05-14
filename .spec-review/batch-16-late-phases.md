# Batch 16: Late phases (Phases 111–121)

## Summary

All phases 111–120a are implemented in source, with phase 121 in the pending
state but **substantially implemented** (4 of 5 iterations marked done: 121.1
quickstart doc, 121.2 player-agent CLI, 121.3 replay bridge, 121.5 polish;
only 121.4 checklist scenarios are marked unchecked and even that doc is
merged with deferred transcript capture). One material spec gap stands out:
the `/api/campaigns/{id}/pause` and `.../resume` POST endpoints are mounted
on the **bare router**, not behind `dmAuthMw` — so any unauthenticated caller
can pause a live campaign (and post the pause announcement to `#the-story`).
This is the same F-2 / auth concern Batch 01 flagged.

The rest of the batch is tight: Open5e is namespaced cleanly, F-8 mutation
is correctly DM-only, the new dedicated `error_log` table (Phase 119) cleanly
displaces the Phase 112 JSONB overload, invisible advantage/disadvantage rules
match 5e, surprise uses spec-mandated message text, Tiled import implements
the three-tier (hard-reject / supported / skipped) classification, and the
Phase 120 E2E harness runs five spec-named scenarios behind a build tag.

## Per-phase findings

### Phase 111 — Open5e Integration
- Status: **matches**
- Key files: `internal/open5e/{client,cache,service,sources,sources_handler,handler,campaign_lookup,filter}.go`, `cmd/dndnd/main.go:803-811`
- Findings:
  - Per-campaign toggle via `campaigns.settings.open5e_sources` JSONB, read by `CampaignSourceLookup.EnabledOpen5eSources` (F-8 satisfied).
  - Mutation routes `PUT /api/open5e/campaigns/{id}/sources` correctly mounted behind `dmAuthMw`. Catalog `GET /api/open5e/sources` is unauthed but read-only static curated list — fine.
  - Source attribution = `open5e:<document_slug>`; ID namespace prefix `open5e_<slug>` prevents collision with SRD ids (`internal/open5e/cache.go:51`).
  - Offline fallback: `Cache.UpsertCreature/Spell` writes through to refdata so cached rows survive Open5e downtime. Live searches return error to caller — no silent failure.

### Phase 112 — Error Handling & Observability
- Status: **matches**
- Key files: `internal/logging/context.go`, `internal/errorlog/{pgstore,recorder}.go`, `internal/discord/router.go:39-407`, `internal/server/health.go`
- Findings:
  - F-20 `WithContext` helper centralizes user/guild/encounter/command attrs; currently only adopted by `/move` (`internal/discord/move_handler.go:228-238`). Other player-facing handlers still log ad hoc — acceptable per F-20 commit intent, but adoption is incomplete.
  - `CommandRouter.handleCommand` panic recovery records via `errorlog.Recorder` (`router.go:400-407`); non-panic error paths *don't* write to error_log — only panics surface in the DM error panel.
  - Health endpoint with subsystem registry at `internal/server/health.go`.

### Phase 113 — Invisible Condition
- Status: **matches**
- Key files: `internal/combat/invisibility.go`, `internal/combat/advantage.go:67-99`, `internal/combat/invisibility_test.go`
- Findings:
  - Attacker invisible → advantage (`advantage.go:67-68`); target invisible → disadvantage on attacks against (`:98-99`); both simultaneously → cancel (test at `advantage_test.go:149`).
  - `ValidateSeeTarget` blocks single-target non-AoE non-self spells on invisible targets (`invisibility.go:49-59`); AoE still affects.
  - Standard `InvisibilitySpellID` breaks on attack/cast via `BreakInvisibilityOnAction` + `breakInvisibilityAndPersist` (which is now generalized by Phase 118's `RemoveSpellSourcedConditions`).
  - `GreaterInvisibilitySpellID` persists — only the standard slug is filtered (`invisibility.go:33`).

### Phase 114 — Surprise
- Status: **matches**
- Key files: `internal/combat/auto_skip.go`, `internal/combat/initiative.go`, `internal/combat/service.go:884-979`, `internal/combat/surprise_integration_test.go`
- Findings:
  - DM marks surprised at `StartCombat` via `SurprisedShortIDs` input field; `markSurprisedByShortIDs` resolves UUIDs and applies the condition.
  - Spec-mandated auto-skip line `⏭️ <name> is surprised — turn skipped` distinguished from other incapacitation in `FormatAutoSkipMessage` (`auto_skip.go:14-15`).
  - Surprised lasts 1 round (`SurprisedCondition()` with `DurationRounds=1`); initiative still rolled — surprised combatants take their position but skip turn 1.

### Phase 115 — Campaign Pause
- Status: **divergent (auth gap)**
- Key files: `internal/campaign/{service,handler}.go`, `cmd/dndnd/main.go:634-635`
- Findings:
  - **CRITICAL**: `campaignHandler.RegisterRoutes(router)` at `main.go:635` mounts `POST /api/campaigns/{id}/{pause,resume}` on the **bare chi router** with no auth middleware. An unauthenticated caller can pause/resume any campaign and trigger the Discord `⏸️` / `▶️` announcement. Compare with line 781 where the dashboard is mounted behind `dmAuthMw`.
  - Per spec line 749 ("Commands remain functional. Timers continue."), pause does **not** block commands — actual implementation matches the spec on this point. (The batch hint that pause "blocks player actions, allows DM" is **incorrect** w.r.t. the actual spec.)
  - Resume re-pings the current-turn player via `ResumeTurnPinger` (`service.go:158-160`). Best-effort, errors swallowed.

### Phase 116 — Tiled Import
- Status: **matches**
- Key files: `internal/gamemap/import.go`, `internal/gamemap/import_handler.go`
- Findings:
  - Three-tier validation as spec'd: hard rejection via typed sentinel errors (`ErrInfiniteMap`, `ErrNonOrthogonal`, `ErrMapTooLarge`, `ErrInvalidDimensions`, `ErrInvalidTiledJSON`); soft strip with `SkippedFeature` for animations, image layers, parallax, group layers, text/point objects, wang sets.
  - Returns sanitized JSON + `[]SkippedFeature` summary in import response (`import_handler.go:22-23, 68-71`).
  - F-7 button verification: this is dashboard-side, not surfaced here. `ImportMap` POST route is in gamemap handler; not seen behind explicit `dmAuthMw` (check map handler mount at `main.go:561-562` — same router-level pattern as campaign, **also unauthed**).

### Phase 117 — Testing Infrastructure
- Status: **matches**
- Key files: `internal/testutil/{fixtures,testdb,discordfake}.go`, `docs/testing.md`, `Makefile:1-7`
- Findings:
  - Three-tier pyramid documented at `docs/testing.md` with named exemplars per tier.
  - Fixture helpers: `NewTestCampaign`, `NewTestCharacter`, `NewTestPlayerCharacter`, `NewTestEncounter`, `NewTestCombatant`, `NewTestMap`.
  - Coverage thresholds enforced by `scripts/coverage_check` (≥90% overall, ≥85% per-package). `COVER_EXCLUDE` regex pinned in Makefile.
  - CI workflow `.github/workflows/test.yml` runs `make cover-check` on every push/PR.

### Phase 118 — Concentration Cleanup Integration
- Status: **matches**
- Key files: `internal/combat/concentration.go`, `internal/combat/condition.go:206-212`, `internal/combat/aoe.go:259`
- Findings:
  - `BreakConcentrationFully` orchestrates the full pipeline: (1) strip spell-sourced conditions across encounter via `RemoveSpellSourcedConditions`, (2) delete concentration-tagged zones via `DeleteConcentrationZonesByCombatant`, (3) dismiss summons via `DismissSummonsByConcentration`, (4) clear concentration columns, (5) emit consolidated `💨` line.
  - Trigger points wired: damage-CON-save failure (`ResolveConcentrationSave`), incapacitation (`condition.go:206-212` via `breakStoredConcentration`), Silence entry (`CheckSilenceBreaksConcentration`), replacement on new concentration cast (`applyConcentrationOnCast`), voluntary drop (dashboard).
  - The N counter in `FormatConcentrationCleanupLog` sums conditions + summons + zones (per 118b).

### Phase 118b — Concentration Cleanup Polish
- Status: **matches**
- Key files: `internal/combat/dm_dashboard_undo.go`, `db/queries/encounter_zones.sql`
- Findings:
  - `BreakConcentrationFullyResult` has `ZonesRemoved int` (not bool) and no `PerSourceMessage` field.
  - `DeleteConcentrationZonesByCombatant` returns row count via `:execrows`.
  - `OverrideCombatantPosition` routes through `Service.UpdateCombatantPosition` (verified by grep — DM-override now triggers silence-zone check).

### Phase 118c — sqlc Drift Reconciliation
- Status: **matches**
- Key files: `scripts/sqlc_drift_check/main.go`, `Makefile:1` (target `sqlc-check`)
- Findings:
  - CI guard `make sqlc-check` exists; binary excluded from coverage.

### Phase 119 — Error Log Schema Follow-up
- Status: **matches** (Option (c) — dedicated table)
- Key files: `db/migrations/20260427120001_create_error_log.sql`, `internal/errorlog/pgstore.go`
- Findings:
  - Standalone `error_log` table with first-class `command`, `user_id` (nullable), `summary`, `error_detail` (JSONB), `created_at` columns + descending index.
  - `PgStore` rewritten to target the new table; `action_log` restored to NOT NULL on `turn_id`/`encounter_id`/`actor_id` (per phase decision note).
  - Phase 112's `nullableUUID` helper removed cleanly.

### Phase 120 — E2E Test Harness
- Status: **matches**
- Key files: `cmd/dndnd/e2e_{harness,scenarios,replay}_test.go` (all `//go:build e2e`), `internal/testutil/discordfake/fake.go`, `Makefile:53` (`e2e` target)
- Findings:
  - Five scenarios: `TestE2E_RegistrationScenario`, `TestE2E_MovementScenario`, `TestE2E_LootScenario`, `TestE2E_SaveScenario`, `TestE2E_RecapEmptyScenario` — exactly the spec-named flows after 120a backfill.
  - Boots real `runWithOptions` against testcontainers Postgres + `discordfake.Fake` session.
  - `RenderTranscript()` with UUID redaction for golden assertions.
  - Build-tag gated to keep default `go test ./...` and `make cover-check` fast.

### Phase 120a — E2E Scenario Backfill
- Status: **matches**
- Key files: `cmd/dndnd/main.go:1112-1155` (RegistrationDeps wiring), `cmd/dndnd/e2e_scenarios_test.go`
- Findings:
  - `RegistrationDeps` wired into `NewCommandRouter` so `/register`, `/import`, `/create-character` route to real handlers.
  - `TestE2E_RegistrationScenario` exercises `/register → DM approve → DM welcome`.
  - `TestE2E_MovementScenario` exercises real Tiled-fixture map with `/move <coord>` → Confirm → assertion on `combatants.position_*`, `turns.movement_remaining_ft`, and `#combat-log`.
  - `TestE2E_LootScenario` exercises DM-places-loot → player `/loot` → Claim button → asserts inventory + `#the-story`.

### Phase 121 — Interactive Playtest Harness
- Status: **partial (4/5 iterations complete; phase itself still `[ ]`)**
- Key files: `docs/playtest-quickstart.md` (204 lines), `docs/playtest-checklist.md` (233 lines), `cmd/playtest-player/{main,live_session}.go`, `internal/playtest/{parser,recorder,replay}.go`, `Makefile:56-60`
- Findings:
  - 121.1 quickstart doc ✓
  - 121.2 player-agent CLI ✓ (with the spec-noted "PASTE THIS" workaround for the Discord bot-as-user limitation)
  - 121.3 transcript recorder + replay bridge ✓ (`make playtest-replay TRANSCRIPT=path`; round-trip + drift tests in `internal/playtest/replay_test.go` and `cmd/dndnd/e2e_replay_test.go`)
  - 121.4 checklist scenarios doc merged but **transcripts deferred** — every scenario has a `Status: pending` marker; phase 121.4 still flagged `[ ]`.
  - 121.5 simplify + polish ✓
  - Net: phase 121 is effectively done; only the real-session transcript backfill is pending live playtest time. The instruction "verify nothing accidentally implemented" overstates the gap — significant code shipped under the iteration markers, but it is **deliberately** scoped (the wrapper task at `[ ]` is mostly waiting on real-world play data).

## Cross-cutting concerns

- **Auth coverage of mutation HTTP endpoints**: Both `campaignHandler.RegisterRoutes` (pause/resume) and `mapHandler.RegisterRoutes` (likely including `/api/maps/import`) are mounted directly on the bare router at `main.go:561-562, 634-635`, **outside** the `dmAuthMw` group. Compare to the explicit `router.Group(func(r chi.Router) { r.Use(dmAuthMw); ... })` pattern used at line 807-811 for Open5e source mutation. This is a systemic pattern — handler packages should not assume their caller wraps them, but the canonical fix is in `cmd/dndnd/main.go`.
- **WithContext adoption**: only `/move` migrated so far; consider follow-up F-issue to migrate `/attack`, `/cast`, `/done`, etc.
- **errorlog write granularity**: only panic paths record to `error_log`. Normal error returns from handlers do not populate the DM dashboard error panel — may surprise DMs expecting to see e.g. "invalid target" errors.

## Critical items

1. **Pause/Resume endpoints unauthed** (`cmd/dndnd/main.go:635`). Any unauthenticated HTTP caller can hit `POST /api/campaigns/{id}/pause` or `.../resume`, transition campaign state, and trigger a Discord `⏸️` / `▶️` announcement in `#the-story`. F-2 violation. Same pattern likely affects `mapHandler.RegisterRoutes` at line 562 (including Tiled import). Fix: wrap both `RegisterRoutes` calls in `router.Group(func(r chi.Router) { r.Use(dmAuthMw); ... })`.
2. **errorlog only fires on panic** (`internal/discord/router.go:400-407`). Routine `error` returns from handlers never reach the dashboard error panel, even though the spec calls for "DM dashboard error notification badge (24h count), error log panel". May be intentional (panic-only is a clean signal-to-noise tradeoff), but worth confirming.
3. **WithContext is only one handler deep**. F-20 explicitly delivered the helper plus the `/move` adoption; the spec's "structured logging with contextual fields" applies to all command handlers, so adoption is currently ~5% of the surface area.
