---
id: G-102-mobile-quickactions
group: G
phase: 102
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 102 — Verify mobile QuickActionsPanel end-turn and Turns tab once Phase 95 lands

## Finding
Phase 102's mobile-lite view is otherwise complete (six-tab bottom bar, desktop-only allow-list at 1024px in `lib/layout.js`, `MobileShell.svelte`, `MobileRedirect.svelte`, `QuickActionsPanel.svelte` with pause/resume + end turn). The only caveat is inherited: the Turns tab `TurnQueue.advanceTurn` and the QuickActionsPanel end-turn button target `DMDashboardHandler` routes that are not mounted (Phase 95 gap), so end-turn 404s in production. Read-only initiative listing similarly depends on the unmounted workspace endpoint (Phase 94a gap).

## Code paths cited
- `dashboard/svelte/src/lib/layout.js` — mobile threshold 1024px, desktop-only allow-list
- `dashboard/svelte/src/MobileShell.svelte` — bottom tab bar (DM Queue/Turns/Narrate/Approvals/Message/Quick)
- `dashboard/svelte/src/MobileRedirect.svelte`
- `dashboard/svelte/src/QuickActionsPanel.svelte` — pause/resume + end turn
- Phase 95 gap (`G-95-dm-dashboard-routes-mount.md`)
- Phase 94a gap (`G-94a-workspace-handler-mount.md`)

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 102: Responsive Mobile-Lite View, "Caveat" bullet

## Acceptance criteria (test-checkable)
- [ ] After G-95 lands, the mobile QuickActionsPanel end-turn button no longer 404s
- [ ] After G-94a lands, the mobile Turns tab renders read-only initiative listing without 404s
- [ ] Test (Svelte/e2e or integration) fails before G-94a/G-95 fixes and passes after, exercising the mobile shell against the mounted endpoints
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — this task carries no independent `cmd/` edits, but depends on G-94a and G-95 which both edit `cmd/dndnd/main.go`.

## Notes
Tracked as its own task per the review doc's caveat bullet, but the actual fix lives in G-94a and G-95. Close this task once both prerequisites are merged and the mobile end-turn / initiative paths are smoke-tested.
