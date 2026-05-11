---
id: G-101-message-player-desktop
group: G
phase: 101
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 101 — Surface Message Player on desktop, add history UI and character picker

## Finding
`MessagePlayerPanel.svelte` is only used inside `MobileShell.svelte:6,56`. On the desktop App it is not in the sidebar nav (`App.svelte:198-204` lists only Combat/Maps/Encounters/Turn Builder/Shops/Narrate/Homebrew/Party) and not embedded in `CharacterOverview.svelte`, contradicting the spec's "Accessible from Character Overview or sidebar." Additionally the backend exposes `GET /api/message-player/history` (`internal/messageplayer/handler.go:73`) but no UI surface renders it — the panel is a send-only form that requires manual `playerCharacterId` UUID entry with no character picker, contradicting "logged in dashboard per-player."

## Code paths cited
- `internal/messageplayer/{handler,service,store_db,adapters}.go` — backend (mounted at `cmd/dndnd/main.go:535-536`)
- `internal/messageplayer/handler.go:73` — `History` endpoint
- `internal/characteroverview/{handler,service,store_db}.go` — mounted at `cmd/dndnd/main.go:523-524`
- `dashboard/svelte/src/App.svelte:198-204` — sidebar nav lacks Message Player entry
- `dashboard/svelte/src/CharacterOverview.svelte` — no MessagePlayerPanel embed
- `dashboard/svelte/src/MessagePlayerPanel.svelte` — send-only, requires manual UUID
- `dashboard/svelte/src/MobileShell.svelte:6,56` — only existing mount

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 101: Character Overview & Message Player

## Acceptance criteria (test-checkable)
- [ ] `MessagePlayerPanel` is reachable on the desktop UI either from the sidebar nav in `App.svelte` or embedded in `CharacterOverview.svelte` (or both, per spec wording)
- [ ] The panel includes a character picker driven by Character Overview data instead of requiring manual `playerCharacterId` UUID entry
- [ ] A history view in the panel (or adjacent UI) renders results from `GET /api/message-player/history`
- [ ] Test in the Svelte test suite (or `internal/messageplayer/handler_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — this task is primarily Svelte UI and does not need to touch `cmd/dndnd/main.go`, but stay aware of concurrent edits there from G-90/G-94a/G-95/G-97b.

## Notes
The review doc lists two distinct gaps under Phase 101 (desktop surfacing + history UI / character picker). They are bundled here because they sit on the same panel component; split during implementation if the diff grows unmanageable.
