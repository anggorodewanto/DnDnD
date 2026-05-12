# Remediation State

Started: 2026-05-12
Driver: orchestrator (Claude Opus 4.7)
Source: `.review-findings/FINAL_REVIEW.md`

## Baseline
- `go build ./...` clean on `main` @ dbb1464.

## Pending
- [ ] F-14: Phase 88a — `modify_speed` not handled in convertPassiveEffect — severity: LOW — origin: FINAL_REVIEW.md §Low #14
- [ ] F-15: Phase 81 — SingleCheck adjacency/action-cost only in handler layer — severity: LOW — origin: FINAL_REVIEW.md §Low #15
- [ ] F-16: Phase 89 — level-up widget server-rendered HTML — severity: LOW — origin: FINAL_REVIEW.md §Low #16
- [ ] F-17: Phase 27 — advisory-lock UUID collapse undocumented — severity: LOW — origin: FINAL_REVIEW.md §Low #17
- [ ] F-18: Rate-limit queue reactive (Discord) — severity: LOW — origin: FINAL_REVIEW.md §Low #18
- [ ] F-19: No CSRF token on state-changing dashboard POSTs — severity: LOW — origin: FINAL_REVIEW.md §Low #19
- [ ] F-20: Structured-log contextual fields not centralized — severity: LOW — origin: FINAL_REVIEW.md §Low #20
- [ ] F-21: Phase 5 — spell `resolution_mode` not auto-classified — severity: LOW — origin: FINAL_REVIEW.md §Low #21
- [ ] F-22: Phase 21a phases.md note stale — severity: LOW — origin: FINAL_REVIEW.md §Low #22
- [ ] F-23: Phase 17 character card field cross-verify (Concentration/Conditions populated) — severity: LOW — origin: FINAL_REVIEW.md §Low #23

## In Progress
- (none)

## Done (closed + reviewer passed)
- [x] F-13: loot pool widget — closed 2026-05-12 — new Svelte `LootPoolPanel.svelte` wired into App.svelte nav, reuses existing `ItemPicker` (search + custom + creature-loot tabs). Reuses backend CRUD endpoints; added `GET /api/campaigns/{cid}/loot/eligible-encounters` (completed-status filter) since neither `/api/encounters` (templates) nor `/api/combat/workspace` (active only) listed completed live encounters. 7 vitest cases for the new API helpers + 2 Go handler tests; full Go + 360 vitest suites green; `npm run build` clean.
- [x] F-1: WS URL mismatch — closed 2026-05-12 — review: PASS — commit `a5ed9de`
- [x] F-2: DM-role enforcement on dashboard routes — closed 2026-05-12 — review: PASS — commit `b23a8cd`. Worker flagged follow-up: per-resource authz on `/api/maps/import`, `/api/combat/*`, `/api/levelup`, party-rest, loot/item-picker/shops (separate scope).
- [x] F-4: TurnGate write-inside-lock — closed 2026-05-12 — added `combat.RunUnderTurnLock` + `TurnGate.AcquireAndRun(ctx, encID, userID, fn)` that holds the per-turn advisory lock across the caller's write callback. Tx is threaded into fn's context via `combat.ContextWithTx` / `combat.TxFromContext` so handlers can opt into running their writes on the lock-holding tx; peers block at `pg_advisory_xact_lock` until our tx commits/rolls back regardless. Migrated `/move` `HandleMoveConfirm` as the proof case. 10 new combat integration tests cover happy/error/panic/concurrency/rollback; 3 new discord unit tests cover the migration. Follow-up: `HandleMoveConfirmWithMode`, `HandleFlyConfirm`, `cast_handler`, `attack_handler`, `bonus_handler`, `shove_handler`, `interact_handler` confirm/persist paths still call `AcquireAndRelease` (or rely on a separate confirm-button flow); migrating each is a separate ticket (no functional regression — all gates still fire pre-write, just without the post-validation lock-held window).
- [x] F-8: Open5e source-toggle UI — closed 2026-05-12 — review: PASS — commit `45a0280`. New `internal/open5e/sources.go` catalog + GET (public) /api/open5e/sources, GET/PUT /api/open5e/campaigns/{id}/sources (DM-gated). JSONB partial merge preserves other settings. Svelte panel + nav tab + api helpers. 11 handler + 7 vitest tests; coverage 93.6%. Catalog hand-curated (follow-up: auto-sync with Open5e `/v1/documents/`).
- [x] F-7: Tiled .tmj import UI — closed 2026-05-12 — review: PASS — commit `62dc9db`. UX: separate "Import Tiled (.tmj)" button on the New Map form; renders skipped-features callout. 4 API tests, SPA rebuilt. Inherits `/api/maps/import` ungated-by-RequireDM gap from F-2 follow-up.
- [x] F-6: Fly min_machines_running=1 — closed 2026-05-12 — direct config edit (4-line fly.toml change), user-approved; commit `8e9fff7`. Self-verified.
- [x] F-12: queue list view — closed 2026-05-12 — review: PASS — commit `556dda6`. GET /dashboard/queue/ list endpoint behind dmAuthMw; reused existing ListPendingDMQueueItems(campaign_id). Svelte DMQueuePanel + nav tab. 7 Go + 6 vitest tests; dashboard cov 92.8%.
- [x] F-11: sqlc-drift Makefile target — closed 2026-05-12 — direct edit, self-verified. CI already runs the drift check (`.github/workflows/test.yml:50-55`); added local convenience `make sqlc-check`. Commit `bc52a11`.
- [x] F-10: portal builder gaps — closed 2026-05-12 — review: PASS — commit `2671af5`. All 4 gaps closed (subclass picker, multiclass rows, subrace, background skills). Subrace/background stashed in `character_data` JSONB (no migration). Multiclass HP/AC uses primary class for MVP. 24 frontend + 5 Go tests; portal cov 94.2%.
- [x] F-9: Phase 104b magic-item handler wired into /attune — closed 2026-05-12 — Added `AttunePublisher` interface + `SetPublisher` on `*discord.AttuneHandler`; nil-safe call to `PublishForCharacter(ctx, char.ID)` after successful `UpdateCharacterAttunementSlots`. Wired in `cmd/dndnd/main.go` after `buildDiscordHandlers` using the existing `magicItemSvc` (replaced `_ = magicItemSvc` placeholder). Two new unit tests assert publisher invoked on success / NOT invoked on persistence error; existing /attune tests unchanged. Build clean; `internal/discord` coverage 86.3%.
- [x] F-3: `conditions_ref` SQL table — closed 2026-05-12 (NO-CODE-CHANGE; already implemented in Phase 3 — `4a1c5d3`). Verified: table created in `db/migrations/20260310120002_create_reference_tables.sql:31-38` with spec columns (id PK, name, description, mechanical_effects JSONB); sqlc queries `GetCondition`, `ListConditions`, `CountConditions`, `UpsertCondition` in `db/queries/conditions.sql`; 16-row seed (14 SRD + exhaustion + surprised) in `internal/refdata/seeder.go:180-355` via `UpsertCondition ON CONFLICT DO UPDATE` (idempotent); integration tests `TestIntegration_ReferenceTablesMigration`, `TestIntegration_SeedAll_ListConditions`, `TestConditionCount_IncludesSurprised` all PASS. Finding text in `FINAL_REVIEW.md §Medium #3` and `.review-findings/05-cross-cutting.md` was stale at audit time. Mechanical-effects JSONB format: `[{effect_type, target?, condition?, value?, description?}]` — descriptive vocabulary (Feature Effect System terms); Go code in `internal/combat/condition_effects.go` remains the source of truth for enforcement per Design Decision #2.

## Skipped (with justification)
- [~] DEFER-1: Phase 121.4 transcripts — documented deferral until live playtest (FINAL_REVIEW.md §Acknowledged)
- [~] DEFER-2: Phase 84 combat-time `/use`/`/give` costs — explicitly deferred at phases.md:485
- [~] F-5: No XP awarding pipeline — STALE REVIEW. Spec `docs/dnd-async-discord-spec.md:2449` explicitly states: "Progression model: milestone only. The DM decides when characters level up based on story progression — there is no XP tracking, no XP rewards, and no XP fields in the data model." Encounter-end summary (spec 2880-2920) lists rounds + casualties; no XP mention. FINAL_REVIEW.md §Medium #5 misread the spec. No action required.

## Final Readiness Review
- Status: NOT STARTED
- Last run: —
- Notes: —

## Build/Test log
- 2026-05-12 boot: `go build ./...` clean
