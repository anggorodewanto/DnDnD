# STEP-003 — DM `/setup` (build channel structure)

Phase: **AUTOMATED** ✅ (crystallized + green). See [../README.md](../README.md).

## RESULT (2026-06-20)

First **DM/admin** step (all prior steps were player commands). Authored the
**existing-campaign DM happy path** — the smallest viable slice, because that
permission gate checks **identity only** (`invoker == campaign DM`) and needs no
`Member.Permissions` bit, which the harness cannot set today.

- **Artifacts:**
  - `internal/playtest/testdata/setup.jsonl` — dispatch `/setup` → deferred ack
    → public success edit.
  - `internal/playtest/testdata/setup.preconditions.json` — seeds a campaign and
    dispatches **as the campaign DM** (`"dispatchAsDM": true`).
  - `cmd/dndnd/e2e_scenarios_test.go` → `TestE2E_SetupScenario` — locks the
    DB side (10 channel IDs persisted in campaign settings JSONB), which the
    `.jsonl` replay cannot see.
- **Run it:**
  - `make playtest-replay TRANSCRIPT=$(pwd)/internal/playtest/testdata/setup.jsonl` → PASS (~2.4s)
  - `go test -tags e2e ./cmd/dndnd/ -run TestE2E_SetupScenario -count=1` → PASS

### Harness feature added (reusable)

**Preconditions `dispatchAsDM` flag** (`cmd/dndnd/e2e_replay_test.go`) — when
true, the replay dispatcher is re-targeted at the seeded campaign's `DmUserID`
instead of `Player`. Needed because `testutil.NewTestCampaign` assigns a random
`dm-<uuid>` DM id, unknowable when authoring the transcript. Generalizes to any
future DM-gated command whose check is `invoker == campaign DM`.

### Confirmed actual output (live harness, signed off by user)

```
[1] interaction_response (deferred ack — empty content)
[2] interaction_edit  "Channel structure created successfully! 10 channels set up.\n\nNext: open the dashboard to build a map, then create an encounter."
```

- The success message arrives via **`InteractionResponseEdit`** (the handler
  defers first), not a direct response → the `.jsonl` needs an **empty first
  observed line** to consume the deferred ack (the observer is strictly
  sequential).
- **Non-ephemeral** (public). Channel count = **10** text channels
  (SYSTEM/NARRATION/COMBAT/REFERENCE = 3+3+2+2).
- `BASE_URL` is empty in the harness → the no-link `nextStep` variant.

### EXPLORE findings (key facts that shaped the test)

- `/setup` is **not** in `playtest.PlayerCommands`, but `Replay` never validates
  against that list (only the playtest-player REPL does) — so a `.jsonl` can
  dispatch it freely.
- Two permission gates (`internal/discord/setup.go:244-251`): existing-campaign
  path = identity only; auto-create path = admin bit. We authored the **former**.
- The fake records `InteractionRespond` / `InteractionResponseEdit` but **not**
  `GuildChannelCreateComplex` — so channel-creation side effects are invisible to
  the transcript; the DB query in `TestE2E_SetupScenario` is what locks them.

### Coverage notes / deferred (backlog) — ✅ RESOLVED by STEP-004

- ~~**Auto-create admin path** needs the harness to set
  `interaction.Member.Permissions`.~~ Done in STEP-004 via
  `PlayerCommandWithPermissions` + the `dispatchAsAdmin` precondition.
- ~~Permission **rejection** messages are unit-only.~~ Now e2e too (3 replay
  transcripts + scenarios). STEP-004 also **fixed an auth bug** found while
  authoring this path (non-admin reject silently created a campaign). See
  [STEP-004-dm-setup-permissions.md](STEP-004-dm-setup-permissions.md).
