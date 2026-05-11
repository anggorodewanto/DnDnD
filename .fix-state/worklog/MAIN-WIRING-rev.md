verdict: approve
reviewer: fresh-reviewer
date: 2026-05-11

## Per-task verdict
- G-94a: approve — `mountCombatDashboardRoutes` registers all 5 workspace routes; chi.Walk test asserts presence.
- G-95: approve — same helper binds advance-turn / pending-actions / action-log / undo / 5 overrides; all 8 paths asserted in the walk test.
- G-97b: approve — `combat.NewDMDashboardHandlerWithDeps(svc, db, poster)` is the constructor; `combatLogPoster` constructed when `discordSession != nil` (main.go:663-666). See concern 1.
- H-105b: approve — `wireEnemyTurnNotifier` calls `combat.Handler.SetEnemyTurnNotifier` (turn_builder_handler.go:25); spy test confirms; nil-safe variant tested.
- G-90: approve — `ddbimport.NewService(NewDDBClient(), queries)` threaded via `RegistrationDeps.DDBImporter`; integration test drives `/import` interaction through `discord.NewCommandRouter` and asserts `Import` is called once. DM approval / dashboard warning UI deferred (worklog flags this).

## Findings
1. `TestMountCombatRoutes_InjectsCombatLogPoster` (main_wiring_test.go:395-403) verifies the poster round-trips through `combatDashboardWiring.poster`, not that it actually reaches `NewDMDashboardHandlerWithDeps`. A refactor that returned the poster but didn't pass it to the constructor would still pass. The internal `dm_dashboard_undo_integration_test.go` covers the behavioural path, so this is a minor tautology rather than a gap.
2. Unused test helpers `newTestHTTPRequest` / `newTestHTTPRecorder` (main_wiring_test.go:507-514) — lint cleanup; non-blocking.
3. Worklog reported `cmd/dndnd: 64.0%`; actual is `63.4%` (-0.6pp). Still passes (main.go excluded).
4. Deviation to `internal/discord/router.go` is documented and minimal (one field + one nil-check); legitimate because `ImportHandler` has no public setter post-construction.
5. Working tree contains 25+ files modified outside the MAIN-WIRING zone (combat/*, discord/*_handler.go, refdata/*, dashboard/*). These are from parallel bundles (DEATH-SAVE, DISPATCH-D, C-33) and are out of scope for this review.

## Verification
- `make build`: pass
- `make test`: pass (no flakes observed in cache)
- `make cover-check`: pass (overall ≥90%, per-package ≥85%; `cmd/dndnd` excluded sources hold)

## Recommended next steps
- Optional: strengthen `TestMountCombatRoutes_InjectsCombatLogPoster` to trigger an override and assert `PostCorrection` fires (closes finding 1).
- Optional: delete unused `newTestHTTPRequest` / `newTestHTTPRecorder` (finding 2).
- Follow-up tasks (out of bundle): DM approval / discard handler, ValidationWarning surface in dashboard UI (G-90 second/third bullets).
