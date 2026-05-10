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

`make build` produces `bin/dndnd`. If it fails, fix the build before going any
further — none of the rest of this doc will work otherwise.

## 2. Database (≈ 3 min)

Start a local Postgres if you don't already have one running, then create a
database for the playtest:

```sh
createdb dndnd_playtest
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/dndnd_playtest?sslmode=disable'
```

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
   https://discord.com/oauth2/authorize?client_id=<APP_ID>&scope=bot+applications.commands&permissions=2416036880
   ```

   The `permissions=2416036880` bitfield grants the minimum the bot needs:
   `View Channels`, `Send Messages`, `Manage Channels` (required by
   `/setup` — see `internal/discord/commands.go`), `Manage Roles`,
   `Embed Links`, `Attach Files`, `Read Message History`, `Use Slash
   Commands`. Tighten or loosen later as you wish; this is a sane playtest
   default.
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
auth middleware (see `cmd/dndnd/main.go:130`) and the bot logs `discord
session skipped` — that mode is fine for poking at the dashboard but not for
a real playtest.

## 5. Bootstrap the campaign in Discord (≈ 5 min)

In your test server, in any text channel:

```
/setup
```

The bot creates the campaign channel structure (`#the-story`, `#combat-log`,
`#dm-private`, etc.). This requires `Manage Channels` on the bot — the invite
URL above already grants it.

Then, as the DM, create the character record. Open
http://localhost:8080/dashboard/characters/new in a browser and fill in
name (e.g. `Aria`), class, race, and background. Save.

Now, as a player on the server, claim the character:

```
/register name:Aria
```

`/register` only takes `name` — it links the invoking Discord user to a
character the DM has already created. (`/create-character` is the
alternative path that opens the web character builder, but that flow needs
a public portal URL — out of scope for a localhost playtest.) The bot
posts an approval request to `#dm-private` and the player gets an
ephemeral "pending DM approval" confirmation.

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
GUILD_ID=<your-server-id> \
./bin/playtest-player
```

See [`docs/playtest-checklist.md`](playtest-checklist.md) (Phase 121.4) for
the scenarios to run.

## Self-test log

Re-time this walkthrough on a clean checkout whenever you touch a step
above. Record the run here so future-you knows the doc still works.

| Date | Wall time (clone → `/move` lands) | Notes / snags |
| --- | --- | --- |
| _pending first self-test_ | — | — |
