# task crit-02 — `/setup` broken (no SetupHandler in production)

## Finding (verbatim from chunk2_campaign_maps.md, Phase 12)

> ❌ **Production never wires `SetupHandler`.** `cmd/dndnd/main.go:741` calls `discord.NewCommandRouter(bot, nil, regDeps)` — first arg is the setup handler. With it `nil`, `internal/discord/router.go:237-239` does NOT register `setup`, and `setup` is also absent from the `gameCommands` slice (router.go:198-204), so `/setup` falls through to `respondEphemeral("Unknown command: /setup")` at router.go:265. **This is the largest single blocker for an end-to-end playtest** — it directly contradicts the Phase 12 done-when bullet.

Spec sections: spec lines 131–151 ("Discord Server Structure"), Phase 12 done-when in `docs/phases.md` ("`/setup` Channel Structure").

Cross-cutting follow-up #1 from chunk2: "Wire `SetupHandler` in `cmd/dndnd/main.go`. Highest priority: pass a real handler into `discord.NewCommandRouter`. Decide whether `/setup` also auto-creates the campaign row when none exists for the guild (closes the Phase 11 done-when gap)."

## Plan (worker fills)

1. Build a `setupCampaignLookup` adapter in `cmd/dndnd/discord_adapters.go` implementing `discord.CampaignLookup`. It wraps a narrow `setupQueries` interface (the two `refdata.Queries` methods we need) so it stays unit-testable without a live Postgres.
   - `GetCampaignForSetup(guildID)` -> `GetCampaignByGuildID` -> `SetupCampaignInfo{DMUserID: c.DmUserID}`.
   - `SaveChannelIDs(guildID, ids)` -> fetch campaign, decode existing `campaign.Settings` (fall back to `campaign.DefaultSettings()` when the JSONB column is null), set `Settings.ChannelIDs = ids`, marshal, call `UpdateCampaignSettings`.
2. Replace `discord.NewCommandRouter(bot, nil, regDeps)` in `cmd/dndnd/main.go` with `discord.NewCommandRouter(bot, discord.NewSetupHandler(bot, newSetupCampaignLookup(queries)), regDeps)`.
3. Red/green tests against the adapter (happy paths + error propagation + null-settings default seeding) — the existing `TestCommandRouter_RoutesToSetupHandler` in `internal/discord/router_test.go` already covers the router's wiring contract, so the new tests focus on the cmd-side adapter that was missing.

## Files touched

- `cmd/dndnd/discord_adapters.go` — added `setupQueries` interface + `setupCampaignLookup` adapter (~60 LoC, plus 2 imports: `encoding/json`, `pqtype`, `internal/campaign`, `internal/discord`).
- `cmd/dndnd/discord_adapters_test.go` — added `fakeSetupQueries` mock + 6 unit tests covering the adapter; added `encoding/json`, `pqtype`, `internal/campaign` imports.
- `cmd/dndnd/main.go` — single 5-line change at line 741: build `setupHandler` via `discord.NewSetupHandler(bot, newSetupCampaignLookup(queries))` and pass it to `NewCommandRouter` instead of `nil`. 3-line comment explains why.

## Tests added

In `cmd/dndnd/discord_adapters_test.go`:
- `TestSetupCampaignLookup_GetCampaignForSetup_ReturnsDMUserID` — happy path: returns campaign DM user id.
- `TestSetupCampaignLookup_GetCampaignForSetup_PropagatesError` — DB error wrapped via `%w`, `errors.Is` succeeds.
- `TestSetupCampaignLookup_SaveChannelIDs_MergesIntoSettings` — existing settings (turn timeout, diagonal rule, open5e sources) survive the channel-id update.
- `TestSetupCampaignLookup_SaveChannelIDs_DefaultSettingsWhenNullSettings` — when the JSONB column is null, defaults are seeded then channel ids merged in.
- `TestSetupCampaignLookup_SaveChannelIDs_PropagatesGetError` — campaign-fetch failure aborts before any update call.
- `TestSetupCampaignLookup_SaveChannelIDs_PropagatesUpdateError` — update-call failure surfaces wrapped error.

All 6 pass. `go test ./internal/discord/... ./cmd/dndnd/...` is green; `make cover-check` is green (overall 94.40%, all per-package thresholds met). The adapter file is in the coverage exclude list (`cmd/dndnd/discord_adapters.go`) so the new code does not move the per-package number, but the unit tests still exercise every branch.

## Implementation notes

- Picked `cmd/dndnd/discord_adapters.go` rather than `main.go` for the adapter because that file already houses sibling adapters (`discordUserEncounterResolver`) wrapped in a small `*Queries`-subset interface for testability — same pattern reused.
- The existing `dashboardCampaignLookup` (lines 88–110) takes a `dmUserID -> active campaign` shape, whereas `discord.CampaignLookup` wants `guildID -> SetupCampaignInfo` + `SaveChannelIDs`. A separate adapter is cleaner than overloading the dashboard one.
- Settings handling preserves existing JSONB fields by unmarshaling first, mutating only `ChannelIDs`, then re-marshaling. When the column is null I seed with `campaign.DefaultSettings()` (matches what `campaign.Service.CreateCampaign` writes for a fresh row, so the JSONB stays consistent with the rest of the codebase).
- Followed early-return style throughout. No dead fields, no premature abstraction (the `setupQueries` interface is the minimum required by `setupCampaignLookup`; same idiom as `resolverQueries`).
- Out-of-scope follow-up appended to `.fix-state/log.md`: even with `/setup` reaching `SetupHandler`, the handler's `GetCampaignForSetup` errors out with "no campaign found for this server" when no campaign row exists for the guild — the Phase 11 done-when bullet ("Campaign created on `/setup`") is still unmet. Suggested fix path noted in `log.md` for a separate task.
- Did NOT change `internal/discord/router.go` — the routing logic at `if setupHandler != nil { r.handlers["setup"] = setupHandler }` was already correct; the bug was strictly that production passed `nil`. The pre-existing `TestCommandRouter_RoutesToSetupHandler` in `internal/discord/router_test.go` already proves the router-level contract; my new tests prove the production-side adapter (`cmd/dndnd`) constructs and behaves correctly.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

- Production wiring fixed: `cmd/dndnd/main.go:742` now constructs `discord.NewSetupHandler(bot, newSetupCampaignLookup(queries))` and passes it as the second arg to `NewCommandRouter` (was `nil`). The 3-line comment justifies the change. Phase 12 done-when ("/setup creates all channels with correct permissions, skips duplicates, stores channel references") is now reachable in production for guilds that already have a campaign row.
- Adapter (`cmd/dndnd/discord_adapters.go`) implements `discord.CampaignLookup` exactly: `GetCampaignForSetup` returns `SetupCampaignInfo{DMUserID: c.DmUserID}`; `SaveChannelIDs` decodes existing settings (or seeds `campaign.DefaultSettings()` on null), mutates only `ChannelIDs`, re-marshals, and calls `UpdateCampaignSettings`. Errors wrapped with `%w`. Pattern (`setupQueries` interface for testability) mirrors the sibling `discordUserEncounterResolver`/`resolverQueries`.
- Tests:
  - Adapter: 6 unit tests in `cmd/dndnd/discord_adapters_test.go` covering both methods (happy path, default-settings seeding for null JSONB, get-error propagation, update-error propagation, settings-merge preserving turn timeout / diagonal rule / open5e sources). All pass.
  - Router wiring: pre-existing `TestCommandRouter_RoutesToSetupHandler` (`internal/discord/router_test.go:55`) proves `setup` reaches `SetupHandler` when non-nil. Both layers covered as required.
- Coverage: `make cover-check` is GREEN. `cmd/dndnd` raw coverage is 82.8% but the adapter file is in `Makefile:7` `COVER_EXCLUDE`, matching the worker's claim; `internal/discord` is 91.7%. All per-package thresholds met (≥85%).
- No scope creep on this task's part. Note: `cmd/dndnd/main.go` and `cmd/dndnd/discord_handlers.go` carry an additional unrelated `db: db` / `turnGateAdapter` diff, but that belongs to crit-05 (verified in `.fix-state/task_crit-05.md`), not this worker.
- Out-of-scope follow-up note (re: Phase 11 campaign-creation gap surfaced via `GetCampaignForSetup` returning "no campaign found") is well-formed and correctly scoped: it identifies the next-in-line Phase 11 done-when bullet ("Campaign created on /setup"), defers to a separate task, and matches the cross-cutting follow-up #1 wording from chunk2_campaign_maps.md (line 150). On the right track.

