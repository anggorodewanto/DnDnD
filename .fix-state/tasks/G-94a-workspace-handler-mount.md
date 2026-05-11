---
id: G-94a-workspace-handler-mount
group: G
phase: 94a
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 94a — Mount combat WorkspaceHandler routes

## Finding
`combat.WorkspaceHandler.RegisterRoutes` is not mounted in `cmd/dndnd/main.go`, so the frontend call `getCombatWorkspace` → `GET /api/combat/workspace?campaign_id=…` (`dashboard/svelte/src/lib/api.js:451`) returns 404 in production. The `CombatManager.svelte` UI (1502 lines, multi-encounter tabs, pending-queue badges, encounter-overview bar, click-to-select, HP/condition tracker) is functional but unreachable. Phase 94b movement/interaction work depends on this fetch and inherits the same outage.

## Code paths cited
- `internal/combat/workspace_handler.go` — `WorkspaceHandler.GetWorkspace`, `RegisterRoutes`
- `cmd/dndnd/main.go` — no caller of `WorkspaceHandler.RegisterRoutes`
- `dashboard/svelte/src/lib/api.js:451` — `getCombatWorkspace` 404s in production
- `dashboard/svelte/src/CombatManager.svelte:871-888,891-901` — encounter-overview bar and pending-queue badges that depend on workspace fetch

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 94a: DM Combat Manager — Map & Token Display
- `.review-state/group-G-phases-90-103.md` — Phase 94b: inherits 94a workspace 404

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/main.go` invokes `combat.WorkspaceHandler.RegisterRoutes` so `GET /api/combat/workspace?campaign_id=…` returns 200 with a valid payload
- [ ] `CombatManager.svelte` map loads in production without 404s on the workspace fetch
- [ ] Test in `internal/combat/workspace_handler_test.go` (or `cmd/dndnd` wiring smoke test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — coordinate with G-90, G-95, and G-97b which also edit `cmd/dndnd/main.go` wiring.

## Notes
The workspace handler is a distinct handler from `DMDashboardHandler` (covered by G-95), so this is kept as a separate task even though both fixes touch the same dispatch site.
