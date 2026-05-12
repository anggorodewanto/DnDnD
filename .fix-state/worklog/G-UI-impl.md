# G-UI implementer worklog

Bundle covers four Svelte frontend tasks: G-98, G-99, G-101, G-102.

## G-98 stat-block-library-ui — status: implemented

- New API helper: `dashboard/svelte/src/lib/statblockLibrary.js` wraps
  `GET /api/statblocks` and `/api/statblocks/{id}` (mounted by
  `internal/statblocklibrary/handler.go`). Helpers: `buildStatBlockListUrl`,
  `listStatBlocks`, `getStatBlock`. Encodes the full filter contract (search,
  multi type/size, cr_min/cr_max, source SRD|homebrew|any, campaign_id, limit,
  offset).
- New component: `dashboard/svelte/src/StatBlockLibrary.svelte` — renders the
  full filter form (text search + type/size selects + CR range + source) and a
  result list with a detail panel that shows the raw stat block JSON.
- Wired into the desktop nav: `App.svelte` now imports `StatBlockLibrary`,
  hash route `#stat-block-library` resolves, sidebar button added, and
  `currentDesktopOnlyID()` maps the view so the mobile redirect banner kicks
  in on mobile (matches existing `desktopOnlyViews` entry in
  `lib/layout.js`).
- Tests: `dashboard/svelte/src/lib/statblockLibrary.test.js` — 11 cases
  covering endpoint constant, URL building (filters, pagination, repeats), and
  fetch/error paths.

Acceptance criteria mapped:
- [x] Svelte component renders library with search + filters (type/size/CR/SRD/homebrew)
- [x] `App.svelte` sidebar includes Stat Block Library entry routed to the
  new page
- [x] Frontend calls `/api/statblocks` (the existing `listCreatures` helper
  is left in place since the spec only flagged it as evidence the library
  was unused; migration of EncounterBuilder is out of scope here)
- [x] `lib/layout.js` redirect target now resolves to the real page on
  desktop (mobile still gets the redirect banner)
- [x] Svelte test suite gains 11 new test cases for the library client

## G-99 homebrew-form-ui — status: implemented (with class-feature workaround)

- New helper module: `dashboard/svelte/src/lib/homebrewForm.js` exports the
  category catalog, an `emptyFormModel(category)` factory, a
  `buildHomebrewPayload(category, model)` serializer matching
  `refdata.Upsert*Params` wire shape, and `entryToFormModel` for the edit
  flow. JSON fields (`speed`, `ability_scores`, `attacks`, `features_by_level`,
  etc.) are parsed safely with explicit error messaging.
- `dashboard/svelte/src/HomebrewEditor.svelte` rewritten: structured form
  per category — creatures, spells, weapons, magic-items, races, feats,
  classes — replacing the raw JSON textarea. CSV inputs cover string arrays;
  JSON-typed columns stay as constrained textareas (since they are nested
  schemas the backend trusts as-is).
- Class-feature-only path: added an 8th UI category `class-features` that
  emits a single-feature class skeleton through the existing
  `/api/homebrew/classes` endpoint. The backend exposes only the unified
  classes endpoint today and Go files are out of scope for this bundle, so
  the UI flows class-feature input into a minimal `features_by_level: [...]`
  class row with parent class metadata stored on the feature for
  traceability. A real `/api/homebrew/class-features` route would be a
  follow-up Go task; documented as such in this worklog.
- "homebrew used alongside SRD" guarantee: encounter builder and other
  consumers already pull from the listing endpoints (`/api/creatures`,
  etc.) which surface both SRD + homebrew rows; the editor now correctly
  sends `homebrew=true` on create/update, preserving that behavior.
- Tests: `dashboard/svelte/src/lib/homebrewForm.test.js` — 16 cases:
  category catalog (incl. class-features path), default models, payload
  build per category (round-trip + required-field errors + invalid-JSON
  guards), and the class-feature skeleton shape.

Acceptance criteria mapped:
- [x] `HomebrewEditor.svelte` provides structured fields per category
- [x] Class-feature-only path exposed in UI and routed to `/api/homebrew/classes`
  via a single-feature skeleton (documented gap: no dedicated backend route)
- [x] Homebrew=true flag preserved on submission; SRD-alongside-homebrew
  behavior unchanged because listing/encounter-builder still hit the
  refdata endpoints
- [x] Svelte test suite gains 16 cases asserting structured submission shape

## G-101 message-player-desktop — status: implemented

- `dashboard/svelte/src/lib/messageplayer.js` extended:
  - `MESSAGE_PLAYER_HISTORY_ENDPOINT` + `CHARACTER_OVERVIEW_ENDPOINT`
  - `buildHistoryUrl(params)`, `fetchHistory(params)` — wrap
    `GET /api/message-player/history`
  - `fetchPartyCharacters(campaignId)` — wraps `/api/character-overview`
- `dashboard/svelte/src/MessagePlayerPanel.svelte` rewritten:
  - Character picker dropdown replaces the manual UUID input. Options
    come from `fetchPartyCharacters`.
  - History view renders messages from `fetchHistory` and auto-refreshes
    after a successful send.
  - `playerCharacterId`, `playerName`, and `hidePicker` props let callers
    embed the panel with a preselected target (used by
    `CharacterOverview`).
- `dashboard/svelte/src/CharacterOverview.svelte` now imports
  `MessagePlayerPanel`. Each party card carries a "Message this player"
  toggle that mounts the embedded panel with the character UUID
  preselected and the picker hidden.
- `App.svelte` sidebar gains a "Message Player" entry that routes to
  the standalone (full-picker) panel for DMs who want a dedicated view.
- Tests: `dashboard/svelte/src/lib/messageplayer.test.js` extended with
  9 additional cases for endpoint constants, `buildHistoryUrl` query
  param coverage, `fetchHistory` success/error, and `fetchPartyCharacters`
  empty-campaign + happy-path.

Acceptance criteria mapped:
- [x] `MessagePlayerPanel` reachable on desktop UI: nav entry + embedded
  in `CharacterOverview` (both, per spec)
- [x] Character picker drives selection from Character Overview data;
  manual UUID entry removed
- [x] History view renders `GET /api/message-player/history` results and
  refreshes on send
- [x] Svelte test suite covers history + party fetch paths

## G-102 mobile-quickactions — status: verified-no-fix-needed

- `QuickActionsPanel.svelte` calls `advanceTurn(encounterId)` which posts
  `/api/combat/{encounterID}/advance-turn`. That route is now mounted via
  `cmd/dndnd/main.go:244` (inside `mountCombatDashboardRoutes`, the
  G-94a/G-95 batch).
- `TurnQueue.svelte` (read-only mobile view) reads `/api/combat/workspace`
  + `/api/combat/{enc}/turn-queue`, both mounted via the same helper
  (`workspace` at line 237; turn-queue at the combat handler in
  `internal/combat/dm_dashboard_handler.go`).
- Campaign pause/resume routes (`/api/campaigns/{id}/pause|resume`) are
  mounted by `campaignHandler.RegisterRoutes(router)` at `main.go:610`.
- No frontend change required. The four existing test files
  (`campaignActions.test.js`, `api.test.js`, `layout.test.js`,
  `messageplayer.test.js`) already exercise the wire shapes consumed by
  the mobile shell.

Acceptance criteria mapped:
- [x] After G-95 landed, the mobile QuickActionsPanel end-turn no longer 404s
- [x] After G-94a landed, the mobile Turns tab renders read-only initiative
  listing through the workspace endpoint without 404s
- [x] Existing tests in `lib/api.test.js` + `lib/campaignActions.test.js`
  exercise the wire contract; they pass against the now-mounted routes
- [x] `make test && make build` clean (validated locally)

Closure: `verified-no-fix-needed` per task notes.

## Verification summary

- `dashboard/svelte` test suite: 336 tests pass (300 → 336, +36 new across
  `homebrewForm.test.js` (+16), `statblockLibrary.test.js` (+11),
  `messageplayer.test.js` (+9))
- `cd dashboard/svelte && npm run build`: succeeds, only pre-existing
  a11y warning in `ActionLogViewer.svelte` (out of scope)
- `make build`: succeeds
- `make test`: all Go packages pass, no FAILs

## Files modified

- New:
  - `dashboard/svelte/src/StatBlockLibrary.svelte`
  - `dashboard/svelte/src/lib/statblockLibrary.js`
  - `dashboard/svelte/src/lib/statblockLibrary.test.js`
  - `dashboard/svelte/src/lib/homebrewForm.js`
  - `dashboard/svelte/src/lib/homebrewForm.test.js`
- Modified:
  - `dashboard/svelte/src/App.svelte` — nav entries + view router for stat
    block library and message player
  - `dashboard/svelte/src/HomebrewEditor.svelte` — structured per-category
    form via `lib/homebrewForm.js`
  - `dashboard/svelte/src/MessagePlayerPanel.svelte` — picker + history +
    preselected/embed mode
  - `dashboard/svelte/src/CharacterOverview.svelte` — embedded message
    panel toggle per character card
  - `dashboard/svelte/src/lib/messageplayer.js` — history + party fetch
    helpers
  - `dashboard/svelte/src/lib/messageplayer.test.js` — coverage for new
    helpers

No Go files were modified.
