---
id: G-97b-combat-log-poster-construction
group: G
phase: 97b
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 97b — Inject CombatLogPoster into DMDashboardHandler in production

## Finding
Even once the Phase 97b override/undo routes are mounted (see `G-95-dm-dashboard-routes-mount`), `CombatLogPoster` injection is required so the Discord `postCorrection` call at `internal/combat/dm_dashboard_undo.go:44` actually delivers correction messages. `NewDMDashboardHandlerWithDeps` is only invoked from `dm_dashboard_undo_integration_test.go`; no caller in `cmd/` constructs the handler with a real `CombatLogPoster`, so correction posts will be silently dropped (or panic) in production.

## Code paths cited
- `internal/combat/dm_dashboard_undo.go:44` — `postCorrection` via `CombatLogPoster`
- `internal/combat/dm_dashboard_undo.go:60` — `UndoLastAction`
- `internal/combat/dm_dashboard_undo.go:366-558` — override HP/Position/Conditions/Initiative/SpellSlots
- `internal/combat/dm_dashboard_undo_integration_test.go` — only caller of `NewDMDashboardHandlerWithDeps`
- `cmd/dndnd/main.go` — no `CombatLogPoster` wiring for DMDashboardHandler

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 97b: Undo & Manual Corrections, second bullet ("CombatLogPoster injection is also required")

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/main.go` constructs `DMDashboardHandler` via `NewDMDashboardHandlerWithDeps` (or equivalent) with a real `CombatLogPoster`
- [ ] Invoking `UndoLastAction` / any override endpoint posts a correction message to Discord through `CombatLogPoster.postCorrection`
- [ ] Test in `internal/combat/dm_dashboard_undo_integration_test.go` (or a new wiring test) fails before the fix and passes after, asserting the poster is called
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — coordinate with G-95 (route mount), G-90, and G-94a which also edit `cmd/dndnd/main.go`.
- Logically requires G-95 to land first or in the same change so the constructor and route registration stay consistent.

## Notes
Split out from `G-95-dm-dashboard-routes-mount` because injecting the poster requires switching the constructor used at wire-up time, not just registering routes. The review doc lists this as a distinct sub-bullet under Phase 97b.
