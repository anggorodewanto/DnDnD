# Playtest Quickstart

Goal: get a fresh checkout of DnDnD from empty machine to a live, `/move`-ready
encounter on a real Discord server in under 30 minutes.

This is the doc you hand to a contributor (or yourself, six months from now)
who wants to drive a manual playtest with the [Phase 121 player-agent
CLI](../cmd/playtest-player). It complements the automated Phase 120 E2E
harness — the harness verifies that the wiring still holds; this doc verifies
that it still feels playable.

> **Self-test target: < 30 minutes.** If you blow past that on a clean machine
> following these steps, file the snag against Phase 121 (or fix the doc in
> the same PR). The whole point of this iteration is to keep the on-ramp
> short.

## 0. Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.22+ | `go version` — matches `go.mod` |
| PostgreSQL | 14+ | Local install or `docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:16` |
| `git` | any | for the checkout |
| Discord account | — | with permission to create a server you control |
| Discord developer account | — | https://discord.com/developers/applications |

You also need a public-facing tunnel **only** if you want OAuth dashboard
login from outside `localhost`. For a single-machine playtest, the dashboard
runs on `http://localhost:8080` and the bot reaches Discord outbound — no
inbound tunnel needed.

## 1. Clone & build (≈ 2 min)

```sh
git clone <your-fork-or-this-repo> dndnd
cd dndnd
make build
```

`make build` produces both `bin/dndnd` (the bot + dashboard) and
`bin/playtest-player` (the player-side REPL used in step 8). If it fails, fix
the build before going any further — none of the rest of this doc will work
otherwise.

## 2. Database (≈ 3 min)

Start a local Postgres if you don't already have one running, then create a
database for the playtest:

```sh
createdb -h localhost -U postgres dndnd_playtest   # prompts for the postgres password
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/dndnd_playtest?sslmode=disable'
```

The `-h localhost -U postgres` forces a TCP connection as the `postgres` role.
Without it, `createdb` uses a unix socket with peer auth and fails
(`role "<your-OS-user>" does not exist`) against the dockerized Postgres
suggested above. The password is whatever you set in `POSTGRES_PASSWORD`
(`postgres` in the `docker run` line in step 0).

Migrations run automatically on `cmd/dndnd` boot the first time it sees a
`DATABASE_URL` — no separate `make migrate` step is required. SRD reference
data (classes, races, spells, creatures, magic items) is seeded the same way.

## 3. Discord application (≈ 8 min)

1. Open https://discord.com/developers/applications and click **New
   Application**. Name it whatever (e.g. `DnDnD-Playtest`).
2. **Bot tab → Reset Token** → copy the token. This is `DISCORD_BOT_TOKEN`.
3. **Bot tab → Privileged Gateway Intents** → enable **Server Members
   Intent** and **Message Content Intent**.
4. **OAuth2 tab → Redirects** → add
   `http://localhost:8080/portal/auth/callback`.
5. **OAuth2 tab → Client ID / Client Secret** → copy both. These are
   `DISCORD_CLIENT_ID` and `DISCORD_CLIENT_SECRET`.
6. **General Information → Application ID** → copy. This is
   `DISCORD_APPLICATION_ID`.
7. Build the invite URL by replacing `<APP_ID>`:

   ```
   https://discord.com/oauth2/authorize?client_id=<APP_ID>&scope=bot+applications.commands&permissions=2416176144
   ```

   The `permissions=2416176144` bitfield grants the minimum the bot needs:
   `View Channels`, `Send Messages`, `Manage Channels` (required by
   `/setup` — see `internal/discord/commands.go`), `Manage Roles`,
   `Manage Messages`, `Mention Everyone`, `Embed Links`, `Attach Files`,
   `Read Message History`, `Use Slash Commands`. Tighten or loosen later as
   you wish; this is a sane playtest default.
8. Open the URL, pick a server you own (create a fresh one for the playtest
   if you don't have a throwaway), and authorize.

## 4. Boot the bot (≈ 1 min)

In the same shell where you exported `DATABASE_URL`:

```sh
export DISCORD_BOT_TOKEN='...'
export DISCORD_APPLICATION_ID='...'
export DISCORD_CLIENT_ID='...'
export DISCORD_CLIENT_SECRET='...'
export ASSET_DATA_DIR="$PWD/.playtest-assets"
export DEBUG=true
mkdir -p "$ASSET_DATA_DIR"
./bin/dndnd
```

You should see:

```
discord session constructed (open deferred until after recovery)
http server listening addr=:8080
```

If `DISCORD_*` env vars are missing, the dashboard falls back to a passthrough
auth middleware (see the `buildAuth` fallback path in
`cmd/dndnd/main.go`) and the bot logs `discord session skipped` — that mode
is fine for poking at the dashboard but not for a real playtest.

## 5. Bootstrap the campaign in Discord (≈ 5 min)

In your test server, in any text channel:

```
/setup
```

The bot creates the campaign channel structure (`#the-story`, `#combat-log`,
`#dm-private`, etc.). This requires `Manage Channels` on the bot — the invite
URL above already grants it.

A player gets a character in one of three ways, and `/register` now
surfaces all of them. As a player on the server, run it with no argument:

```
/register
```

The bot replies with three buttons:

- **📋 Claim Existing** — opens a modal to type the name of a character the
  DM pre-created on the dashboard
  (http://localhost:8080/dashboard/app/#characters-new). Same as
  `/register name:<name>`.
- **🆕 Build New** — runs `/create-character`: mints a one-time link to the
  web character builder at `http://localhost:8080/portal/create?token=…`.
  The portal is served from the same host as the dashboard, so a localhost
  playtest needs no public tunnel.
- **📥 Import from D&D Beyond** — opens a modal to paste a D&D Beyond
  character URL. Same as `/import ddb-url:<url>`.

All three submit a registration with status `pending`: the bot posts an
approval request to `#dm-private` and the player gets an ephemeral
"pending DM approval" confirmation.

## 6. Approve the character on the dashboard (≈ 3 min)

1. Open http://localhost:8080 in a browser.
2. Click **Login with Discord** → authorize. Discord redirects back to
   `/portal/auth/callback` and you land on the campaign list.
3. Pick the campaign, find the pending registration, click **Approve**.
4. The bot DMs the player the welcome message and the player can now run
   `/character`, `/inventory`, etc.

## 7. Build an encounter & go live (≈ 5 min)

From the dashboard:

1. **Maps → Upload** a Tiled `.tmj` map. A 10×10 sample lives at
   [`docs/testdata/sample.tmj`](testdata/sample.tmj) — upload it directly
   if you don't have your own.
2. **Encounters → New** → pick the map → drag the player and one or two SRD
   monsters onto the grid.
3. **Go Live** → the bot rolls initiative and posts the turn order to
   `#combat-log`.

In Discord:

```
/move A1
```

Confirm, then `/done`. If both lines land in `#combat-log` and the
combatant's position updates on the dashboard map, the playtest stack is
green.

## 8. Hand off to the player agent

You are now ready to drive the player side from the [player-agent
CLI](../cmd/playtest-player) instead of typing slash commands by hand.
The player-agent CLI uses a separate bot account; repeat steps 3–4 with a
second Discord application (name it `DnDnD-Playtest-Player`) and invite that
bot to the same server. Then:

```sh
DISCORD_BOT_TOKEN=<player-bot-token> \
DISCORD_APPLICATION_ID=<dndnd-bot-app-id> \
GUILD_ID=<your-server-id> \
./bin/playtest-player
```

`DISCORD_APPLICATION_ID` is the **dndnd bot's** app ID (from step 3.6),
**not** the player bot's. The player-agent loads that app's registered slash
commands as its validation table — point it at the wrong app and you'll get
zero commands and a silent fall-back to the in-process default set.

See [`docs/playtest-checklist.md`](playtest-checklist.md) (Phase 121.4) for
the scenarios to run.

## Self-test log

Re-time this walkthrough on a clean checkout whenever you touch a step
above. Record the run here so future-you knows the doc still works.

| Date | Wall time (clone → `/move` lands) | Notes / snags |
| --- | --- | --- |
| _pending first self-test_ | — | — |
