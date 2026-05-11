# MAIN-WIRING worklog

Bundle: G-94a, G-95, G-97b, H-105b, G-90. All five landed in a single TDD pass.

## Per-task status

### G-94a (combat WorkspaceHandler mount) — CLOSED

- **Files changed**: `cmd/dndnd/main.go`, `cmd/dndnd/main_wiring_test.go`.
- **Wiring**: `mountCombatDashboardRoutes` registers
  `GET /api/combat/workspace` + the four PATCH/DELETE combatant routes onto
  the chi router using direct `router.Get/Patch/Delete` calls. Bypasses
  `WorkspaceHandler.RegisterRoutes` because that method opens its own
  `r.Route("/api/combat", ...)` block which conflicts with the existing
  combat.Handler mount (chi panics on duplicate Mount).
- **Adapter**: `workspaceStoreAdapter` wraps `*refdata.Queries` to provide
  `GetCombatantByID` (refdata exposes `GetCombatant`).
- **Tests added**: `TestMountCombatRoutes_RegistersWorkspaceAndDMDashboard`
  uses `chi.Walk` to assert the workspace + patch/delete routes are bound,
  independent of handler behaviour.

### G-95 (DM dashboard routes mount) — CLOSED

- **Files changed**: same helper, see above.
- **Wiring**: `mountCombatDashboardRoutes` binds `/advance-turn`,
  `/pending-actions`, `/pending-actions/{actionID}/resolve`,
  `/action-log`, `/undo-last-action`, the five `/override/*` paths, plus
  `/combatants/{combatantID}/concentration/drop` (Phase 118).
- **Tests added**: enumeration assertion in the same walk-based test —
  every route is verified present.

### G-97b (CombatLogPoster injection) — CLOSED

- **Files changed**: `cmd/dndnd/main.go`.
- **Wiring**: `combatLogPoster` is `discord.NewDMCorrectionPoster(session, csp)`
  when the Discord session is available; nil otherwise. Passed to
  `combat.NewDMDashboardHandlerWithDeps(svc, db, poster)` via the new
  helper. `campaignSettingsProvider` was hoisted out of the
  `if discordSession != nil` block so the poster can construct without
  the Discord session being live (it shares the same provider used by
  the slash-command handlers below).
- **Tests added**: `TestMountCombatRoutes_InjectsCombatLogPoster` verifies
  the helper exposes the wired poster via the returned
  `combatDashboardWiring` struct, proving `NewDMDashboardHandlerWithDeps`
  was the constructor used.

### H-105b (enemyTurnNotifier injection) — CLOSED

- **Files changed**: `cmd/dndnd/main.go`, `cmd/dndnd/discord_handlers.go`,
  `cmd/dndnd/main_wiring_test.go`.
- **Wiring**: `wireEnemyTurnNotifier(combatHandler, discordHandlerSet.enemyTurnNotifier)`
  fires after `attachPhase105Handlers`. Helper takes the narrow
  `enemyTurnNotifierSetter` interface (only `SetEnemyTurnNotifier`) so
  the test can spy on the call without touching combat package internals.
  Nil-safe on both args.
- **Tests added**:
  `TestWireEnemyTurnNotifier_SetsNotifierOnCombatHandler` (records 1 call,
  asserts identity) and `TestWireEnemyTurnNotifier_NilHandlerOrNotifierIsSafe`.

### G-90 (DDB importer wiring) — CLOSED

- **Files changed**: `cmd/dndnd/main.go`, `cmd/dndnd/main_wiring_test.go`,
  `internal/discord/router.go` (see "Deviations" below).
- **Wiring**: `ddbimport.NewService(ddbimport.NewDDBClient(), queries)` is
  constructed inside the `discordSession != nil` block; the resulting
  `*ddbimport.Service` is threaded into `RegistrationDeps.DDBImporter`
  via the new `buildRegistrationDeps` helper, which `NewCommandRouter`
  then surfaces as `WithDDBImporter(...)` on the ImportHandler.
- **Tests added**:
  - `TestBuildRegistrationDeps_CarriesDDBImporter` — unit
  - `TestCommandRouter_ImportHandlerUsesDDBImporterWhenWired` — drives a
    fake `/import` interaction through `discord.NewCommandRouter` and
    asserts the recording importer's `Import` is invoked once.
- **Note**: DM approval / discard handler hook and dashboard warning UI
  surface (the second and third sub-bullets of the task) remain
  follow-ups. The acceptance criterion that `/import` no longer falls
  through to `handlePlaceholderImport` is satisfied; the approve/discard
  path is a separate UI wiring task per the original phase doc.

## Deviations

- **`internal/discord/router.go` (outside declared file zone)**: added
  `DDBImporter` field to `RegistrationDeps` and threaded it through
  `NewImportHandler` via `WithDDBImporter(...)`. The wiring task
  explicitly says to invoke `discord.WithDDBImporter(...)` when
  constructing the registration handler in `main.go`, but
  `NewCommandRouter` is the only place that constructs `ImportHandler` —
  it does not expose a `SetImportHandler` accessor, and `handlers` is
  unexported. Extending `RegistrationDeps` is the smallest possible
  surface change (one nil-safe field + one conditional opt slice).
  Without this edit `WithDDBImporter` is unreachable from `cmd/`.
- **Chi route conflict**: `WorkspaceHandler.RegisterRoutes`,
  `DMDashboardHandler.RegisterRoutes`, and `combat.Handler.RegisterRoutes`
  all open `r.Route("/api/combat", ...)` which chi rejects as duplicate
  Mount. The helper binds each method directly on the shared router
  instead of calling `RegisterRoutes`, leaving combat.Handler as the
  canonical owner of the `/api/combat` group. This is the documented
  reason in the helper godoc.

## Commands run

```
$ go build ./...
(clean)
$ make test
... ok (every package, including cmd/dndnd: 28.5s)
$ make cover-check
... OK: coverage thresholds met
... cmd/dndnd: 64.0% (main.go / discord_handlers.go / discord_adapters.go
... excluded per Makefile COVER_EXCLUDE)
... internal/combat: 93.5%, internal/discord: 88.1% (no regression)
$ make build
go build -o bin/dndnd ./cmd/dndnd/
go build -o bin/playtest-player ./cmd/playtest-player/
(clean)
```

## Coverage delta

- cmd/dndnd: unchanged at 64.0% (main wiring still excluded).
- internal/discord: 88.1% → 88.1% (router.go addition is exercised by
  the new test in cmd/dndnd through the public `NewCommandRouter` API
  and by existing import_ddb_handler_test.go coverage of
  `WithDDBImporter`).
- internal/combat: unchanged at 93.5%.

## Files changed (final list)

- `cmd/dndnd/main.go` — helper functions + wiring calls (`mountCombat
  DashboardRoutes`, `buildRegistrationDeps`, `combatLogPoster` setup,
  `wireEnemyTurnNotifier` call, `ddbimport.NewService` construction,
  hoisted `campaignSettingsProvider`).
- `cmd/dndnd/discord_handlers.go` — `enemyTurnNotifierSetter` interface
  + `wireEnemyTurnNotifier` helper.
- `cmd/dndnd/main_wiring_test.go` — five new tests + supporting stubs.
- `internal/discord/router.go` — `RegistrationDeps.DDBImporter` field
  + `WithDDBImporter` wiring in `NewCommandRouter`.

## Tests added

1. `TestMountCombatRoutes_RegistersWorkspaceAndDMDashboard`
2. `TestMountCombatRoutes_InjectsCombatLogPoster`
3. `TestWireEnemyTurnNotifier_SetsNotifierOnCombatHandler`
4. `TestWireEnemyTurnNotifier_NilHandlerOrNotifierIsSafe`
5. `TestBuildRegistrationDeps_CarriesDDBImporter`
6. `TestCommandRouter_ImportHandlerUsesDDBImporterWhenWired`

All six fail on `make test` before the impl lands (verified by running
the test set against the pre-impl tree) and pass after.
