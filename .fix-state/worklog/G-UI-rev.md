# G-UI reviewer worklog

Bundle: G-98, G-99, G-101, G-102 (Svelte frontend). Read-only audit.

## G-98 stat-block-library-ui — APPROVED

- `dashboard/svelte/src/StatBlockLibrary.svelte` + `lib/statblockLibrary.js`
  present; helper covers full filter contract (search, type/size repeats,
  cr_min/cr_max, source, campaign_id, limit, offset).
- `App.svelte` adds sidebar nav button, `#stat-block-library` hash route,
  view router branch, and desktopOnlyID mapping. `lib/layout.js:44` allow-list
  entry already present.
- 11 new lib tests pass. Embedding renders a JSON `<pre>` for the detail
  panel — pragmatic given the spec is read-only browsing.

## G-99 homebrew-form-ui — APPROVED with note

- `lib/homebrewForm.js` field names match `refdata.Upsert*Params` JSON tags
  for all 7 native categories: creatures (creatures.sql.go:286), spells
  (spells.sql.go:477), weapons (weapons.sql.go:133), magic-items
  (magic_items.sql.go:229), races (races.sql.go:132), feats (feats.sql.go:121),
  classes (classes.sql.go:150). CSV→[]string and JSON-textarea→json.RawMessage
  conversions correct.
- `HomebrewEditor.svelte` per-field forms render structured inputs for all
  categories (creatures/spells/weapons partially inspected; full diff confirms
  remaining four wired analogously).
- 8th `class-features` category synthesizes single-feature class skeletons via
  `/api/homebrew/classes`. Documented as a workaround — no backend
  `/api/homebrew/class-features` route added. The spec wanted "class-feature-only
  path"; the worklog's acceptance bullet is met but the spec gap remains as a
  follow-up.
- 16 new lib tests pass.

## G-101 message-player-desktop — APPROVED

- `MessagePlayerPanel.svelte` rewritten: character picker drives
  `/api/character-overview`, history pulls `/api/message-player/history`,
  refresh on send. `playerCharacterId`/`playerName`/`hidePicker` props enable
  embed. `CharacterOverview.svelte` adds per-card toggle. `App.svelte` adds
  sidebar nav + view router branch.
- `lib/messageplayer.js` adds `buildHistoryUrl`/`fetchHistory`/
  `fetchPartyCharacters`. 9 new tests pass.

## G-102 mobile-quickactions — VERIFIED

- `QuickActionsPanel.advanceTurn` → `/api/combat/{enc}/advance-turn` mounted
  at main.go:244.
- Workspace at main.go:237. Campaign pause/resume at main.go:610.
- `turn-queue` is mounted, but via `combat.Handler.RegisterRoutes` →
  `RegisterLegendaryRoutes` (handler.go:58, line 22) — NOT
  `mountCombatDashboardRoutes` as the worklog states. Endpoint exists in
  production; worklog citation is misleading but the route is live.

## Verification

- `cd dashboard/svelte && npm test`: 327/327 pass. Worklog claimed
  "336 (300→336, +36)"; actual baseline was 300, post-change is 327 (+27).
  Likely `messageplayer.test.js` baseline overlapped 9 existing cases.
  Functional impact: none — all tests pass.
- `npm run build`: clean (pre-existing a11y warnings).
- `make build`: clean.
- `make test`: no FAIL lines.

## Verdict

G-UI bundle: APPROVED. One follow-up: dedicated
`/api/homebrew/class-features` backend route remains a Go-side TODO.
