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

### Open questions awaiting answers

- Q1: How does the AI drive Discord as DM + player(s)? → STEP-000.
- Q2: Smallest first real-world step to author? (candidate: "DM creates a campaign") → STEP-000.
- Q3: Crystallized-case format + location? (candidate: `internal/playtest/testdata/*.jsonl`) → STEP-000.

---

## Steps

| ID | Real-world step | Phase | Artifact (test/case path) | Notes / bugs |
| --- | --- | --- | --- | --- |
| STEP-000 | Explore existing harness & refine the plan | `TODO` | — | First task. See README §8. No test authored here. |

### Candidate backlog (unstarted — order to be confirmed by STEP-000)

These are *guesses* at the real-world step sequence, smallest-first. STEP-000
will confirm, reorder, and split them. Do not start any until STEP-000 is done.

1. DM creates a campaign.
2. DM creates / opens a play channel (or session).
3. Player joins the campaign.
4. Player rolls / creates a character.
5. DM starts an encounter.
6. DM places tokens / loads a map.
7. Player moves one tile.
8. Combat: initiative is rolled.
9. Combat: a player takes one action (attack / cast).
10. Combat: damage + condition applied.

---

## Session log

Append a short entry per working session: date, step touched, what happened,
what's next.

| Date | Step | What happened | Next |
| --- | --- | --- | --- |
| 2026-06-19 | — | Set up `docs/ai-playtest/` (README + this ledger). Captured mission, lifecycle, modes, rules. | Run STEP-000 (explore + refine plan). |
