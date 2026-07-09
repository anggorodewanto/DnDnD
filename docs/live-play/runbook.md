# Runbook — operating the live game

Everything a DM-agent needs to stand up the stack, authenticate, drive DM
actions, observe state, and tear down. Pair this with `game-state.md` for the
live IDs.

## 1. Stand up the stack

The full stack (Postgres + app built from current source) comes up with:

```sh
make local-up      # docker compose up --build  (runs in foreground; background it)
```

This builds the app image from the Dockerfile (so it includes the latest combat
fixes — the prebuilt `bin/dndnd` may be stale) and starts:

- `dndnd-db-1` — postgres:16 on `localhost:5432` (user/pass/db all `dndnd`).
- `dndnd-app-1` — the bot + dashboard on `localhost:8080`.

Config comes from `.env` (see `.env.example`). Key values for this rig:
`BASE_URL=http://localhost:8080`, `SKIP_SRD_SEED=false` (SRD classes/monsters/
spells DO seed), `COOKIE_SECURE=false`, real `DISCORD_*` creds set.

### Verify it's healthy

```sh
docker compose ps                                   # both services Up; db healthy
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080   # 307 (redirect to login)
docker compose logs app | grep -E 'discord session opened|channel-bindings|server starting'
```

Healthy looks like: `discord session opened`, `discord check passed` for
`token-identity` / `guild-membership` / `channel-bindings`, `server starting addr=:8080`.

### Restart / logs / teardown

```sh
docker compose restart app            # restart just the app (after a rebuild)
docker compose logs -f app            # tail app logs
make local-down                       # stop (keeps volumes/data)
docker compose down -v                # stop + WIPE db/assets (destructive — ask first)
```

## 2. Auth model (how the DM reaches the dashboard)

- **Real Discord OAuth is active** because `DISCORD_CLIENT_ID` and
  `DISCORD_CLIENT_SECRET` are set (`cmd/dndnd/main.go:350-359`,`590-601`).
- OAuth redirect, when `OAUTH_REDIRECT_URL` is empty, derives from
  `BASE_URL + /portal/auth/callback` → `http://localhost:8080/portal/auth/callback`
  (must be registered as a redirect on the Discord app).
- **Dev bypass** (`passthroughMiddleware`, injects `DEV_DISCORD_USER_ID`) only
  activates when CLIENT_ID/SECRET are *unset* — **not** our case.

**Therefore:** the user logs into `http://localhost:8080` once (Login with
Discord), and Claude drives that **already-authenticated browser tab**
(claude-in-chrome). The logged-in user must be the campaign's `dm_user_id` to see
DM controls.

## 3. Becoming / being the DM

- A campaign is created by running **`/setup`** in the guild; the invoker becomes
  `dm_user_id` (`cmd/dndnd/discord_adapters.go:164-184`). Channels are created
  then too. *(Already done for this campaign — see `game-state.md`.)*
- The dashboard shows DM controls for the campaign whose `dm_user_id` matches the
  logged-in Discord user.

## 4. DM actions and how to drive them

Prefer driving the **dashboard SPA** (`/dashboard/app/`, hash-routed) in the
browser. The underlying HTTP endpoints are listed so you can verify/observe (or
fall back to API calls with the session cookie).

| Action | Dashboard | HTTP endpoint (cite) |
| --- | --- | --- |
| List campaign / DM home | `/dashboard/app/` | resolves DM's active campaign (`main.go:149-169`) |
| Pending character approvals | approvals view | `GET /dashboard/api/approvals/`, `POST …/{id}/approve` (`internal/dashboard/approval_handler.go:46-51`) |
| **Build a map (in-app, preferred)** | Maps → **+ New Map** (set name + W×H squares) → **Map Editor** (paint Terrain / Wall / Spawn Zone, then **Save**) | `POST /api/maps`, save edits `PUT /api/maps/{id}` (`internal/gamemap/handler.go:187,296`) |
| Import a map (Tiled .tmj) | Maps → + New Map → **Import Tiled (.tmj + images)** | `POST /api/maps/import` (`internal/gamemap/handler.go:29`) |
| Create encounter + place tokens | Encounters → New | `POST /api/encounters/` (`internal/encounter/handler.go:29`) |
| Start combat (roll initiative) | encounter → Start Combat | `POST /api/combat/start` (`internal/combat/handler.go:31`) |
| End combat | encounter → End | `POST /api/combat/{encounterID}/end` |
| Adjust HP / conditions / position | combat workspace | `PATCH /api/combat/{encID}/combatants/{cID}/{hp,conditions,position}` (`main.go:512-516`) |
| Advance turn / resolve pending / undo | combat workspace | DM dashboard routes (`main.go:519-532`) |
| **Resolve a monster's AoE save** (Shatter/Fireball/etc.) | combat workspace "Pending monster saves" → **Resolve save**, or DM Console `pending[]` → **Resolve** | `GET /api/combat/{encID}/pending-saves`, `POST …/pending-saves/{saveID}/resolve` (`main.go` mount; `internal/combat/pending_save_handler.go`) |

**Make a map with the dashboard map tools (no file needed) — preferred.** Open the
**Maps** tab → **+ New Map**, set the **Name** and **Width × Height (squares)**, click
**Create Map**. That opens the in-app **Map Editor**: a grid with paint **Modes** —
*Terrain* (Open Ground / Difficult Terrain / Water / Lava / Pit), *Lighting*, *Elevation*,
*Wall* / *Erase Wall*, *Spawn Zone* (Player / Enemy) / *Erase Spawn*, *Select* — plus
*Undo/Redo*, *Duplicate Map*, and **Save**. Click/drag tiles to paint; **Save** persists
the map and shows its **ID** in the footer. House convention (matches both existing maps):
**leave terrain blank and narrate the features** (pillars, the reek, the shaft) — only
paint a **Spawn Zone** at the PCs' entry edge so encounter token placement has a landing.
*Import Tiled (.tmj + images)* is the secondary path (button in the create form;
*Reimport Tiled* in the editor); a sample lives at `docs/testdata/sample.tmj` (10×10).
Existing maps for this campaign are listed in [`game-state.md`](game-state.md) "Maps" — reuse
or build alongside them.

### Starting a live combat (player-authoritative initiative)

**Read the verified facts first — do not re-investigate the API live.** The start-body
shape, the `Position.Col` letter format, the override contract, and the turn-ordering
rules are all pinned in memory ([[project_combat_start_pc_init_seat_repair]],
[[project_liveplay_build_combat_via_api]]) and in the **Appendix** of
[`combat-ops-improvements.md`](combat-ops-improvements.md) ("execute, don't
re-investigate"). Read those, then run the checklist below. Only re-investigate if the
code has changed under them.

1. **Build the encounter DM-side** (map + homebrew creature + template + tokens) — the
   Homebrew form omits `saving_throws`, so a boss's save proficiencies must go in via the
   API path (see [[project_liveplay_build_combat_via_api]]). Place tokens with
   `character_positions` where **`col` is a LETTER** (`A=0, B=1, …`) and **`row` is
   1-based** — sending a numeric `col` 400s with a bare `"invalid JSON body"` (APP-4).
2. **Prompt each player to roll their OWN initiative with exact, per-player syntax that
   includes their fixed modifier.** Give each their literal string — e.g. "Forge:
   `/roll 1d20+2 reason:initiative`", "Windreth: `/roll 1d20+4 reason:initiative`" — and
   say "include your +N; I read the total." An ambiguous prompt got mixed bare-`1d20` and
   `1d20+4` rolls this session and forced hand-added modifiers. Verify the `/roll` form
   parses before you post it (see [`dm-rules.md`](dm-rules.md) "Verify every slash
   command's syntax"). **Never roll a PC's die**; adding a player's *fixed, known* init
   modifier to a die they reported is legitimate adjudication (a deterministic stat, not a
   re-roll). Collect the totals from **#roll-history and #in-character both** — helper
   rolls (Guidance, a bare die) land in #roll-history with no queue row.
3. **Start combat — but know `POST /api/combat/start` auto-rolls ALL initiative, PCs
   included, with no opt-out** (APP-1), and seats the first turn on the app's own highest
   roll. So after the call:
   - **Discard the app's PC rolls** (they are not the players' dice).
   - **DM-override every combatant** to the players' real totals + the derived order via
     `POST /api/combat/{enc}/override/combatant/{cb}/initiative` — always send **both**
     `initiative_roll` and `initiative_order` (omitting order writes `0` → jumps to front,
     APP-3). Order = roll desc, then DEX-mod desc, then name — the app's own tie-break,
     applied honestly. Keep any **NPC** auto-roll (a legitimate DM-side die).
   - **If `start` seated the wrong round-1 actor**, re-seat with a guarded one-off `UPDATE`
     (no re-seat endpoint exists yet, APP-2) — see the **DB-repair exception** in
     [`dm-rules.md`](dm-rules.md): a single tightly-`WHERE`d `UPDATE turns SET
     combatant_id=<real order-1>, movement_remaining_ft=<their `characters.speed_ft`> WHERE
     id=<the auto-seated turn row>`, verified after; **never a `DELETE`**.
4. **Verify the seat**: read back round / order / first-turn combatant from the DM Console
   (or the DB — `encounters.current_turn_id` → `turns`, `combatants.initiative_order` ASC).
   The displaced combatant should have no turn row and get picked at its correct order next.
5. **Narrate round 1 + the first actor's turn**, then run the render + stat-leak check (§8).

> **This whole discard → override → seat-repair dance is a workaround for missing product
> capability**, tracked as **APP-1 / APP-2 / APP-5** in
> [`combat-ops-improvements.md`](combat-ops-improvements.md). When those ship (player-supplied
> initiative at `start` + a set-active-turn endpoint + an `/initiative` command) this
> collapses to a single clean start with zero overrides and no DB write.

### Resolving a monster's saving throw (AoE save spells)

When a PC casts an **AoE save-for-half** spell (Shatter, Fireball, Thunderwave…)
the app records one `pending_saves` row per creature in the area and **holds the
damage** until every target's save is resolved. Two resolution paths, by who owns
the target:

- **Player targets roll their own** save in Discord with **`/save`** (never roll
  for them — see [`dm-rules.md`](dm-rules.md)).
- **Monsters / NPCs are resolved by the DM** through the dashboard. The DM does
  **not** hand-roll the d20 — the engine rolls `d20 + the creature's save
  modifier` (from its stat block `saving_throws`, else the ability mod) vs the
  spell DC, then applies the AoE damage (half on a success).

**How:** open the **Combat** workspace's *Pending monster saves* section (in
combat) or the **DM Console** `pending[]` list (works out of combat too), and
click **Resolve** on each monster save. Endpoints, if you need to verify/drive by
API with the session cookie: `GET /api/combat/{encID}/pending-saves` lists the
unresolved monster saves; `POST /api/combat/{encID}/pending-saves/{saveID}/resolve`
(empty `{}` body) rolls + applies and returns `{natural_roll, save_bonus, total,
success, damage}`. It posts the result to `#combat-log` as the roll vs DC + damage
dealt — **never the monster's HP** (enemy HP/AC stay secret).

**Gotchas worth knowing (so you don't re-derive them):**

- **Damage lands only after the *last* target's save.** One unresolved monster
  save blocks the whole blast (the apply gate waits for all). If a spell "did no
  damage," check for an unresolved save and resolve it.
- **Idempotent + recoverable.** Re-POSTing resolve on an already-rolled save does
  **not** re-roll — it just (re)applies the stored result, then locks the row
  (`applied`). A second resolve after that returns `409`.
- Player-owned saves return `409` from this endpoint on purpose — they belong in
  Discord `/save`.

## 5. Onboarding players (one or many)

Per player:

1. User runs **`/register`** in Discord → taps **🆕 Build New** (runs
   `/create-character`) → bot DMs a one-time link `…/portal/create?token=…`
   (24h TTL, `cmd/dndnd/discord_adapters.go:35-49`).
2. User opens the link, builds the character in the web builder; it POSTs to
   `/portal/api/characters` (`internal/portal/api_handler.go:214-258`), creating a
   registration with status `pending`; a request lands in `#dm-queue`.
3. **Claude approves** it from the dashboard approvals view (or
   `POST /dashboard/api/approvals/{id}/approve`).
4. Bot DMs the player a welcome; `/character`, `/inventory`, etc. now work.
5. **Record it:** add a row to [`party/roster.md`](party/roster.md) + a
   `party/<name>.md` sheet, and fold the PC into the fiction ([`world.md`](world.md)).

Alternative paths: **📋 Claim Existing** (DM pre-creates on the dashboard, player
`/register name:<n>`), **📥 Import from D&D Beyond** (`/import ddb-url:<url>`).

**Big party (5-6 PCs):** approve each player as they finish — don't make an
onboarded player wait on the others. Remote players reach the portal + OAuth via
the tunnel (next section). Spotlight / pacing technique: [`big-party.md`](big-party.md).

## 5a. Remote players (ngrok tunnel, stable domain)

A player joining from another location reaches the local app (web builder + Discord
OAuth) via an **ngrok tunnel bound to a reserved domain** — so the public URL is
**stable** and the OAuth callback is registered in Discord **once**.

- **One-time setup** (per machine; see the header of
  [`scripts/tunnel.sh`](../../scripts/tunnel.sh)): create a free ngrok account,
  claim a free static domain, and put `NGROK_DOMAIN` + `NGROK_AUTHTOKEN` in `.env`
  (gitignored — the script never prints the token). Then register
  `https://<NGROK_DOMAIN>/portal/auth/callback` in the Discord Developer Portal →
  app (`DISCORD_CLIENT_ID` 1507…) → OAuth2 → Redirects. **Done once, never again.**
- **Managed by `make tunnel-up` / `make tunnel-down` / `make tunnel-status`**
  (`scripts/tunnel.sh`). `tunnel-up` auto-installs ngrok to `bin/`, starts the tunnel
  on the reserved domain, repoints `.env` (`BASE_URL` + `OAUTH_REDIRECT_URL`), and
  restarts the app. `tunnel-down` stops it and restores `.env` from
  `.env.bak.preTunnel` (which keeps the `NGROK_*` vars). State lives in `.tunnel/`
  (gitignored).
- **The public URL is STABLE** — every `tunnel-up` yields the same reserved domain,
  so there is **no per-restart Discord step**. (Discord has no API to script
  `redirect_uris`, which is why the stable URL matters.)
- **Current stable URL** is recorded in [`game-state.md`](game-state.md) "Ops
  snapshot."
- **Teardown after the session:** `make tunnel-down` (stops ngrok, restores `.env`,
  restarts the app). While up, the app is publicly reachable but gated: login by
  OAuth, build by a minted token, dashboard by DM auth.

## 6. Observing game state (Discord via Chrome · DM Console · Postgres)

Three read surfaces, pick by what you need:

- **Discord via Chrome (claude-in-chrome)** — open the Discord web app in the DM's
  logged-in Chrome and read any channel directly. **Required** for the human/roleplay
  layer the generated views never capture — above all **#in-character**, which is
  Discord-only and lands in no DB/Console feed. Also handy to eyeball #combat-log,
  #dm-queue, #the-story as the players see them. Read-only: never type in Discord
  (see [`dm-rules.md`](dm-rules.md)). Channel IDs are in
  [`game-state.md`](game-state.md); navigate to
  `https://discord.com/channels/<guildID>/<channelID>`.
- **DM Console** (`GET /api/dm/situation` / the `#dm-console` tab) — the generated
  source of truth for *mechanical* state (pending worklist, live encounter,
  combat+narration timeline). Start here for "what do I do / where are we."
- **Postgres** (raw reads) — when you need a field the above don't surface:

```sh
docker exec -e PGPASSWORD=dndnd dndnd-db-1 psql -U dndnd -d dndnd -X -c "SQL"
```

Useful tables: `campaigns` (settings JSONB has channel IDs), `characters` +
`player_characters` (sheet, HP, status), `maps`, `encounters`,
`encounter_templates`, `combatants` (HP/position/initiative), `turns`
(turn order / whose turn), `dm_queue_items` (pending approvals/whispers),
`action_log`. Or read state from the dashboard combat workspace in the browser.

**DB schema cheat-sheet (column names that cost round-trips — reference before querying):**

- `combatants`: **no speed column** — speed lives on `characters.speed_ft`. Ability scores
  are `characters.ability_scores` as **scores** (`{"dex":18,…}`), not modifiers.
  `position_col` is a **letter**, `position_row` is **1-based**.
- `turns`: the status column is **`status`** (not `is_complete`); also has
  `movement_remaining_ft`, `attacks_remaining`, `action_used`; `completed_at` is nullable.
- `encounters.current_turn_id` points at the active `turns.id`.
- `narration_posts` timestamp column is **`posted_at`** (not `created_at`).
- `dm_queue_items.status` ∈ `pending | resolved | cancelled`.

The [`combat-ops-improvements.md`](combat-ops-improvements.md) Appendix has the same list
plus the combat-start / override / turn-engine facts, verified 2026-07-09.

## 7. Common slash commands (the player types these)

`/register`, `/create-character`, `/character`, `/inventory`, `/equip`,
`/move <cell>`, `/done` (end turn), `/attack`, `/cast`, `/roll <dice> reason:<…>`
(players roll their own dice — see [`dm-rules.md`](dm-rules.md)), `/map` (re-posts
the combat board to #combat-map), `/loot`, `/give`, `/save`, `/recap`. The bot
replies in `#your-turn` / `#combat-log` / etc. Exact command set is whatever the
bot registered with the guild (the app's command table).

**#combat-map board posting:** since commit `7b6c125`, **StartCombat auto-posts the
opening board** to #combat-map. The board also (re)lands on the **first `/done`**, on
a **DM-run enemy turn**, or whenever any player runs **`/map`**. Player-view
fog-of-war is on (shows what a PC can see, hides the rest).

## 8. Posting DM narration to #the-story

To narrate a beat into the story channel from the dashboard:

1. In the DM dashboard SPA, open the **Narrate** tab (`#narrate`).
2. Type the narration text in the editor, then **wrap the story prose in a
   read-aloud block** — click **Insert Read-Aloud Block** and put the prose inside
   the `:::read-aloud … :::` fence. This is a standing DM preference: all #the-story
   narration goes out as a read-aloud block.
3. Click **Post to #the-story** — the bot relays the text to **#the-story**,
   stores a `narration_posts` row (timestamp + the Discord message id of the
   relayed post), and the beat also surfaces in the **DM Console** timeline.

The underlying call is `POST /api/narration/post` (behind DM auth). Verify a post
landed by querying the `narration_posts` table (or by the returned Discord message
id) — e.g. the Hold Person beat is `narration_posts` row at 13:51:18 UTC, msg id
`1519701526946386084`.

> **Standing rule (see [`dm-rules.md`](dm-rules.md)):** posting narration — like every
> DM *mutation* — must go through the dashboard driven by Chrome (claude-in-chrome),
> never raw SQL / curl. The mutation endpoints are behind `dmAuthMw`; only the
> logged-in dashboard tab can authenticate. Postgres is for *reads/observation* (§6).

### Render + stat-leak check (run before every post)

Before posting, verify the beat renders **OOC coda first / read-aloud box last** and leaks
no secret stat. The renderer ([`dashboard/svelte/src/lib/narration.js`](../../dashboard/svelte/src/lib/narration.js))
splits the source into a message `body` (the OOC coda — renders first) and one or more
`:::read-aloud` **embeds** (the boxed prose — render last). Don't re-author the check each
beat — paste the composed source and run this snippet (mentally, or via the browser
`javascript_tool` against the Narrate preview):

```js
// paste the exact narration source you're about to post:
const src = `...OOC coda...\n:::read-aloud\n...boxed prose...\n:::`;

// 1) split body vs read-aloud boxes (mirrors narration.js)
const lines = src.split('\n');
const body = [], boxes = []; let box = null;
for (const ln of lines) {
  const t = ln.trim();
  if (t === ':::read-aloud') { box = []; continue; }
  if (box && t === ':::') { boxes.push(box.join('\n')); box = null; continue; }
  (box ?? body).push(ln);
}
const coda = body.join('\n').trim();

// 2) structure: OOC coda present + non-empty, and ≥1 read-aloud box (OOC-first / box-last)
const structureOK = coda.length > 0 && boxes.length >= 1;

// 3) stat-leak scan over the WHOLE rendered text.
//    Leaks = internal numbers/ids. NOT leaks: board coords (D5), targeting short-ids (F1) —
//    those are table-visible in the tracker and used in /cast target:F1.
const all = [coda, ...boxes].join('\n');
const leaks = [
  [/\bAC\b/i,            'exact AC'],
  [/\b\d+\s*\/\s*\d+\b/, 'HP fraction (e.g. 15/22)'],
  [/\bhp\b\s*[:=]?\s*\d/i,'quoted HP number'],
  [/\bCR\s*\d/i,         'challenge rating'],
  [/\bhb_[0-9a-f]{6,}/i, 'homebrew creature id'],
  [/\b[0-9a-f]{8}-[0-9a-f]{4}-/i, 'internal UUID'],
].filter(([re]) => re.test(all)).map(([, why]) => why);

console.log(structureOK && leaks.length === 0
  ? 'PASS — OOC-first/box-last, no stat leak'
  : `FAIL — structure ${structureOK}, leaks: ${leaks.join(', ') || 'none'}`);
```

**Pass criteria:** structure `true` (a non-empty OOC coda **and** at least one read-aloud
box) **and** zero leaks. Describe enemy state in prose (*"staggers, bloodied"*) — never quote
`AC` / an HP fraction / `CR` / an `hb_…` or UUID id. This is the "Enemy HP and AC are secret"
and OOC-first/box-last rules from [`dm-rules.md`](dm-rules.md) made mechanical.
</content>
