---
id: G-95-dm-dashboard-routes-mount
group: G
phase: 95
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 95/97a/97b — Mount DMDashboardHandler routes (turn queue, action log, undo/override)

## Finding
`combat.DMDashboardHandler.RegisterRoutes` is not mounted in `cmd/dndnd/main.go` (no caller in `cmd/`). The handler exposes `/{encounterID}/advance-turn`, `/pending-actions`, `/pending-actions/{actionID}/resolve` (Phase 95), `/{encounterID}/action-log` (Phase 97a), and the full undo/override surface (`UndoLastAction`, HP/Position/Conditions/Initiative/SpellSlots overrides — Phase 97b). All of these 404 in production. Frontend calls in `dashboard/svelte/src/lib/api.js` (`advanceTurn` line 518, `getPendingActions` line 533, `resolvePendingAction` line 543, `listActionLog` line 816, and the override APIs) all fail. Service-level logic and integration tests pass, but the HTTP surface is unreachable. Phase 102's mobile QuickActionsPanel end-turn button inherits this failure.

## Code paths cited
- `internal/combat/dm_dashboard_handler.go:48-50` — `/{encounterID}/advance-turn`, `/pending-actions`, `/pending-actions/{actionID}/resolve`
- `internal/combat/dm_dashboard_handler.go:51` — `/{encounterID}/action-log` mount
- `internal/combat/action_log_viewer.go` — service backing 97a
- `internal/combat/dm_dashboard_undo.go:60` — `UndoLastAction`
- `internal/combat/dm_dashboard_undo.go:27` — `withTurnLock`
- `internal/combat/dm_dashboard_undo.go:44` — Discord `postCorrection` via `CombatLogPoster`
- `internal/combat/dm_dashboard_undo.go:366-558` — HP/Position/Conditions/Initiative/SpellSlots overrides
- `cmd/dndnd/main.go` — no `DMDashboardHandler.RegisterRoutes` caller
- `dashboard/svelte/src/lib/api.js:518,533,543,816` — frontend calls that 404
- `dashboard/svelte/src/{TurnQueue,ActionResolver,ActionLogViewer}.svelte` and `lib/diff.js`

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 95: Turn Queue & Action Resolver
- `.review-state/group-G-phases-90-103.md` — Phase 97a: Action Log Viewer
- `.review-state/group-G-phases-90-103.md` — Phase 97b: Undo & Manual Corrections
- `.review-state/group-G-phases-90-103.md` — Phase 102: QuickActionsPanel end-turn inherits gap

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/main.go` invokes `combat.DMDashboardHandler.RegisterRoutes` so the Phase 95/97a/97b endpoints return non-404
- [ ] `advanceTurn`, `getPendingActions`, `resolvePendingAction` work end-to-end from the Svelte UI
- [ ] `listActionLog` returns filtered/sorted results for `/{encounterID}/action-log`
- [ ] `undoLastAction`, `overrideCombatantHP`, `overrideCombatantPosition`, `overrideCombatantConditions`, `overrideCombatantInitiative`, `overrideCharacterSpellSlots` are reachable
- [ ] Mobile QuickActionsPanel end-turn no longer 404s
- [ ] Test in `cmd/dndnd` wiring smoke or `internal/combat/dm_dashboard_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — coordinate with G-90, G-94a, and G-97b (CombatLogPoster construction) which also edit `cmd/dndnd/main.go` wiring.
- 97b override endpoints additionally need `CombatLogPoster` injected; tracked separately in `G-97b-combat-log-poster-construction.md`.

## Notes
The review doc treats Phases 95, 97a, and 97b as sharing the same root cause (`DMDashboardHandler` never mounted), so they are consolidated here per the campaign rules. The `CombatLogPoster` injection issue for 97b is split out because it requires a constructor-level change (`NewDMDashboardHandlerWithDeps`) beyond the route mount.
