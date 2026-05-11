# Playtest Quickstart — Verifier Verdict

Date: 2026-05-11
Source: `docs/playtest-quickstart.md` (v post-Crit/High remediation campaign)

## Summary

All nine Critical and ten High blockers from `.review-findings/SUMMARY.md` are
closed in the production wiring. `make build`, `make test`, `make e2e`, and
`make playtest-replay TRANSCRIPT=internal/playtest/testdata/sample.jsonl` all
pass cleanly against the current tree. Every step a contributor would walk
through has either a real production code path with non-stub adapters in
`cmd/dndnd/main.go` + `cmd/dndnd/discord_handlers.go`, or — for the
Discord-server-side steps that cannot be mechanically verified from a
sandbox — production wiring that the e2e harness exercises end-to-end via
`internal/testutil/discordfake`.

## Per-step assessment

- **0. Prerequisites** — READY. Tool/version table is concrete and accurate.
  Postgres docker one-liner works; `go.mod` confirms Go 1.22+.
- **1. Clone & build** — READY. `make build` produced `bin/dndnd` and
  `bin/playtest-player` cleanly (no warnings) on this checkout.
- **2. Database** — READY. `cmd/dndnd/main.go:376` calls
  `database.MigrateUp(db, dbfs.Migrations)` on boot and seeds SRD reference
  data unless `SKIP_SRD_SEED=true`; `make test` exercises the seeded path
  via `TestE2E_*` scenarios.
- **3. Discord application** — READY-WITH-NOTE. Instructions and the
  `permissions=2416036880` invite bitfield match what `internal/discord/commands.go`
  needs (`Manage Channels` for `/setup`). Cannot drive a real Discord
  developer portal from this sandbox; production OAuth env wiring is in
  `buildAuth` (`cmd/dndnd/main.go:176`).
- **4. Boot the bot** — READY. Doc's described boot log lines match the
  actual logger calls; missing-env fallback to passthrough auth is honoured at
  `cmd/dndnd/main.go:180`. `ASSET_DATA_DIR` default of `data/assets`
  (relative) is documented (med-42 is a separate fly-deploy concern, not a
  local-playtest blocker).
- **5. Bootstrap campaign** — READY. crit-02 closed: `setupHandler :=
  discord.NewSetupHandler(bot, newSetupCampaignLookup(queries))` is wired at
  `cmd/dndnd/main.go:858`; routed at `internal/discord/router.go:291`.
  Dashboard char-create lives at `internal/dashboard/charcreate_service.go`
  with the menu link `/dashboard/characters/new` at
  `internal/dashboard/handler.go:42`. `/register` handler is real
  (`internal/discord/registration_handler.go:70`). Verified by
  `TestCommandRouter_RoutesToSetupHandler` and the e2e replay scenario.
- **6. Approve the character** — READY. crit-06 + crit-07 closed:
  `approvalNotifier = newPlayerNotifierAdapter(discord.NewDirectMessenger(...))`
  at `cmd/dndnd/main.go:608` is passed into `dashboard.NewApprovalHandler` so
  approval/rejection actually DMs the player. Portal token validator wired at
  `cmd/dndnd/main.go:619` with `portal.NewTokenService(...)` and matching
  `TokenFunc: newPortalTokenIssuer(...)` at line 846 (no more `e2e-token`
  placeholder). high-17 closed: `portal.RegisterRoutes` is called with
  `WithAPI` + `WithCharacterSheet` (`cmd/dndnd/main.go:631-636`), verified by
  `TestPortalRegisterRoutes_*` in `cmd/dndnd/main_wiring_test.go:280`.
- **7. Build encounter & go live** — READY. The Encounter Builder route
  (`/dashboard/encounters`) is in the dashboard menu, the `/move` slash
  handler is wired (crit-05 turn-lock + ownership), high-10 wires
  `mapRegenerator` so PostCombatMap actually fires (verified by
  `TestBuildDiscordHandlers_WiresMapRegenerator` and
  `TestMapRegeneratorAdapter_RendersAndDebouncesViaQueue`). The
  `docs/testdata/sample.tmj` file ships in-tree as the doc claims.
- **8. Hand off to player agent** — READY. `bin/playtest-player` builds; env
  contract documented in the doc matches `cmd/playtest-player/main.go:75-85`
  exactly (`DISCORD_BOT_TOKEN`, `DISCORD_APPLICATION_ID`, `GUILD_ID`).

## Test/harness evidence

- `make build` — exit 0, both binaries produced.
- `make test` — exit 0 (full unit + integration suite).
- `make e2e` — exit 0; `TestE2E_RecapEmptyScenario` and the rest of
  `cmd/dndnd/e2e_*_test.go` run the full Discord-fake interaction loop
  against the same wiring path the playtest doc invokes.
- `make playtest-replay TRANSCRIPT=$PWD/internal/playtest/testdata/sample.jsonl`
  — exit 0; `TestE2E_ReplayFromFile` PASS (2.82s) drives the Phase 121
  replay harness against in-process Discord.

## Note on what was not verifiable from sandbox

Steps 3 (real developer-portal click-through), 4 (live `discord.Open` against
gateway), 5's actual `#dm-private` channel creation in a real guild, 6's
real OAuth callback hop, and 8's live second-bot session require a real
Discord guild and cannot be exercised here. For each, the production code
path is in place (no nil deps, no stub handlers) and the e2e harness drives
the equivalent path through `internal/testutil/discordfake` — they are
ruled READY on that basis.

## Verdict: READY
