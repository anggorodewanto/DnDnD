---
id: G-98-stat-block-library-ui
group: G
phase: 98
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 98 — Build Stat Block Library UI

## Finding
The stat block library backend is complete and mounted (`internal/statblocklibrary/{handler,service}.go` at `cmd/dndnd/main.go:471-472`) with search/filter (type, size, CR, SRD/homebrew) and per-campaign Open5e source gating. The UI is missing entirely: no Svelte component for browsing stat blocks, no nav button in `App.svelte:198-204`, and the Encounter Builder uses `/api/creatures` (refdata `listCreatures`) directly via `lib/api.js:179` instead of `/api/statblocks`. References to `'stat-block-library'` in `dashboard/svelte/src/lib/layout.js:44` exist only as a desktop-only redirect target with no actual page.

## Code paths cited
- `internal/statblocklibrary/{handler,service}.go` — backend (complete)
- `cmd/dndnd/main.go:471-472` — backend mount
- `dashboard/svelte/src/App.svelte:198-204` — sidebar nav (missing entry)
- `dashboard/svelte/src/lib/api.js:179` — `listCreatures` (wrong endpoint for library browsing)
- `dashboard/svelte/src/lib/layout.js:44` — `'stat-block-library'` referenced but has no page

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 98: DM Dashboard — Stat Block Library

## Acceptance criteria (test-checkable)
- [ ] A new Svelte component renders the stat block library with search and filters (type, size, CR, SRD/homebrew)
- [ ] `App.svelte` sidebar nav includes a Stat Block Library entry that routes to the new page
- [ ] Frontend calls `/api/statblocks` (not `/api/creatures`) for library browsing
- [ ] `dashboard/svelte/src/lib/layout.js:44` `'stat-block-library'` redirect target resolves to the real page
- [ ] Test in the Svelte test suite (or a backend integration test exercising `/api/statblocks`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — backend is already mounted, so this task mainly touches the Svelte tree, but stay aware of concurrent edits to `cmd/dndnd/main.go` from G-90/G-94a/G-95/G-97b.

## Notes
Per the review doc the backend is complete; this is a UI-only build-out. Encounter Builder's existing `listCreatures` call may need to be re-evaluated for whether it should also migrate to `/api/statblocks`, but the doc only flags this as evidence the library is unused, not as a required migration.
