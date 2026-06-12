# Usability Review — 2026-06-10

Scope: the full user journey — onboarding (DM gets the bot online), campaign
setup (DM reaches a playable encounter), player onboarding (character creation
and approval), and live play (the slash-command loop). Method: four parallel
read-only code audits, one per journey stage, with every claim verified against
the code at the cited `file:line`. Line numbers reflect commit `4f90ea2`.

## Fix tracker (start here)

**How to use.** "Fix next item" = take the **first unchecked `[ ]` box from the
top**. Implement per the linked detail's **Fix** note, following the repo's
red/green TDD rule (`CLAUDE.md`). When done: flip `[ ]`→`[x]`, append `→ <short-sha>`,
bump the Progress count, and stop for review. IDs (`T01`…) are stable — the user
may also say "fix T17" to jump. Items within one group may be batched into a
single PR when each is trivial (noted per group).

**Legend:** 🔴 blocker · 🟠 major · 🟡 minor · 🔵 validation · Progress: **43 / 52**

#### Tier 1 — Blockers (unblock the documented happy path)
- [x] **T01** · 🔴 · Accept number→letter `position_col` in `ParseTemplateCreatures` + dashboard-shaped JSON test · _Finding 1_ → `d26ba13`
- [x] **T02** · 🔴 · Wrap portal routes with `RedirectNavigationOnUnauth` + carry return-to state back to `/portal/create?token=…` · _Finding 2_ → `c8e453d`

#### Tier 2 — Async-play premise
- [x] **T03** · 🟠 · Real `<@id>` turn mentions in `sendTurnNotifications` (NPC turns keep plain name) · _Finding 3_ → `cff16af`

#### Tier 3 — Silent-failure cluster (_Finding 6_)
- [x] **T04** · 🟠 · Add channel-binding check to `discordcheck` ("run /setup") + persist dm-queue items even when channel unresolved · _Finding 6 ·a_ → `a349d6d`
- [x] **T05** · 🟠 · ERROR / refuse boot when `DISCORD_BOT_TOKEN` set but `DATABASE_URL` unset · _Finding 6 ·b_ → `f910f17`
- [x] **T06** · 🟠 · Fail (not skip) the app-id check when a bot token is configured · _Finding 6 ·c_ → `72d4e4e`
- [x] **T07** · 🟠 · Set `Identify.Intents` (GuildMembers) **before** `Open()` + add intent check to `discordcheck` · _Finding 6 ·d_ → `1950d09`
- [x] **T08** · 🟠 · Surface wrong-length `TOKEN_ENCRYPTION_KEY` in checks / refuse boot when `COOKIE_SECURE=true` · _Finding 6 ·e_ → `1f960e2`
- [x] **T09** · 🟠 · Log + dm-queue notice on combat-map render failure · _Finding 6 ·f_ → `8600b76`

#### Tier 4 — Lost-work cluster (_Finding 4_)
- [x] **T10** · 🟠 · Map token errors → 400/409/410 with "request a new link"; key localStorage draft by user+campaign · _Finding 4 ·a_ → `c401587`
- [x] **T11** · 🟠 · Hydrate builder from the player's pending / changes_requested character (or persist draft server-side) · _Finding 4 ·b_ → `e82b4b2`
- [x] **T12** · 🟠 · Confirm-on-navigate when dirty + `beforeunload` handler (map editor & encounter builder) · _Finding 4 ·c_ → `bdd21f7`
- [x] **T13** · 🟠 · Run `ActivePlayerCharacter` check **before** creating the record / redeeming the token · _Finding 4 ·d_ → `cf3b1e7`

#### Tier 5 — Rules correctness
- [x] **T14** · 🟠 · Spell budget: exclude cantrips from cap + per-class cantrips-known + known-spells tables; merge subrace ASI into `finalScores()` w/ server validation · _Finding 5_ → `35931b9` (spell budget) + `58fbcd4` (subrace ASI)

#### Tier 6 — Ergonomics (_Findings 7–12_)
- [x] **T15** · 🟠 · Discord option autocomplete / case-insensitive name→slug + "did you mean" for items & spells · _Finding 7_ → `34f551d`
- [x] **T16** · 🟠 · Split `/help` reply via `SplitMessage`; stop discarding the `InteractionRespond` error · _Finding 8_ → `c6c2b11`
- [x] **T17** · 🟠 · Seat PCs into `player` spawn zones at StartCombat (reuse `AssignPCsToSpawnZones`) · _Finding 9_ → `6748335`
- [x] **T18** · 🟠 · `/done`: respond/defer immediately after `AdvanceTurn`, post notifications + map async · _Finding 10_ → `876009e`
- [x] **T19** · 🟠 · Post a debounced fogged map render after exploration moves (or mirror to `#the-story`) · _Finding 11_ → `9724d8a`
- [x] **T20** · 🟠 · Explicit per-DM active-campaign selector + guild dropdown (or make `/setup` the single creation path) · _Finding 12_ → `baa6aab`

#### Tier 7 — Other majors (stage detail) — _may need own PRs_
- [x] **T21** · 🟠 · Encounter no-map dead-end: disable/validate the "-- No map --" option inline before save · _Campaign setup_ → `28bc880`
- [x] **T22** · 🟠 · Rejection feedback in `/character` + channel/ephemeral fallback when DM send fails · _Player onboarding_ → `08d2b74`
- [x] **T23** · 🟠 · Add an out-of-combat branch to `CastHandler.Handle` (skip turn lookup, permit rituals/utility) · _Live play_ → `d6680dd`
- [x] **T24** · 🟠 · Shop buy/claim buttons + gold deduction (mirror `loot_claim:` flow) · _Live play_ → `b896b40`
- [x] **T25** · 🟠 · `/give` posts publicly + DMs the receiver + adjacency/range check · _Live play_ → `4f795ac`
- [x] **T26** · 🟠 · Markdown setup doc (or publish HTML via Pages) mirroring `setup.html` · _Onboarding_ → `1ee5072`

#### Tier 8 — Minors (stage detail) — _batchable per stage_
- [x] **T27** · 🟡 · Quickstart DB step: `createdb -h localhost -U postgres dndnd_playtest` + password note · _Onboarding_ → `7813dfd`
- [x] **T28** · 🟡 · Map close code 4004 → "check DISCORD_BOT_TOKEN"; reorder troubleshooting row · _Onboarding_ → `e8a7f83`
- [x] **T29** · 🟡 · Resolve privileged-intents contradiction (quickstart vs setup.html vs code) · _Onboarding_ → `5c6a077`
- [x] **T30** · 🟡 · Quickstart drift: Go `1.25.7`, real boot line ("server starting") · _Onboarding_ → `ab8db8d`
- [x] **T31** · 🟡 · Passthrough-ownership callout + warning log when passthrough creates campaign rows · _Onboarding_ → `e44a754`
- [x] **T32** · 🟡 · README/fly polish: `make run` env prereqs, mention `make local-up`, rename `fly.toml` app · _Onboarding_ → `a7d60ab`
- [x] **T33** · 🟡 · Map editor: disable creation without a campaign + map the 400 to "create/select a campaign first" · _Campaign setup_ → `7ba4159`
- [x] **T34** · 🟡 · Next-step links in `/setup` success + dashboard Home (dashboard URL, "next: build a map") · _Campaign setup_ → `645ded0`
- [x] **T35** · 🟡 · Align `/setup` gate (Manage Channels exposure vs Administrator requirement) · _Campaign setup_ → `3401438`
- [x] **T36** · 🟡 · Stat-block library: filter by campaign, surface load errors, reword the empty state · _Campaign setup_ → `536f693`
- [x] **T37** · 🟡 · Submit errors: scroll into view / inline near button; humanize raw body text · _Player onboarding_ → `e190a5d`
- [x] **T38** · 🟡 · Unmet-requirements checklist next to Submit; gate Review on rolled scores · _Player onboarding_ → `015845d`
- [x] **T39** · 🟡 · Distinguish spell load-error (with retry) from genuinely empty; skip Spells step for non-casters · _Player onboarding_ → `1c78183`
- [x] **T40** · 🟡 · Mobile: collapse `.skill-grid` / `.class-row` to one column under 480px · _Player onboarding_ → `8d92fa0`
- [x] **T41** · 🟡 · Loot claim shows `<@id>` / character name, not a raw snowflake · _Live play_ → `1a7eb77`
- [x] **T42** · 🟡 · "Not your turn" hides the raw user ID + gives a next step · _Live play_ → `197ba29`
- [x] **T43** · 🟡 · Route both move-confirm variants through the same write/notify body (turnGate, log, snapshot, OA) · _Live play_ → `21caa4e`
- [ ] **T44** · 🟡 · Fix in-Discord help drift (`/reaction` subcommands, metamagic `twin`, `option:value` examples) · _Live play_
- [ ] **T45** · 🟡 · `/status` shows HP/position (or correct the `/help status` text) · _Live play_
- [ ] **T46** · 🟡 · Gated `/check` echoes the raw roll; add resolve buttons to dm-queue messages · _Live play_

#### Tier 9 — Documentation drift — _batchable into one PR_
- [ ] **T47** · 🟡 · Fix `#dm-private` → `#dm-queue` in quickstart · _Doc drift_
- [ ] **T48** · 🟡 · Refresh stale quickstart UI walkthrough (Maps → + New Map, Start Combat, no drag-to-place) · _Doc drift_
- [ ] **T49** · 🟡 · Fix checklist syntax the bot rejects (`/cast` target, `/give` target) · _Doc drift_
- [ ] **T50** · 🟡 · `usage.html` option drift (`/action args`, `/check target`, `/prepare subclass`); consider a generator · _Doc drift_
- [ ] **T51** · 🟡 · Correct `usage.html` `/status` claim (combat-state only) · _Doc drift_

#### Tier 10 — Validation gate
- [ ] **T52** · 🔵 · Run `docs/playtest-checklist.md` end-to-end and record a transcript (do **after** T49) · _Recommended order #7_

## TL;DR

The core combat loop (`/move` → confirm → `/done`) is solid and has the best
error messages in the project. But the journey to reach it is full of traps,
and **two blockers sit on the documented happy path** (encounter-builder
creature placement breaks Start Combat; the player portal link dead-ends at a
raw 401 on OAuth deployments). The biggest systemic problem is **silent
failure**: the bot looks healthy while slash commands, channel posts, queue
items, and notifications quietly vanish.

A telling meta-signal: all 11 scenarios in `docs/playtest-checklist.md` are
still `Status: pending`, and the quickstart's self-test log reads "_pending
first self-test_". The product has never survived a recorded live session, and
many findings below are exactly what that run would catch.

---

## Fix-first list (ranked)

### 1. BLOCKER — The builder's own hint breaks Start Combat

The encounter builder tells the DM to "Drag creatures from the list onto the
map to place them". Doing so writes `position_col` as a number
(`dashboard/svelte/src/EncounterBuilder.svelte:340`, from `canvasTileCoords`
at :315-323), but the backend expects a letter string
(`internal/combat/domain.go:44`, `colToIndex` at
`internal/combat/attack.go:1819`). Start Combat then fails with a raw 500:

> creating encounter from template: parsing template creatures: json: cannot
> unmarshal number into Go struct field TemplateCreature.position_col of type
> string

surfaced at `internal/combat/service.go:879-882` /
`internal/combat/handler.go:243-246`. No recovery hint; the only workaround is
to never place creatures. The e2e harness never exercises this path — it seeds
combatants directly with `"A",1` (`cmd/dndnd/e2e_harness_test.go:306`).

**Fix:** accept both formats in `ParseTemplateCreatures` (normalize
number→letter) and add a dashboard-shaped JSON test.

### 2. BLOCKER (production deploys) — Portal link dead-ends at raw 401

A first-time player clicks the `/create-character` link with no session cookie
and gets plain-text "unauthorized" — no login link, no redirect. If they find
`/portal/auth/login` themselves, the OAuth callback hard-redirects to `/`
(`internal/auth/oauth2.go:191`) → `/dashboard/` (`cmd/dndnd/main.go:1771-1774`)
→ 403 for non-DM players; the token URL is lost and they must re-click in
Discord. The redirect-to-login wrapper exists but is applied only to dashboard
mounts (`cmd/dndnd/main.go:1167`), not the portal mount (`:1330`); the bare 401
is even the tested behavior (`internal/portal/routes_test.go:73-88`). Masked in
local dev because passthrough auth kicks in when OAuth env vars are unset
(`cmd/dndnd/main.go:584-587`), so playtests never see it.

**Fix:** wrap portal page routes with
`auth.RedirectNavigationOnUnauth("/portal/auth/login", authMw)` and carry a
return-to cookie/state so the callback lands back on the original
`/portal/create?token=...` URL.

### 3. MAJOR — Turn-start "ping" is fake text; players are never notified

When a turn starts, `#your-turn` gets `🔔 @Aria — it's your turn!` where
`@Aria` is the character name as plain text — no Discord mention fires, no
notification badge (`internal/combat/turnresources.go:326`,
`internal/combat/impact_summary.go:47`; same in the initiative tracker,
`internal/combat/initiative.go:220`). Async play stalls until someone happens
to look. The correct pattern (`<@%s>` with `pc.DiscordUserID`) already exists
in `internal/discord/resume_turn_pinger.go:95`, and `done_handler.go` holds a
`playerLookup` capable of resolving the ID but uses it only for authorization
(`done_handler.go:299-307`).

**Fix:** resolve the next combatant's `DiscordUserID` in
`sendTurnNotifications` and emit a real `<@id>` mention (NPC turns keep the
plain name).

### 4. MAJOR — Lost-work cluster

- **Expired/used token at submit → 500 and the build is gone.** The error
  `validating token: portal token expired/already used` doesn't match the
  `validation` prefix check (`internal/portal/api_handler.go:210-222, 252-254`;
  wrap at `internal/portal/builder_service.go:489-491`), so the player sees
  "Submission failed: internal server error" with no next step. Re-running
  `/create-character` mints a new token, but the localStorage draft is keyed
  to the dead token (`portal/svelte/src/lib/builder-draft.js:53-56`,
  `CharacterBuilder.svelte:31`) — the new builder page is blank. Commit
  089b18e fixed only the *empty* token/campaign case.
  **Fix:** map `ErrTokenExpired/ErrTokenUsed/ErrTokenNotFound/ErrTokenOwnership`
  to 400/409/410 with "request a new link with /create-character" text; key
  the draft by user+campaign instead of token.
- **"Request changes" forces a full rebuild from a blank form.** The player
  must re-run `/create-character`
  (`internal/discord/registration_handler.go:453,457`), gets a fresh token and
  an empty builder (draft cleared on submit,
  `CharacterBuilder.svelte:525-537`). No prefill from the pending character,
  even though the pending row is relinked on resubmit
  (`internal/portal/builder_service.go:558-580`).
  **Fix:** hydrate the builder from the player's pending/changes_requested
  character, or persist the draft server-side keyed by user+campaign.
- **Map editor and encounter builder can lose work on navigation.** Both track
  `dirty` and show "*unsaved" (`MapEditor.svelte:39,1053-1055`;
  `EncounterBuilder.svelte:28,581-583`), but `App.svelte`
  `onBack`/`navigateTo` (`App.svelte:79-82,136-153`) discard state without
  confirmation and there is no `beforeunload` handler anywhere in
  `dashboard/svelte/src`. A new map isn't persisted until the first manual
  Save (`MapEditor.svelte:153-199`).
  **Fix:** confirm-on-navigate when dirty plus a `beforeunload` handler.
- **Token redeemed before the already-active check.** Order in `create()`:
  create character record → redeem token → check existing approved row →
  `ErrAlreadyActive` (`internal/portal/builder_service.go:498-514, 558-574`).
  A player approved mid-build sees the 409, retries, and now hits "token
  already used" → 500.
  **Fix:** run the `ActivePlayerCharacter` check before creating the
  record/redeeming the token.

### 5. MAJOR — Spell-budget math blocks standard 5e builds

One combined cap of `mod + level` is applied to cantrips *and* leveled spells,
for every caster class. A L1 wizard with INT 16 gets 4 total picks; RAW is
3 cantrips + 4 prepared = 7. Known-casters (bard/sorcerer/warlock/ranger) also
get `mod + level` instead of their known-spells table. Client:
`portal/svelte/src/lib/spell-picker.js:21-24` (`countAgainstCap` counts
level 0), `lib/spellcasting.js:49-52`, hint at
`CharacterBuilder.svelte:1066-1071`; the server mirrors it via `len(s.Spells)`
in `internal/portal/builder_service.go:104-133`.

Related: **subrace ability bonuses are advertised but never applied.** Picking
High Elf shows "+1 INT" (`CharacterBuilder.svelte:686-699`), but
`finalScores()` only adds the base race's bonuses
(`CharacterBuilder.svelte:422-441`) and the server stores submitted scores
as-is (`internal/portal/builder_service.go:471`; no subrace handling in
`internal/portal/derive_stats.go`).

**Fix:** exclude cantrips from the cap, add a per-class cantrips-known budget
(server + picker), use known-spells tables for known-casters, and merge
`subracePerks(...).abilityBonuses` into `finalScores()` with matching
server-side validation.

### 6. MAJOR — Silent-failure cluster

The health banner shows green while core functionality is off:

- **Missing channel bindings fail silently — dm-queue items aren't even
  persisted.** If `/setup` never ran in the guild (e.g. campaign created via
  the dashboard form) or a channel was deleted: combat-log/combat-map/turn
  posts are skipped (`internal/discord/combat_log.go:35-38`,
  `done_handler.go:481`), registration notices dropped
  (`internal/discord/registration_handler.go:403-405`), and `dmqueue.Post`
  returns **before** `store.Insert` when no `#dm-queue` resolves
  (`internal/dmqueue/notifier.go:163-167`, resolver at
  `cmd/dndnd/main.go:329-341`) — whisper/undo/rest/check requests vanish from
  the dashboard queue too. The checks banner verifies token/app/guild but
  never channels (`internal/discordcheck/checks.go:90-169`).
  **Fix:** add a channel-binding check to discordcheck ("run /setup"); persist
  dm-queue items even when the channel is unresolved.
- **`DISCORD_BOT_TOKEN` set without `DATABASE_URL`: bot never opens,
  silently.** `Open()` lives inside the `if databaseURL != ""` block
  (`cmd/dndnd/main.go:839-841`, `:1471`); the user sees "discord session
  constructed (open deferred until after recovery)" (`main.go:807`) and the
  bot stays offline forever. Hits `make run` users (README suggests it with
  zero env guidance) and quickstart users who open a new shell.
  **Fix:** log an ERROR or refuse to boot when the token is set but
  `DATABASE_URL` is not.
- **Missing `DISCORD_APPLICATION_ID` degrades silently with a green
  banner.** Per-guild command registration fails (per-guild errors at
  `internal/discord/bot.go:84-86`, count-only WARN at `main.go:1708-1710`),
  guild permission validation no-ops (`internal/discord/permissions.go:50-67`
  via `bot.go:115`), and the `application-id-match` check reports OK "skipped
  (env not set)" (`internal/discordcheck/checks.go:128-135`). Bot online, no
  slash commands, green checks.
  **Fix:** fail (not skip) the app-id check when a bot token is configured.
- **Server Members intent is set after the gateway opens.** `rawDG.Open()`
  runs at `cmd/dndnd/main.go:1471-1477`; `wireBotHandlers` ORs
  `IntentsGuildMembers` into `Identify.Intents` only afterwards
  (`main.go:1700` → `cmd/dndnd/discord_handlers.go:99-101`). The boot-time
  validation promised by `docs/setup.html:134-137` never happens; with the
  portal toggle ON, welcome DMs silently never arrive on the first session;
  failures surface later as a cryptic close 4014.
  **Fix:** set `Identify.Intents` before `Open()` and add an intent check to
  `internal/discordcheck`.
- **Wrong-length `TOKEN_ENCRYPTION_KEY` silently downgrades to plaintext
  token storage.** One ERROR log, then continues unencrypted
  (`cmd/dndnd/main.go:599-606`); not surfaced in the checks banner. Anyone
  running `openssl rand -hex 32` (64 chars) hits it.
  **Fix:** surface in the discord-checks report or refuse to boot when
  `COOKIE_SECURE=true`.
- **Combat-map render failures are swallowed.** `PostCombatMap` ignores errors
  by design (`done_handler.go:486-489`); players target off a stale PNG and
  nobody is told.
  **Fix:** log + dm-queue notice on render failure.

### 7. MAJOR — Slug-typing hell: exact IDs required, no autocomplete

`/inventory` shows "Potion of Healing ×2" (display names only,
`internal/inventory/service.go:261-277`), but `/use item:Potion of Healing`
fails with `Cannot use item: item "Potion of Healing" not found in inventory` —
the required slug `healing-potion` is shown nowhere in Discord. Exact match at
`internal/inventory/service.go:36-43`; same for `/equip` (`equip.go:30`),
`/attune` (`attunement.go:40`), `/give`, and `/cast spell:Fire Bolt` →
`Spell "Fire Bolt" not found.` (`internal/discord/cast_handler.go:278`). No
`Autocomplete: true` anywhere in `internal/discord/commands.go`.

**Fix:** add Discord option autocomplete (inventory/spell lists are small and
per-character), or at minimum case-insensitive name→slug matching plus "did
you mean / valid IDs:" in the error. `ResolveTarget`
(`internal/combat/distance.go:29-56`) is the in-repo model: case-insensitive
short IDs or grid coordinates.

### 8. MAJOR — `/help` (no topic) fails exactly during live play

`generalHelp` is 1942 unicode chars; `help_handler.go:40` appends context tips
when the user is in an encounter → 2219 chars (combat) / 2192 (exploration),
over Discord's 2000-char content limit. The send error is discarded
(`internal/discord/router.go:684-692`, `_ = s.InteractionRespond(...)`), so
the player just sees "The application did not respond".

**Fix:** split the reply (reuse `SplitMessage` from `message.go`) or trim
`generalHelp`; stop discarding the `InteractionRespond` error.

### 9. MAJOR — PCs pile up at the top-left tile; spawn zones ignored by combat

The builder always sends `character_positions: {}`
(`EncounterBuilder.svelte:385`), so each PC gets a zero-value `Position` →
col "", row 0 (`internal/combat/service.go:1008-1010`). The `spawn_zones` the
DM painted are consumed only by exploration mode
(`internal/exploration/service.go:68-86`). Recovery is manual per-combatant
position override in CombatManager (`CombatManager.svelte:217-218`).

**Fix:** seat PCs into `player` spawn zones at StartCombat (reuse
`exploration.AssignPCsToSpawnZones`) or add PC drag placement to the builder.

### 10. MAJOR — `/done` acknowledges only after rendering + uploading the map

`h.sendTurnNotifications` (`done_handler.go:392`) — which calls
`PostCombatMap` → `RegenerateMap` PNG render + `ChannelMessageSendComplex`
file upload (`done_handler.go:466,486-497`) — executes *before*
`respond(msg)` at `done_handler.go:401`. On large/Tiled-asset maps this blows
Discord's 3-second interaction window: the player sees "The application did
not respond" even though the turn advanced, retries `/done`, and gets "It's
not your turn". No deferred response is used in the /done path (only
rest/asi/setup defer).

**Fix:** respond (or defer) immediately after `AdvanceTurn`, then post
notifications/map asynchronously.

### 11. MAJOR — Exploration mode is invisible to players

In exploration, `/move D4` is ephemeral-only (`🏃 Moved to D4.`) — no map PNG
is ever posted to Discord, fog-of-war reveal is invisible, and other party
members see nothing (`internal/discord/move_handler.go:593-672`; the only map
posters in the codebase are `done_handler.go:466` and
`enemy_turn_notifier.go:69`, both combat-only). Only the DM (dashboard) can
see positions. Players navigate blind.

**Fix:** post a fogged map render to a channel after exploration moves
(debounced), or at least mirror moves to `#the-story`.

### 12. MAJOR — No campaign switcher; dashboard binds to newest campaign

`LookupActiveCampaign` returns the first owned match of `ListCampaigns`
ordered `created_at DESC` (`cmd/dndnd/main.go:148-163`,
`db/queries/campaigns.sql:27-28`). Creating any second campaign (test, typo
retry) instantly flips Maps/Encounters/Party context for the whole dashboard,
indicated only by a small "Active" badge on the Campaigns page
(`CampaignsPage.svelte:91-104`); the only way back is archiving.

Compounding it: the dashboard campaign form requires hand-typing the Guild ID
with no validation that the bot is in that guild
(`CampaignsPage.svelte:70-79`, `internal/dashboard/campaigns.go:100-105`),
while `/setup` auto-creates its own row (`cmd/dndnd/discord_adapters.go:148-176`)
— two creation paths, no reconciliation; a typo'd guild ID creates an orphan
that hijacks the dashboard.

**Fix:** explicit active-campaign selector persisted per DM; replace the
free-text guild field with a dropdown of guilds the bot is in (or make
`/setup` the single creation path).

---

## Stage detail

### Onboarding (clone → bot online)

Step counts: Docker Compose path ~13 manual steps; bare-binary quickstart
~18-20. Riskiest steps: the Server Members intent toggle (not actually
validated at boot — see finding 6), the 4-credential paste (missing app ID is
silent — finding 6), keeping `DATABASE_URL` exported in the same shell
(finding 6), and `createdb` against the suggested docker Postgres (below).

- **MAJOR — The primary setup guide is unreadable where users find it.**
  `README.md:9-14` routes to `docs/setup.html`; GitHub renders `.html` as raw
  source (231 lines of markup/CSS) and there is no Pages workflow
  (`.github/workflows/` has only `test.yml`). README has no inline
  clone→online steps; the markdown fallback (`docs/local-run.md`) skips the
  entire Discord-app section.
  **Fix:** a markdown setup doc (or README quick-start) mirroring setup.html,
  or publish the HTML via Pages.
- **MAJOR — Quickstart DB step fails against its own suggested Postgres.**
  `docs/playtest-quickstart.md:22` suggests dockerized Postgres, then step 2
  (`:51`) says bare `createdb dndnd_playtest`, which uses unix-socket peer
  auth and fails.
  **Fix:** `createdb -h localhost -U postgres dndnd_playtest` with the
  password note.
- **MINOR — Bad token = crash loop with a misdiagnosing troubleshooting
  row.** A bad token exits via `main.go:1472-1474` → `os.Exit(1)`
  (`main.go:701-703`); under Compose `restart: unless-stopped`
  (`docker-compose.yml:44`) the app crash-loops, re-running migrations+seed
  each pass. `setup.html:214` tells users the first fix is the Members intent
  toggle — the wrong place.
  **Fix:** map close code 4004 to "check DISCORD_BOT_TOKEN"; reorder the
  troubleshooting row.
- **MINOR — Docs contradict each other on privileged intents.**
  `playtest-quickstart.md:64-65` instructs enabling Message Content Intent;
  `setup.html:137` says it's not required; code only requests
  `IntentsGuildMembers` (`discord_handlers.go:100`).
- **MINOR — Quickstart drift.** Go version "1.22+" vs `go.mod:3` requiring
  `go 1.25.7`; promised boot line `http server listening addr=:8080` doesn't
  exist (actual: "server starting", `main.go:1741`).
- **MINOR — Passthrough-identity ownership trap.** In no-OAuth mode everyone
  is `DEV_DISCORD_USER_ID` (default `local-dev`, `main.go:346-354`);
  campaigns created then are owned by `local-dev`, and after enabling real
  OAuth the DM fails `IsDM` (`main.go:148-179`) — campaigns vanish/403.
  **Fix:** setup.html callout + warning log when passthrough creates campaign
  rows.
- **MINOR — README/fly polish.** `README.md:44-47` offers `make run` with no
  env prerequisites (yields the half-dead no-DB mode) and never mentions
  `make local-up`; `fly.toml:1` hard-codes `app = "dndnd"` (name taken) and
  `local-run.md:66-90` never says to rename it.

### Campaign setup (bot online → playable encounter)

Minimal happy path: `/setup` (Discord, admin) → dashboard OAuth login →
players `/register` + DM approves → Maps → + New Map (paint or Tiled import)
→ Encounters → + New Encounter (must pick a map; must NOT drag creatures —
finding 1) → Start Combat → first `/move`. Two mandatory surfaces (three with
Tiled), ~4 context switches. The must-just-know cliffs: `/setup` existence,
the multi-file Tiled selection rule, the map-required rule, and the placement
bug.

- **MAJOR — Encounter save dead-end on the default map option.** Builder
  defaults to "-- No map --" (`EncounterBuilder.svelte:465-471`); create then
  fails with "map_id is required for encounter templates"
  (`internal/encounter/service.go:55-57`) — only after the DM built the whole
  encounter. Update has no such check (`service.go:94-112`).
  **Fix:** remove/disable the no-map option for new encounters, or validate
  inline before save.
- **MINOR — Map editor reachable with no campaign; fails only at Save.**
  `App.svelte:206-208` renders the editor with `campaignId=''`; `+ New Map`
  always enabled (`MapList.svelte:45`) even when the list says "No active
  campaign selected"; backend 400 at `internal/gamemap/handler.go:195-198`.
  **Fix:** disable creation without a campaign; map the error to
  "create/select a campaign first."
- **MINOR — No next-step guidance at the two key hand-offs.** `/setup`
  success says only "Campaign created and channel structure set up!"
  (`internal/discord/setup.go:259-263`) — no dashboard URL, no "next: build a
  map". Dashboard Home empty state (`HomePanel.svelte:72-76`) doesn't mention
  `/setup`. The required order (setup → approve → map → encounter → start)
  exists only in docs.
  **Fix:** links/next-steps in both messages; consider a Home setup
  checklist.
- **MINOR — `/setup` visibility vs requirement mismatch.** Exposed to anyone
  with Manage Channels (`internal/discord/commands.go:48-49,624-626`), but
  first-run auto-create demands Administrator (`setup.go:237-240`).
  **Fix:** align the gate or explain the requirement in the description.
- **MINOR — Stat block library is global and its empty state misleads.**
  `ListCreatures` has no campaign filter (`db/queries/creatures.sql:4-5`), so
  homebrew from campaign A appears in campaign B. On API failure the builder
  swallows the error and shows "No creatures in library. Import stat blocks
  first." (`EncounterBuilder.svelte:132-139,490-492`) — but no "import stat
  blocks" feature exists (creatures come from SRD seed, Homebrew, or Open5e).
  **Fix:** filter by campaign, surface load errors, reword the empty state.

### Player onboarding & character creation

Happy path: welcome DM (missed if DMs closed; `/help` is the fallback) →
`/register` chooser or `/create-character` → ephemeral 24h single-use link →
7-step builder (draft auto-saved per token) → Submit → `#dm-queue` ping → DM
approves in dashboard → player DM'd + public card in `#character-cards`.

Beyond findings 2, 4, 5 above:

- **MAJOR — Rejection feedback is DM-only and silently droppable.**
  `NotifyChangesRequested`/`NotifyRejection` failures are merely logged
  (`internal/dashboard/approval_handler.go:298-302, 321-325`); no channel
  fallback. `/character` shows only the bare status word ("currently
  **changes_requested**") without the DM's feedback text
  (`internal/discord/character_handler.go:60-63`), and the notifier DM omits
  the "how to resubmit" hint (`cmd/dndnd/discord_adapters.go:206-225`).
  **Fix:** include feedback + next step in `/character`; fall back to an
  ephemeral/channel ping when the DM send fails.
- **MINOR — Submit errors render off-screen as raw server text.** Error div
  sits above the step content (`CharacterBuilder.svelte:623-625`); after
  clicking Submit at the bottom of the long Review page, a failure can be
  invisible ("button did nothing"). Text is the raw body, e.g.
  `validation failed: STR roll must include four d6 results; token is required`
  (`portal/svelte/src/lib/api.js:13-19`).
  **Fix:** scroll error into view / inline near the button; humanize.
- **MINOR — Weak pre-submit gating.** Submit is disabled only on
  `!name || !race || !selectedClass` with no reason shown
  (`CharacterBuilder.svelte:1220`); skill under-selection passes silently;
  choosing the Roll method and never rolling leaves default 8s, failing only
  server-side (`internal/portal/builder_service.go:776-781`).
  **Fix:** unmet-requirements checklist next to Submit; gate Review on rolled
  scores.
- **MINOR — Spell-list load failures masquerade as "not a spellcaster".**
  `loadSpells` catches and sets `spells = []`
  (`CharacterBuilder.svelte:217-223`); a cleric on a flaky network sees "No
  spells available for your class, or your class is not a spellcaster."
  (`:1063-1064`) and may submit a caster with zero spells. Non-casters still
  click through the Spells step.
  **Fix:** distinguish load-error (with retry) from genuinely empty; skip the
  step for non-casters.
- **MINOR — Mobile.** The shell's only media query restyles the header
  (`internal/portal/handler.go:273-276`); `.skill-grid` is fixed 2-column
  (`CharacterBuilder.svelte:1278`) and `.class-row` is 4-column
  `1fr 80px 1fr 32px` (`:1355-1357`) — cramped under ~400px, where most
  players (phone + Discord) will be.
  **Fix:** collapse those grids to one column under 480px.

### Live play

Beyond findings 3, 7, 8, 10, 11 above:

- **MAJOR — `/cast` is impossible out of combat, and `ritual` is a dead
  end.** Exploration: `/cast spell:light` → "No active turn."; no encounter →
  "You are not in an active encounter."
  (`internal/discord/cast_handler.go:245-260`). In combat, rituals are
  rejected: "cannot cast rituals during active combat"
  (`internal/combat/spellcasting.go:1304`). So `ritual:true` ("out of combat
  only" per its own description, `commands.go:200`) can never succeed (only
  `identify`/`detect-magic` work via an inventory bypass,
  `cast_handler.go:239`).
  **Fix:** add an exploration/out-of-combat branch to `CastHandler.Handle`
  (mirroring `/move`'s exploration branch) that skips the turn lookup and
  permits rituals/utility casts.
- **MAJOR — Shops are announce-only.** `HandlePostToDiscord` sends a text
  announcement (`internal/shops/handler.go:233-272`); there is no `/buy`, no
  claim buttons, no purchase flow (no Sell/Buy/Purchase in
  `internal/shops/service.go`; no shop component handler in
  `internal/discord/router.go`). The DM hand-edits inventory and gold per
  transaction.
  **Fix:** claim/buy buttons on the shop post (mirroring the `loot_claim:`
  flow) with gold deduction.
- **MAJOR — `/give` is silent.** Only the giver's ephemeral confirmation is
  sent (`internal/discord/give_handler.go:191`); the receiver is never
  notified and nothing is posted publicly. Also no adjacency/range check —
  anyone can give to anyone across the map mid-combat.
  `docs/playtest-checklist.md:148-150` expects a DM-accept prompt and a
  `#the-story` post that don't exist.
  **Fix:** post the transfer to `#the-story`/`#combat-log` and DM the
  receiver.
- **MINOR — Loot claim prints a raw snowflake.** `#the-story` shows
  `💰 123456789012345678 claimed **Potion of Healing**!`
  (`internal/discord/loot_handler.go:187` — neither `<@id>` nor the character
  name, which is available via the claim's custom ID).
- **MINOR — "Not your turn" exposes a raw user ID.**
  `It's not your turn. Current turn: **Aria** (@123456789012345678)`
  (`internal/combat/turnvalidation.go:21`); the DB-level fallback "Failed to
  validate turn ownership." (`internal/discord/turnguard.go:85`) gives no
  next step.
- **MINOR — Prone Stand&Move/Crawl confirm bypasses the standard move
  pipeline.** `HandleMoveConfirmWithMode` (`move_handler.go:1523-1600`) has
  no `turnGate`, no combat-log mirror, no snapshot publish, and no
  opportunity-attack detection — all present in `HandleMoveConfirm`
  (`move_handler.go:697-805`).
  **Fix:** route both confirm variants through the same write/notify body.
- **MINOR — In-Discord help drift.** `/help rogue` prints the untypeable
  `/reaction uncanny-dodge` (`help_content.go:192` vs subcommands
  `declare|cancel|cancel-all`, `commands.go:313-348`); `/help metamagic`
  advertises `--twinned` vs registered option `twin` (`help_content.go:250`
  vs `commands.go:159`); examples use CLI `--flag` style throughout instead
  of Discord's `option:value` (`help_content.go:101-126,243-258`).
- **MINOR — `/status` doesn't show HP/position despite help promising it.**
  `/help status` says "Show your current HP, conditions, resources, and
  position" (`help_content.go:398-401`), but `status.Info` has no HP/position
  fields (`internal/status/format.go:14-46`;
  `status_handler.go:104-130` never reads HpCurrent/PositionCol).
- **MINOR — Gated checks hide the player's own roll; queue is
  dashboard-only.** `/check perception` often returns only "🎲 Check rolled —
  result sent to the DM for narration." (`check_handler.go:371-376`); every
  `#dm-queue` item is plain text with a dashboard link and no Discord-side
  buttons (`internal/dmqueue/notifier.go:163-183`) — a mobile/Discord-only DM
  can't resolve anything.
  **Fix:** echo the raw roll ("you rolled 17 — outcome pending DM"); add
  resolve buttons to queue messages.

---

## Documentation drift summary

- `#dm-private` doesn't exist; the real channel is `#dm-queue`
  (`internal/discord/setup.go:25-58`, resolver `cmd/dndnd/main.go:329-342`).
  Wrong in `docs/playtest-quickstart.md:122-123,146-148`.
- Quickstart UI walkthrough is stale: "Maps → Upload" is actually Maps →
  + New Map → "Import Tiled (.tmj + images)" (`MapEditor.svelte:948-965`);
  "drag the player onto the grid" is impossible (finding 9); "Go Live" button
  is "Start Combat" (`EncounterBuilder.svelte:569-574`).
- `docs/playtest-checklist.md` uses syntax the bot rejects:
  `/cast spell:burning-hands target:cone-from-here` (line 68; target must be
  coordinate/creature ID) and `/give item:potion-of-healing to:@PlayerB`
  (line 146; the option is `target` with a creature short-ID, @mentions
  unsupported).
- `docs/usage.html` covers all 35 registered commands (none missing), with
  option-level drift: `/action` omits `args`, `/check` omits `target`,
  `/prepare` omits `subclass`. Its footer claims it is "generated from
  internal/discord/commands.go" — it's hand-maintained, so drift will recur.
  Consider an actual generator.
- `docs/usage.html` bills "`/character` / `/status` — view your sheet summary
  and current status", but `/status` is combat-state only
  (`internal/discord/status_handler.go:82-96`).

## What works well (keep, and use as the bar)

- **Movement validation errors are exemplary** — quantified and actionable:
  "Not enough movement: 15ft needed, 10ft remaining"
  (`internal/combat/movement.go:127`), "You can move through allies but
  cannot end your turn in their tile (D4)" (`movement.go:90`), "❌ Target is
  out of range — Xft away (max Yft)." (`internal/combat/distance.go:24`),
  plus a Confirm/Cancel preview with cost before any move commits.
- **Turn context is rich once seen**: "⚠️ Since your last turn:" impact
  summary (`impact_summary.go:87`), "Available: Action | Bonus | 30ft move"
  resource lines, `/done` unspent-resource warnings with buttons
  (`unused_resources.go`), live-edited initiative tracker, automatic DM ping
  on NPC turns (`combat/initiative.go:829-845`).
- **`/setup` is idempotent and self-healing**: reconciles channels by name,
  persists partial progress, auto-creates the campaign row
  (`internal/discord/setup.go:125-183,242-251`). The documented permissions
  integer `2416176144` decodes exactly to the ten permissions listed.
- **`.env.example` is excellent** — thorough comments, zero env-name drift
  against `docker-compose.yml`/`fly.toml`/`main.go` (verified); `make
  local-env` copies it; `.env` gitignored. Migrations + SRD seed run
  automatically on boot.
- **Tiled import errors are strict and actionable** (embed-tileset guidance,
  missing images listed by basename, `internal/gamemap/import.go:40-49,
  563-565`), and `docs/tiled-maps.md` matches the code's rules almost
  exactly, including a troubleshooting section per error string.
- **Builder fundamentals**: localStorage draft restores all 16 fields with
  versioning and legality reconciliation; skills step shows budgets and locks
  granted skills with source tags; spell picker gives per-spell disabled
  reasons; token error pages say exactly what to do; DDB import 403 hint
  ("character may not be set to public sharing").
- **Forgiving targeting**: `ResolveTarget` accepts case-insensitive short IDs
  or grid coordinates (`combat/distance.go:29-56`); unknown `/bonus` actions
  list every valid option; router-level panic recovery always replies with a
  friendly ephemeral and logs to the DM error panel.

## Recommended order

1. Fix the two blockers (findings 1, 2) — small changes that unblock the
   documented happy path.
2. Real turn mentions (finding 3) — one function; the async-play premise
   depends on it.
3. Silent-failure cluster (finding 6): channel-binding check in the
   discordcheck banner, persist dm-queue items unconditionally, fail the
   app-id check when a token is set, set intents before `Open()`.
4. Lost-work cluster (finding 4): token errors → 4xx with "request a new
   link", draft keyed by user+campaign, builder prefill on
   changes-requested, dirty-navigation guards.
5. Spell math + subrace ASI (finding 5) — correctness, server and client.
6. Ergonomics: autocomplete/fuzzy slugs (7), `/help` split (8), spawn-zone
   seating (9), `/done` defer (10), exploration map posts (11), campaign
   switcher (12).
7. Fix the docs drift (quickstart, checklist, usage.html), then **run the
   playtest checklist end-to-end and record a transcript** — the checklist
   itself currently uses syntax the bot rejects, so the doc fix plus a live
   run will surface whatever this review missed.
