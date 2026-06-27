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
| Import a map (Tiled .tmj) | Maps → New → Import | `POST /api/maps/import` (`internal/gamemap/handler.go:29`) |
| Create encounter + place tokens | Encounters → New | `POST /api/encounters/` (`internal/encounter/handler.go:29`) |
| Start combat (roll initiative) | encounter → Start Combat | `POST /api/combat/start` (`internal/combat/handler.go:31`) |
| End combat | encounter → End | `POST /api/combat/{encounterID}/end` |
| Adjust HP / conditions / position | combat workspace | `PATCH /api/combat/{encID}/combatants/{cID}/{hp,conditions,position}` (`main.go:512-516`) |
| Advance turn / resolve pending / undo | combat workspace | DM dashboard routes (`main.go:519-532`) |

Sample Tiled map for import lives at `docs/testdata/sample.tmj` (10×10). A map is
**already imported** for this campaign (see `game-state.md`); reuse it.

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

## 6. Observing game state (Claude can't see Discord)

Query Postgres directly:

```sh
docker exec -e PGPASSWORD=dndnd dndnd-db-1 psql -U dndnd -d dndnd -X -c "SQL"
```

Useful tables: `campaigns` (settings JSONB has channel IDs), `characters` +
`player_characters` (sheet, HP, status), `maps`, `encounters`,
`encounter_templates`, `combatants` (HP/position/initiative), `turns`
(turn order / whose turn), `dm_queue_items` (pending approvals/whispers),
`action_log`. Or read state from the dashboard combat workspace in the browser.

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
</content>
