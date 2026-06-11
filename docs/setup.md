# Setting up Discord & DnDnD

Get a DnDnD instance running — a Discord-native D&D 5e assistant with a web
dashboard for DMs and slash commands for players. This guide covers creating the
Discord application, configuring the service, and bringing it online with Docker
Compose.

**Guides:** ① Setup (this page) · ② [How to use](usage.html) · ③ [Tiled maps](tiled-maps.md) ([HTML version](tiled-maps.html))

## On this page

1. [Run modes](#run-modes)
2. [Prerequisites](#prerequisites)
3. [Create the Discord application](#create-the-discord-application)
4. [Configure `.env`](#configure-env)
5. [Run & verify](#run--verify)
6. [First run in Discord](#first-run-in-discord)
7. [Troubleshooting](#troubleshooting)

## Run modes

DnDnD can run two ways, controlled entirely by which environment variables you set:

- **Dashboard-only** — leave the Discord credentials blank. The web dashboard
  runs and, with `DISCORD_CLIENT_ID`/`DISCORD_CLIENT_SECRET` blank, logs everyone
  in as `DEV_DISCORD_USER_ID` (passthrough auth). Good for poking at the UI.
- **Full bot** — set the four `DISCORD_*` values. The bot connects to the
  gateway, registers slash commands per guild, and players interact from Discord.

## Prerequisites

- **Docker** + **Docker Compose** (the supported path), *or* Go + a local
  PostgreSQL if you prefer running the binary directly.
- A **Discord account** and a server (guild) you can manage, for the full-bot mode.

## Create the Discord application

Skip this section if you only want dashboard-only mode.

### 1. Create the app & bot

- Go to the [Discord Developer Portal](https://discord.com/developers/applications)
  → **New Application**.
- **Bot** tab → add a bot → copy the **token** → this is `DISCORD_BOT_TOKEN`.
- From **General Information**, copy the **Application ID** → `DISCORD_APPLICATION_ID`.
- From **OAuth2**, copy the **Client ID** and **Client Secret** →
  `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET`.

### 2. Enable the Server Members intent

> ⚠ The bot declares the **Server Members** gateway intent (privileged). In the
> **Bot** tab, enable **Server Members Intent**, or the bot fails to start its
> session. *Message Content intent is not required* — DnDnD uses slash commands.

### 3. Add the OAuth2 redirect

Under **OAuth2 → Redirects**, add the dashboard login callback. The app builds it
as `BASE_URL + /portal/auth/callback`, so for local Docker:

```
http://localhost:8080/portal/auth/callback
```

The dashboard login requests only the `identify` scope. (For a deployed instance,
add the `https://…/portal/auth/callback` form and set `OAUTH_REDIRECT_URL` if
behind a proxy/tunnel.)

### 4. Invite the bot to your server

Build an invite URL with the `bot` and `applications.commands` scopes. A
recommended permissions integer (view/manage channels & roles, send/embed/attach,
use slash commands) is `2416176144`:

```
https://discord.com/oauth2/authorize?client_id=<APP_ID>&scope=bot+applications.commands&permissions=2416176144
```

## Configure `.env`

Copy the template and fill it in:

```bash
make local-env      # copies .env.example → .env (if absent)
$EDITOR .env
```

| Variable | Purpose | Required? | Default |
| --- | --- | --- | --- |
| `DATABASE_URL` | Postgres connection string. In Compose it's derived from the `POSTGRES_*` values. | For real use | — (blank → DB features skipped) |
| `DISCORD_BOT_TOKEN` | Bot gateway token. | Full bot | — |
| `DISCORD_APPLICATION_ID` | App ID used to register slash commands per guild. | Slash commands | — |
| `DISCORD_CLIENT_ID` | OAuth2 client ID for dashboard login. | Blank → passthrough | — |
| `DISCORD_CLIENT_SECRET` | OAuth2 client secret. | Blank → passthrough | — |
| `BASE_URL` | Public base URL for OAuth redirect & links. | Optional | `http://localhost:8080` |
| `OAUTH_REDIRECT_URL` | Explicit callback override for proxies/tunnels. | Optional | `BASE_URL` + `/portal/auth/callback` |
| `TOKEN_ENCRYPTION_KEY` | AES-256 key encrypting stored OAuth tokens. | Recommended | — (blank → plaintext) |
| `COOKIE_SECURE` | Session cookie Secure flag. Set `false` for local HTTP. | Optional | secure unless `false` |
| `DEBUG` | Verbose logging when `true`. | Optional | `false` |
| `SKIP_SRD_SEED` | Skip the SRD reference-data seed when `true`. | Optional | `false` |
| `ASSET_DATA_DIR` | Directory for uploaded maps/images. Compose mounts a volume. | Optional | `/data/assets` (Compose) |
| `DEV_DISCORD_USER_ID` | The user logged in as during passthrough (no-OAuth) mode. | Optional | `local-dev` |

> **Compose-only (not read by the app):** `APP_PORT` (8080), `POSTGRES_PORT`
> (5432), `POSTGRES_DB`/`POSTGRES_USER`/`POSTGRES_PASSWORD` (all `dndnd`).

> 🔑 **`TOKEN_ENCRYPTION_KEY` must be exactly 32 characters** (a 32-byte AES-256
> key). Any other length is rejected and tokens silently fall back to *plaintext*
> storage. Generate one with:
>
> ```bash
> openssl rand -hex 16   # 32 hex characters
> ```

## Run & verify

```bash
make local-up       # docker compose up --build
# logs:    make local-logs
# stop:    make local-down
# reset:   make local-reset   # ⚠ destroys the DB + asset volumes
```

On boot, the app connects to Postgres, **runs migrations automatically**, and
**seeds the 5e SRD reference data** (classes, races, spells, creatures, magic
items) — there is no separate migrate/seed step. Disable the seed with
`SKIP_SRD_SEED=true`.

> ✓ Open [http://localhost:8080](http://localhost:8080) — it redirects to the
> dashboard. With OAuth configured you're sent to `/portal/auth/login`; with
> passthrough you're in immediately as `DEV_DISCORD_USER_ID`.

## First run in Discord

Once the bot is in your server, an admin runs the setup command to create the
campaign's channel structure:

```
/setup
```

`/setup` requires the **Manage Channels** permission (it's the only DM/admin-gated
command). All other slash commands are player-facing. See the
[How to use](usage.html) guide for the full command list and the DM dashboard tour.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| Bot won't connect / session fails | Enable **Server Members Intent** (Bot tab) and check `DISCORD_BOT_TOKEN`. |
| Slash commands don't appear | Set `DISCORD_APPLICATION_ID`; commands register per guild on startup once the bot has joined the server. Re-invite with the `applications.commands` scope. |
| Login does nothing / everyone is "local-dev" | Passthrough mode — `DISCORD_CLIENT_ID` or `DISCORD_CLIENT_SECRET` is blank. Set both for real OAuth. |
| OAuth redirect mismatch error | The redirect in the Developer Portal must exactly match `BASE_URL + /portal/auth/callback` (or `OAUTH_REDIRECT_URL`). |
| Login loops on local HTTP | Set `COOKIE_SECURE=false` for plain `http://localhost`. |
| Tokens stored unencrypted warning | `TOKEN_ENCRYPTION_KEY` isn't exactly 32 characters. Use `openssl rand -hex 16`. |
| Dashboard runs but data features are dead | `DATABASE_URL` is unset — the DB block is skipped. Set it (Compose derives it from `POSTGRES_*`). |

---

See also: [How to use](usage.html) · [Building maps with Tiled](tiled-maps.md).
Sources: `docs/local-run.md`, `docs/playtest-quickstart.md`, `.env.example`.
