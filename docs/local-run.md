# Local Runbook

This is the production-shaped local path: one app container built from the
same `Dockerfile` used by Fly, one Postgres container, and a persistent
`/data/assets` volume matching `fly.toml`.

## First Run

```sh
make local-env
```

Edit `.env` and fill the Discord values if you want a real bot playtest:

```text
DISCORD_BOT_TOKEN=...
DISCORD_APPLICATION_ID=...
DISCORD_CLIENT_ID=...
DISCORD_CLIENT_SECRET=...
```

If `DISCORD_CLIENT_ID` or `DISCORD_CLIENT_SECRET` is blank, the app uses local
passthrough auth and opens the dashboard as `DEV_DISCORD_USER_ID` instead of
requiring login. Set `DEV_DISCORD_USER_ID` to your Discord user ID if you want
local campaign ownership checks to match real Discord data.

For local OAuth, add this redirect URL in the Discord developer portal:

```text
http://localhost:8080/portal/auth/callback
```

Then start everything:

```sh
make local-up
```

Open:

```text
http://localhost:8080
```

The app runs migrations and seeds SRD reference data automatically on boot.

## Daily Commands

```sh
make local-up       # build and start app + db
make local-logs     # follow app logs
make local-down     # stop containers, keep db/assets volumes
make local-reset    # delete local db/assets volumes, then start fresh
```

## Cloud Parity Notes

Local Compose deliberately uses the same runtime contract as Fly:

- app image is built from `Dockerfile`
- database is configured only through `DATABASE_URL`
- uploaded assets live under `/data/assets`
- OAuth and portal links come from `BASE_URL`
- secure cookies are controlled by `COOKIE_SECURE`

For Fly, first rename the app in `fly.toml` — `app = "dndnd"` is already taken,
so pick a globally-unique name (e.g. `dndnd-<yourname>`). Then use the same
values with production-safe differences:

```sh
fly secrets set \
  DATABASE_URL='postgres://...' \
  BASE_URL='https://<your-fly-app>.fly.dev' \
  COOKIE_SECURE='true' \
  DISCORD_BOT_TOKEN='...' \
  DISCORD_APPLICATION_ID='...' \
  DISCORD_CLIENT_ID='...' \
  DISCORD_CLIENT_SECRET='...' \
  TOKEN_ENCRYPTION_KEY='...'
```

Create the persistent Fly volume once, matching `fly.toml`:

```sh
fly volumes create dndnd_data --region ord --size 1
```

Then deploy:

```sh
fly deploy
```
