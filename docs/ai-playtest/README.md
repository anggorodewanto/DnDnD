# AI Playtest Harness — Agent Guide & Pickup Prompt

> **You are an AI agent picking this up.** Read this whole file, then open
> [`LEDGER.md`](LEDGER.md), find the first step that is not `DONE`/`AUTOMATED`,
> and continue from there. Do **not** try to do everything at once. Do **one
> small real-world step** per working session, in the mode that step is in.

---

## 1. Mission

Build a test harness in which an **AI agent plays both the DM and the
player(s)** and exercises DnDnD through **real play flows** — DM sets up a
campaign, a player rolls a character, an encounter starts, someone moves on the
map, combat resolves, and so on.

This is the **next level beyond static test cases**. Instead of hand-writing
fixed assertions up front, each real-world play step is:

1. actually *run and explored* with a human in the loop,
2. confirmed to behave correctly (bugs found and logged along the way),
3. then *crystallized into an automated test case* that runs unattended forever
   after.

The harness therefore serves **all four** of these purposes at once — the only
difference between them is **which lifecycle phase a given step is in**:

| Purpose | When it happens |
| --- | --- |
| **Exploratory bug hunting** | during EXPLORE / AUTHOR — free play surfaces breakage |
| **Feature acceptance** | during AUTHOR — confirm the feature works end-to-end |
| **Regression safety net** | after CRYSTALLIZE — the case replays unattended |
| **Living documentation** | the authored step doc *is* a readable record of real play |

The user already has a working Discord server wired to DnDnD; using it for true
end-to-end play is on the table (see the open question in §7).

---

## 2. The test-case lifecycle (the central idea)

Every **real-world play step** moves through four phases. A step is the
smallest meaningful player/DM action (e.g. "DM creates a campaign", "player
rolls a character", "player moves one tile"). Keep steps **as small as
possible** — if a step has two observable outcomes, it is probably two steps.

```
EXPLORE  ──►  AUTHOR  ──►  CRYSTALLIZE  ──►  AUTOMATED
(building mode, interactive)              (automation mode, unattended)
```

1. **EXPLORE** — Understand how this step works *today*. Which command(s)
   trigger it? What code path runs? What Discord output and dashboard/DB state
   should result? Read code, don't guess. Interactive.

2. **AUTHOR** *(building mode — interactive)* — Drive the step against the
   chosen Discord mechanism while acting as DM/player. Observe the **real**
   behavior. With the user, decide the **expected outcome and the assertions**.
   This is where bugs are found (exploratory) and features are accepted. **Never
   assume the "correct" D&D-rules outcome** — confirm it with the user via
   `AskUserQuestion`.

3. **CRYSTALLIZE** — Once the user confirms the observed behavior is correct,
   turn the step into a **deterministic, replayable test case** (a recorded
   transcript / scenario the harness can run). Add it to the suite. Do **not**
   crystallize behavior the user has not confirmed is correct.

4. **AUTOMATED** *(automation mode — unattended)* — The case now runs in
   `make e2e` / replay with no human. A failure means a regression.

---

## 3. The two operating modes

- **Building mode (interactive).** Authoring a *new* step. Expect heavy use of
  `AskUserQuestion`: confirm expected behavior, propose assertions for sign-off,
  surface anything surprising. Slow and deliberate by design.

- **Automation mode (unattended).** Replaying *crystallized* cases. Fully
  deterministic, no human, CI-friendly. This is what a step looks like once it
  has graduated.

> A step starts in building mode and graduates to automation mode. The harness
> is "done" for a step when its case runs green in automation mode.

---

## 4. Working rules (read before every session)

1. **One small real-world step per session.** Pick the current step from
   `LEDGER.md`. Do not boil the ocean. Do not implement multiple steps at once.
2. **Smallest steps possible.** Split aggressively. A step should map to a
   single player/DM intention with a checkable outcome.
3. **Interactive while authoring.** In building mode, ask the user to confirm
   expected behavior *before* you write assertions, and again *before* you
   crystallize. Use `AskUserQuestion`.
4. **Red/green TDD spirit.** State the expected outcome (the assertion) first,
   then observe whether reality matches. A mismatch is either a bug to log or an
   assumption to correct — decide which *with the user*.
5. **Document-driven memory.** After every session, update `LEDGER.md` (status,
   artifact path, bugs found, decisions). The ledger is the source of truth for
   "where are we"; this file is the source of truth for "how we work".
6. **Don't crystallize unconfirmed behavior.** Automating a wrong outcome bakes
   in a bug.
7. **Log bugs, don't silently fix.** If a step reveals a bug, record it in the
   ledger. Fixing it is a separate decision for the user.
8. **Respect the project's own rules.** Orchestrate (delegate exploration to
   subagents), follow TDD, run `make cover-check` when you touch Go, and obey
   [`/CLAUDE.md`](../../CLAUDE.md) and [`docs/testing.md`](../testing.md).

---

## 5. Document map (our memory)

| File | Role | Who updates it |
| --- | --- | --- |
| `README.md` (this file) | Stable: mission, lifecycle, rules, current open questions. Changes rarely. | Updated only when *how we work* changes. |
| [`LEDGER.md`](LEDGER.md) | Live: every step, its lifecycle phase, artifact path, bugs, decisions. The track record. | Updated **every session**. |
| `steps/STEP-XXX-<slug>.md` *(created as needed)* | Per-step authoring notes: explore findings, expected outcomes, assertions, the resulting test artifact. | Created during AUTHOR for non-trivial steps. |

Resume protocol for a fresh agent: **read this file → read `LEDGER.md` → open
the current step's `steps/` note (if any) → continue.**

---

## 6. What we already have (confirmed by STEP-000, 2026-06-19)

The **in-process e2e harness is the backbone** — it boots the *real* production
app (real `CommandRouter`, real handlers, real testcontainers Postgres) and only
**fakes the Discord transport**. This is what lets an AI drive play deterministically.

- **`cmd/dndnd/e2e_harness_test.go`** — `e2eHarness`: boots production
  `runWithOptions(...)` in-process with a fake Discord session + a fresh
  testcontainers Postgres. Helpers: `SeedCampaign`, `SeedApprovedPlayer`,
  `PlayerCommand(playerID, name, opts...)`, `AssertEphemeralContains`. **This is
  where AUTHOR happens.**
- **`internal/testutil/discordfake/fake.go`** — the fake `discord.Session`.
  `SetInteractionHandler(router.Handle)` + `InjectInteraction(*discordgo.Interaction)`
  dispatch a command **in-process, no real Discord**; `Transcript() []Entry`
  captures every outbound message/embed for assertions. **This is the seam.**
- **`internal/discord/router.go:362` — `CommandRouter.Handle(*discordgo.Interaction)`** —
  the single dispatch boundary. Same code path for real Discord and the fake.
- **`cmd/dndnd/main.go` `runWithOptions` / `withDiscordSession` / `withCommandRouterReady`** —
  the production seam that injects the fake session and exposes the router.
- **`internal/playtest/`** — record/replay. `recorder.go` `TranscriptEntry`
  schema = `{ts, dir(dispatch|observed), channel_id, author, command, content}`.
  The **`observed` `content` lines ARE the assertion** (substring match after
  `DefaultNormalize`: UUIDs → `<uuid>`, whitespace collapsed). `parser.go` has a
  `PlayerCommands` allow-list that **rejects `/setup` and other DM/admin commands** —
  enforcing the DM/player split. Transcripts: `testdata/{sample,combat_round}.jsonl`.
- **`cmd/playtest-player/`** — a REPL that connects to a **real Discord gateway**
  (`live_session.go`, bot token). Because Discord forbids bot-to-bot slash
  invocation, it prints `PASTE THIS` lines for a **human** to paste, then records
  the round-trip the bot's response makes back through the gateway. Real-Discord,
  human-in-the-loop only.
- **`make e2e`** → `go test -tags e2e ./cmd/dndnd/ -run TestE2E_ -count=1 -v`.
  5 reference scenarios already exist: `TestE2E_{Registration,Movement,Loot,Save,RecapEmpty}Scenario`.
- **`make playtest-replay TRANSCRIPT=<path>`** → `TestE2E_ReplayFromFile` replays
  a `.jsonl` transcript through the harness. **The backbone of AUTOMATED.**
- **DM setup is split:** `/setup` (build channel structure) is a slash command;
  but **campaign creation, character approval, encounter setup are dashboard HTTP
  APIs** (`cmd/dndnd/dashboard_apis.go`, OAuth-gated), not slash commands. The
  harness side-steps these with `SeedCampaign` / `SeedApprovedPlayer` / direct DB.
- **`docs/playtest-checklist.md`** — 11 human scenarios = a good backlog source.
- **`docs/testing.md`** — test pyramid, `internal/testutil`, coverage exclusions.
- **`make local-up`** — full app + Postgres (live bot gateway, for real-Discord lane).

---

## 7. How the AI drives Discord — RESOLVED (STEP-000, awaiting user sign-off)

**Recommendation: a hybrid anchored on the in-process e2e harness.** Real
Discord cannot drive *player* input automatically — bot-to-bot slash invocation
is forbidden and user-token automation violates Discord ToS — so the automated
pipeline runs in-process, and real Discord is a separate human-assisted lane.

- **Player actions (the bulk):** in-process harness. AI builds an interaction,
  calls `PlayerCommand` / `InjectInteraction`, asserts on `fake.Transcript()`.
  Crystallize to a `.jsonl` transcript; `make playtest-replay` runs it unattended.
  Exercises the *real* router/handlers/DB — only the Discord wire is faked.
- **DM setup actions:** `/setup` is an injectable slash command. Campaign create
  / approval / encounter setup are dashboard HTTP APIs — drive via the harness's
  `SeedCampaign` / `SeedApprovedPlayer` / direct calls (browser automation of the
  dashboard only if we want true UI coverage later).
- **Real Discord lane (periodic, human-in-the-loop):** `cmd/playtest-player`
  against `make local-up`. A human pastes the `PASTE THIS` lines; the round-trip
  is recorded. Purpose: confirm the live gateway matches the fake. **Never** in
  the automated regression loop.

Settled sub-answers:
- Crystallized cases = `.jsonl` transcripts in `internal/playtest/testdata/`,
  replayed by `TestE2E_ReplayFromFile`. The `observed` lines are the assertions.
- `/setup` is DM-only and rejected by the player REPL's `PlayerCommands` list.

---

## 8. CURRENT TASK

> The active task is always the first non-`DONE` row in
> [`LEDGER.md`](LEDGER.md). At time of writing that is **STEP-000**.

**STEP-000 — Explore the existing harness and refine this plan.** *(Do not
author or write any test case yet.)*

Deliverables:

1. **Inventory the existing harness.** Confirm the §6 facts by reading the code:
   `cmd/playtest-player/`, `internal/playtest/`, the `TestE2E_*` tests in
   `cmd/dndnd/`, the `.jsonl` transcript format, and the existing playtest docs.
   Delegate this to a subagent (read-only) to save context.
2. **Answer open question #1.** Assess each Discord-driving mechanism's
   feasibility (real-Discord constraints, ToS, what playtest-player can already
   do) and **recommend one** (or a hybrid), with reasons.
3. **Identify the smallest first real-world step** to author next, and a short
   backlog of the next few (in dependency order).
4. **Refine this plan.** Update §6/§7 of this file with confirmed facts, and
   write the recommendation + backlog into `LEDGER.md`.
5. **Report back via `AskUserQuestion`** before any authoring begins — present
   the recommended Discord mechanism and the proposed first step for sign-off.

When STEP-000 is done, mark it `DONE` in the ledger and the next step becomes
current.
