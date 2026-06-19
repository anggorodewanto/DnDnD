# AI Playtest Harness — Ledger (live track record)

> This is our **memory**. Update it **every session**. See
> [`README.md`](README.md) for how we work. The **current task** is always the
> first row below whose status is not `DONE`/`AUTOMATED`.

## Lifecycle / status legend

- `TODO` — not started.
- `EXPLORE` — understanding how the step works today.
- `AUTHOR` — building mode: running the step interactively, deciding assertions.
- `CRYSTALLIZE` — turning the confirmed step into a replayable case.
- `AUTOMATED` — case runs unattended (e.g. `make playtest-replay` / `make e2e`); green.
- `DONE` — non-test tasks (e.g. exploration) that are complete.
- `BLOCKED` — waiting on a decision or a dependency (note why).

---

## Decisions log

Record every settled decision here so a fresh agent doesn't re-litigate it.

| Date | Decision |
| --- | --- |
| 2026-06-19 | Harness goal = **all four** (bug hunting / acceptance / regression / living docs); achieved via the per-step lifecycle, not separate suites. |
| 2026-06-19 | Autonomy = **interactive while building a case**, **unattended once crystallized**. |
| 2026-06-19 | Discord-driving mechanism = **decide after exploration** (STEP-000 must recommend). |
| 2026-06-19 | **(STEP-000)** Mechanism = **hybrid anchored on the in-process e2e harness**. Player actions via `InjectInteraction`/`PlayerCommand` + assert `fake.Transcript()`; crystallize to `.jsonl`, run with `make playtest-replay`. DM setup via `/setup` + `SeedCampaign`/dashboard APIs. Real Discord (`cmd/playtest-player` paste flow) = periodic human-assisted smoke only, never in the auto loop. *Why:* bot-to-bot slash invocation is forbidden + user-token automation violates ToS, so real-Discord player input can't be automated; the harness runs the real router/handlers/DB with only the Discord wire faked. (Awaiting user sign-off.) |
| 2026-06-19 | **(STEP-000)** Crystallized cases = `.jsonl` in `internal/playtest/testdata/`, replayed by `TestE2E_ReplayFromFile`; `observed` `content` lines ARE the assertion (substring match after `DefaultNormalize`). |
| 2026-06-19 | **Real-Discord lane = human-assisted manual smoke only** (playtest-player paste). **Rejected: automating discordo / any self-bot** — user-token automation violates Discord ToS + risks account ban; "avoid detection" jitter = enforcement evasion, out of scope. No compliant way to auto-invoke another app's slash commands as a user. |
| 2026-06-19 | **Jitter / randomness / varied-content → build as a harness FUZZING + timing layer** (randomized inter-command timing + varied input on the in-process harness, to surface races/timing/input bugs). NOT a real-Discord evasion feature. Backlog item, after core steps. |
| 2026-06-19 | **STEP-001 = Player `/register`** (first authored step). |
| 2026-06-19 | **(STEP-001)** Crystallize route = **B**: per-transcript preconditions sidecar (`<transcript>.preconditions.json`) + `.jsonl` for Discord-visible assertions; DB row stays in the existing Go scenario. `.jsonl` replay can't assert DB — deferred replay-engine enhancement if ever needed. |
| 2026-06-20 | **(STEP-002)** Authored full `/move` in 2 increments (002a preconditions+confirm, 002b click model). Added transcript `click` direction (selector = CustomID **prefix**, since live CustomID embeds runtime UUIDs) + `Clicker` engine interface + `ClickButton` harness helper + preconditions `encounter` block. These generalize to all confirm-gated commands. |

### Open questions awaiting answers

- Q1: ~~How does the AI drive Discord?~~ **RESOLVED** by STEP-000 (see decisions). Awaiting user sign-off.
- Q2: Smallest first real-world step to author? → proposed to user (STEP-000 QnA). Candidates: DM `/setup`, player `/register`, player `/move`.
- Q3: ~~Crystallized-case format + location?~~ **RESOLVED**: `internal/playtest/testdata/*.jsonl`.
- Q4 (new): Do we want a true real-Discord smoke lane at all, or harness-only? → ask user.

---

## Steps

| ID | Real-world step | Phase | Artifact (test/case path) | Notes / bugs |
| --- | --- | --- | --- | --- |
| STEP-000 | Explore existing harness & refine the plan | `DONE` | docs only | Inventory confirmed (README §6), mechanism recommended (README §7). 5 reference scenarios + record/replay already exist. |
| STEP-001 | Player `/register` (create character) | `AUTOMATED` ✅ | `internal/playtest/testdata/register.jsonl` (+`.preconditions.json`) | `make playtest-replay TRANSCRIPT=…/register.jsonl` → PASS. Added per-transcript preconditions to `cmd/dndnd/e2e_replay_test.go`. DB row locked by `TestE2E_RegistrationScenario`. See [steps/STEP-001-player-register.md](steps/STEP-001-player-register.md). |
| STEP-002 | Player `/move` one tile (button-confirm) | `AUTOMATED` ✅ | `internal/playtest/testdata/move.jsonl` (+`.preconditions.json`) | Full confirm flow. Added: preconditions `encounter` block, transcript `click` direction, `Clicker`/`harnessClicker`, `ClickButton` helper. DB position locked by `TestE2E_MovementScenario`. See [steps/STEP-002-player-move.md](steps/STEP-002-player-move.md). |
| STEP-003 | *(next — pick via QnA)* | `TODO` | — | Confirm-flow commands now cheap (reuse `click`/`Clicker`): `/attack`, `/cast`. Or DM `/setup`. |

### Refined backlog (smallest-first; mechanics noted)

Order reflects the real play journey **and** harness mechanics. "Covered?" =
whether a reference `TestE2E_*Scenario` already exists to template from.

| # | Real-world step | How to drive | Covered? |
| --- | --- | --- | --- |
| pre | Campaign exists | `SeedCampaign` (dashboard API in real life) | n/a (seeded) |
| 1 | DM runs `/setup` (build channel structure) | inject `/setup` interaction | no — net-new |
| 2 | Player `/register` (create character) | `PlayerCommand` | yes (Registration) |
| 3 | DM approves the character | dashboard API / seed `SeedApprovedPlayer` | partial |
| 4 | DM starts an encounter | dashboard API / seed | partial |
| 5 | DM loads a map / places tokens | dashboard API / seed | partial |
| 6 | Player `/move` one tile | `PlayerCommand` | yes (Movement) |
| 7 | Combat: initiative | inject/seed | no |
| 8 | Player `/attack` or `/cast` (one action) | `PlayerCommand` | partial |
| 9 | Damage + condition applied | assert transcript + DB | no |
| 10 | Player `/loot`, `/give`, `/save`, `/recap` | `PlayerCommand` | yes (Loot/Save/Recap) |

> Note: "Covered" scenarios are *reference examples*, not proof the journey is
> regression-locked. Re-walking them as small authored steps is still valuable —
> it confirms current behavior and turns each into a maintained `.jsonl` case.

### Cross-cutting feature backlog (not a play step)

- **Harness fuzzing + timing layer** — randomized inter-command timing (jitter)
  and varied input content driven into the in-process harness to surface race
  conditions / timing / input-handling bugs. Deterministic when seeded. Build
  *after* a few core steps exist to fuzz against. (See decisions log, 2026-06-19.)

---

## Session log

Append a short entry per working session: date, step touched, what happened,
what's next.

| Date | Step | What happened | Next |
| --- | --- | --- | --- |
| 2026-06-19 | — | Set up `docs/ai-playtest/` (README + this ledger). Captured mission, lifecycle, modes, rules. | Run STEP-000 (explore + refine plan). |
| 2026-06-19 | STEP-000 | Explored harness via 2 read-only subagents + verified claims. Found in-process e2e harness (real router/DB, fake Discord wire) + record/replay + 5 reference scenarios. Recommended hybrid mechanism. Updated README §6/§7 + ledger. | User sign-off on mechanism + pick STEP-001 (QnA sent). |
| 2026-06-19 | STEP-001 | EXPLORE'd `/register` (subagent). Hit harness limit: replay seeding fixed → built per-transcript preconditions feature. Crystallized `register.jsonl` (+sidecar); replay PASS first run; backward compat + gofmt/vet clean. | Commit/push (ask user), then pick STEP-002. |
| 2026-06-19 | STEP-001 | Committed+pushed (489e0d1). Demo'd drift (red) on a temp copy to prove the assertion bites. | Start STEP-002. |
| 2026-06-20 | STEP-002 | EXPLORE'd `/move` (subagent). Built encounter preconditions (002a, confirm ephemeral green) + transcript click model & Clicker (002b). `move.jsonl` PASS; full regression green (playtest unit, replay group, movement+registration scenarios). gofmt/vet clean. | Commit/push, pick STEP-003. |
| 2026-06-20 | STEP-002 | Committed+pushed (cec763f). **Session paused at user request** — clean stopping point: STEP-001 + STEP-002 + reusable infra (preconditions incl. encounter, dispatch/observe/click model, Clicker/ClickButton) all on `main`. | Resume: read README.md + this ledger; pick STEP-003 (candidates: `/attack` reusing click model, or DM `/setup`). |
