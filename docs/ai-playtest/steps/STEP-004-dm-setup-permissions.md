# STEP-004 — DM `/setup` permission paths (admin auto-create + rejections)

Phase: **AUTOMATED** ✅ (crystallized + green). See [../README.md](../README.md).

## RESULT (2026-06-20)

Completes the `/setup` authorization surface deferred by STEP-003. Three paths,
all confirmed live and crystallized:

| Path | Precondition | Observed (public, via deferred→edit) |
| --- | --- | --- |
| **admin auto-create success** | no campaign, invoker is server admin | `Campaign created and channel structure set up! 10 channels set up.` |
| **non-DM reject** | campaign exists, invoker ≠ DM | `⛔ Only the campaign DM can run /setup for this server.` |
| **non-admin reject** | no campaign, invoker not admin | `⛔ Only a server administrator can create a new campaign via /setup.` |

This is the **first step that fixed a bug found during authoring** (vs. just
logging it) — the user signed off on "fix it now too".

### Bug found + fixed (authorization)

`GetCampaignForSetup` (`cmd/dndnd/discord_adapters.go`) **persisted** the
auto-created campaign *inside the lookup*, before the handler's server-admin gate
(`internal/discord/setup.go`) ran. So a non-admin who ran `/setup` on a fresh
guild saw `⛔ Only a server administrator…` **yet a campaign was created with
them as DM** — the gate failed its own purpose. The unit tests never caught it
because they mocked the lookup and faked `AutoCreated: true`.

**Fix:** split the `CampaignLookup` interface so the gate runs *between*
detection and persistence:

- `FindCampaignForSetup(guildID) (info, exists, err)` — never creates.
- `CreateCampaignForSetup(guildID, invokerUserID)` — called only after the admin
  gate passes.
- Handler reordered: find → (exists? DM gate) / (not exists? admin gate → create)
  → channels. Dropped the now-unused `SetupCampaignInfo.AutoCreated`; the success
  prefix keys off the local `!exists`.

Proof (red→green):
- e2e `TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign` asserts the reject
  message **and** `GetCampaignByGuildID → sql.ErrNoRows`. RED on pre-fix code
  (`err = <nil>`, a campaign existed); GREEN after.
- unit `TestHandleSetupCommand_RejectsNonAdminAutoCreate` / `RejectsNonDM…` now
  assert `createCalled == false`.

### Harness feature added (reusable)

**Permission injection** — `interaction.Member.Permissions` was hardcoded zero.

- `e2eHarness.PlayerCommandWithPermissions(userID, name, permissions, opts…)`;
  `PlayerCommand` now delegates with `0`.
- `harnessDispatcher.permissions` threaded into replay dispatch.
- Preconditions sidecar gains `"dispatchAsAdmin": true` → sets the
  `discordgo.PermissionAdministrator` bit. Generalizes to any future
  permission-gated command.

Non-DM / non-admin identities need **no** new field: non-DM dispatches as a
plain `player` ≠ the seeded campaign's random DM; non-admin uses default perms 0.

### Artifacts

- Transcripts (+ `.preconditions.json` sidecars) in `internal/playtest/testdata/`:
  - `setup_autocreate_admin.jsonl` — `{player, dispatchAsAdmin:true}`, no campaign.
  - `setup_reject_nondm.jsonl` — `{campaign, player:"intruder-not-dm"}`.
  - `setup_reject_nonadmin.jsonl` — `{player:"normie-no-admin"}`, no campaign.
- DB-lock scenario (`cmd/dndnd/e2e_scenarios_test.go`):
  - `TestE2E_SetupAutoCreateScenario` — campaign auto-created (admin = DM) + 10
    channel IDs persisted in settings JSONB.
  - `TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign` — regression lock for
    the bug (no campaign row created on reject).
- Prod fix: `internal/discord/setup.go`, `cmd/dndnd/discord_adapters.go`.

### Run it

```sh
for t in setup_autocreate_admin setup_reject_nondm setup_reject_nonadmin; do
  make playtest-replay TRANSCRIPT=$(pwd)/internal/playtest/testdata/$t.jsonl
done
make e2e   # all 11 scenarios + replays green
```

### Verification

- `make e2e` → 11/11 PASS (incl. both new setup scenarios + the 3 replays).
- unit: `internal/discord` + `cmd/dndnd` setup/adapter tests PASS; gofmt + vet clean.
- `make cover-check`: overall 90.58%, `internal/discord` 85.74% (≥85). Only
  `internal/refdata` 84.13% misses — **pre-existing DB-flaky gate, untouched here**
  (see memory `project_cover_check_refdata`).

### Deferred / notes

- Real-Discord `Member.Permissions` arrives resolved from the gateway; the fake
  now mirrors that field, so the in-process path matches.
- The auto-create success message has no dashboard link because `BASE_URL` is
  empty in the harness (the no-link `nextStep` variant).
