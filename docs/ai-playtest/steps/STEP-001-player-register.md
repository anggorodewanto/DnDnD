# STEP-001 — Player `/register` (create character)

Phase: **AUTOMATED** ✅ (crystallized + green). See [../README.md](../README.md).

## RESULT (2026-06-19)

Crystallized as a replayable transcript + a small reusable harness feature.

- **Artifacts:**
  - `internal/playtest/testdata/register.jsonl` — the case (dispatch + 2 observed).
  - `internal/playtest/testdata/register.preconditions.json` — its seed manifest.
- **Run it:** `make playtest-replay TRANSCRIPT=$(pwd)/internal/playtest/testdata/register.jsonl` → PASS (3.1s).
- **Harness feature added** (`cmd/dndnd/e2e_replay_test.go`): per-transcript
  preconditions — a sidecar `<transcript>.preconditions.json` (`campaign`,
  `player`, `approvedPlayers`, `placeholderCharacters`) seeds the harness before
  replay; no sidecar → legacy default (campaign + approved "Gale"). Backward
  compat verified: `TestE2E_Replay{Roundtrip,DetectsDrift,FromFile-default}` green.
- **Confirmed actual strings** (replay matched on first run):
  - ephemeral: `✅ Registration submitted — Aria is pending DM approval. You'll be pinged when approved.`
  - DM-queue: `🆕 <@DM> — **Aria** registration by <@user-register> via /register. Pending approval.`
- **DB row** (`player_characters` status=pending): NOT in `.jsonl` (replay is
  Discord-visible only) — still locked by `TestE2E_RegistrationScenario` in `make e2e`.
- **Deferred gap:** `.jsonl` replay can't assert DB state. If we want DB
  assertions in transcripts later, that's a separate replay-engine enhancement.

---

### Original EXPLORE/AUTHOR notes below

## Goal

Author a replayable `.jsonl` case for a player registering an existing
DM-created character via `/register name:<name>`, against the in-process harness.

## EXPLORE findings (confirmed in code)

- **Command:** `internal/discord/commands.go:552` — `/register`, one optional
  string option `name`. With no name → onboarding chooser (3 buttons). With a
  name → claim/register that character.
- **Handler:** `internal/discord/registration_handler.go:120` `registerByName`.
- **Responses (ephemeral, quoted):**
  - Exact match → `✅ Registration submitted — <name> is pending DM approval. You'll be pinged when approved.` (`:137`)
  - Fuzzy → `❌ No character named "<name>" found. Did you mean: **X**, **Y**? Use /register X to confirm.` (`:146`)
  - No match → `❌ No character named "<name>" found. No close matches available.` (`:150`)
  - No campaign → `No campaign found for this server.` (`:124`)
- **Side effect:** posts to DM-queue channel (`ch-dmqueue-<guild>`) — content
  contains the character name, `register`, and the player ID. (Exact format
  string NOT yet quoted; confirm at AUTHOR-run time, assert as substrings.)
- **DB write:** creates a `player_characters` row, `status="pending"`,
  `created_via="register"`.
- **Preconditions:** seeded campaign (`SeedCampaign`) + a DM-curated placeholder
  character (`SeedCharacterOnly("Aria")`, NOT linked to a player). Player not
  already approved.
- **Reference scenario:** `cmd/dndnd/e2e_scenarios_test.go` `TestE2E_RegistrationScenario`
  (lines 44-114) — seeds campaign + `SeedCharacterOnly("Aria")`, runs
  `PlayerCommand("user-register","register", stringOpt("name","Aria"))`, asserts
  ephemeral contains `Registration submitted`, DM-queue msg contains Aria/register/playerID,
  pending row created, then `SeedDMApproval` flips to approved + approval DM.

## OPEN before crystallize (resolve at AUTHOR-run time)

1. Exact `channel_id` + `author` values the fake records for an **ephemeral**
   response — needed to write a matching `observed` line for replay. Determine by
   running the harness and dumping `fake.Transcript()` / `RenderTranscript()`.
2. Exact DM-queue notification format string (`postDMQueueNotification`).

## Draft transcript (to be replaced by observed reality)

```jsonl
{"dir":"dispatch","channel_id":"ch-cmd-<guild>","author":"user-register","command":"/register name:Aria"}
{"dir":"observed","channel_id":"<tbd>","author":"bot","content":"Registration submitted — Aria is pending DM approval."}
```

## Assertion scope — DECIDED: **Rich** (ephemeral + DM-queue + pending DB row).

## BLOCKING FINDING (AUTHOR-run, 2026-06-19)

`make playtest-replay` → `TestE2E_ReplayFromFile` (`cmd/dndnd/e2e_replay_test.go:140`)
seeds a **fixed** precondition set: `SeedCampaign("replay-file-campaign")` +
`SeedApprovedPlayer("user-file","Gale")`. It does **not** seed an unlinked
placeholder character, so a `register.jsonl` cannot reach the happy path
(`Registration submitted`) — it lands on the no-match error path.

Two gaps in the `.jsonl` replay route for a *Rich* `/register` case:
1. **No per-transcript preconditions** — can't declare "seed placeholder char Aria".
2. **No DB assertion** — `replay.go` matches Discord-visible `content` only
   (substring, sequential; `channel_id`/`author` ignored). The pending-row check
   can't live in a transcript.

Note: the existing Go scenario `TestE2E_RegistrationScenario` **already** asserts
the Rich set (ephemeral + DM-queue + pending row + approval), and runs in `make e2e`.

## Direction options (user QnA)

- **A — Go scenario:** crystallize STEP-001 as a Go e2e test (Rich, incl. DB). Fast; overlaps the existing reference scenario. No `.jsonl`.
- **B — Build transcript preconditions, then `register.jsonl`:** small harness feature so transcripts declare their setup; `register.jsonl` replays the happy path (ephemeral + DM-queue). DB row stays locked by the existing Go scenario. Builds the reusable record→replay capability.
- **C — Both:** B + a dedicated Go scenario. Most complete, most work.
