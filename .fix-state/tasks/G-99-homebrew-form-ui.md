---
id: G-99-homebrew-form-ui
group: G
phase: 99
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 99 — Replace raw JSON textarea with structured Homebrew editor; expose class-feature-only path

## Finding
`HomebrewEditor.svelte` covers all seven categories (creatures, spells, weapons, magic-items, races, feats, classes) but the UI is a raw JSON `<textarea>` at line 169 instead of the "full stat block editor" the spec calls for; the DM must hand-type JSON matching `refdata.Upsert*Params`. Additionally, class-feature-only homebrew has no separate path — only whole-class homebrew is exposed via `/api/homebrew/classes`. The backend correctly stores `homebrew=true` but the "used alongside SRD data in all contexts" guarantee is not separately verified per ref type.

## Code paths cited
- `internal/homebrew/{handler,service,creatures,spells,weapons,magic_items,races,feats,classes}.go` — backend CRUD for 7 categories
- `dashboard/svelte/src/HomebrewEditor.svelte:169` — raw JSON `<textarea>`
- `/api/homebrew/classes` — only path for class homebrew; no class-feature-only sub-path

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 99: DM Dashboard — Homebrew Content

## Acceptance criteria (test-checkable)
- [ ] `HomebrewEditor.svelte` provides structured fields per category (creatures, spells, weapons, magic-items, races, feats, classes) instead of a raw JSON textarea
- [ ] A class-feature-only homebrew path is exposed in the UI and routed to an appropriate backend endpoint (not whole-class)
- [ ] Homebrew entries flagged `homebrew=true` continue to surface alongside SRD data in the contexts the spec calls for (search, encounter builder, etc.)
- [ ] Test in the Svelte test suite (or `internal/homebrew/*_test.go`) fails before the fix and passes after, asserting structured submission shape
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — this task is largely frontend plus possibly a new `/api/homebrew/class-features` route, so coordinate any `cmd/dndnd/main.go` edits with G-90/G-94a/G-95/G-97b.

## Notes
Three sub-findings live under Phase 99: the textarea UI, the missing class-feature-only path, and the unverified SRD-alongside-homebrew behavior. They are bundled here per the review doc's structure; split during implementation if a single PR becomes unwieldy.
