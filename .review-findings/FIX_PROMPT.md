# Prompt: Remediate DnDnD Implementation Review Findings

You are the **orchestrator** for a remediation campaign. Your job is to drive every finding in `/home/ab/projects/DnDnD/.review-findings/` to closure using a worker-and-reviewer subagent pattern, persisting all coordination state to disk so neither you nor any worker has to hold the full plan in context.

## Mission

Address every finding from the prior implementation review. Land any remaining Phase 121 work. Confirm a fresh contributor can complete the `docs/playtest-quickstart.md` flow end-to-end without manual intervention. Do not stop until that final confirmation lands as a written verdict on disk.

## Context to load first

Read these in order before doing anything else:

1. `/home/ab/projects/DnDnD/.review-findings/SUMMARY.md` — severity-tiered list of ~50 findings with file:line refs. This is the canonical work list.
2. `/home/ab/projects/DnDnD/.review-findings/chunk{1..8}_*.md` — detail per phase chunk; cite when needed but do not preload all eight.
3. `/home/ab/projects/DnDnD/CLAUDE.md` and `/home/ab/.claude/CLAUDE.md` — house rules: red/green TDD, ≥90% overall / ≥85% per-package coverage, run `/simplify` after coding, early-return style, ask before destructive ops.
4. `/home/ab/projects/DnDnD/docs/dnd-async-discord-spec.md` — original spec, only read the sections referenced by the finding you are working on.
5. `/home/ab/projects/DnDnD/docs/phases.md` Coverage Map at line 833 — maps spec sections to phases so reviewers can re-check Done When criteria.
6. `/home/ab/projects/DnDnD/docs/playtest-quickstart.md` — the final acceptance gate.

## File-based coordination

Create `/home/ab/projects/DnDnD/.fix-state/` and use it as your durable scratch. Every coordination artifact lives there as a markdown file so subagents can read state without you re-explaining context:

- `tasks.md` — full checklist of findings extracted from `SUMMARY.md`. One row per finding with: `id` (e.g. `crit-01`, `high-08`, `med-18`), severity, one-line title, status (`pending` / `claimed` / `in_review` / `revisit` / `done`), worker agent ID, current `task_<id>.md` revision counter.
- `batches.md` — append-only log of which task IDs were dispatched together and when.
- `reviews.md` — append-only log of reviewer verdicts.
- `log.md` — append-only orchestrator decisions, blockers, ordering choices.
- `task_<id>.md` — one file per finding. Worker writes here, reviewer appends a verdict section. Schema:
  ```
  # task <id> — <title>
  ## Finding (verbatim from SUMMARY.md or chunkN.md)
  ## Plan (worker fills)
  ## Files touched
  ## Tests added
  ## Implementation notes
  ## Review (reviewer fills) — Verdict: PASS | REVISIT
    - reasons / unaddressed gaps / spec-section refs
  ```
- `playtest_ready.md` — final verdict file, only written when the playtest-quickstart self-test passes clean.

## Worker pattern

Spawn the following kinds of subagents via the `Agent` tool. Always brief them with the absolute path to their task file plus the spec section to consult — never paste the finding text into the prompt; they read it from disk.

- **Implementer** (`subagent_type: general-purpose`). Owns one `task_<id>.md`. Workflow: read task file → read spec section → write a failing test → minimal fix → re-run targeted tests → run `make cover-check` → run `/simplify` semantics by hand on the touched code (collapse duplication, drop dead fields, no premature abstraction) → update `task_<id>.md` with files touched, tests added, notes. No commits — orchestrator commits per batch.
- **Reviewer** (`subagent_type: code-reviewer`). Reads `task_<id>.md`, the diff against `HEAD`, the spec section the task cites, and the original chunk findings file. Confirms the fix matches Done When, no scope creep, tests cover the failure mode, no regression. Appends `## Review` section with `Verdict: PASS` or `Verdict: REVISIT` plus specifics. Never edits source.
- **Integration verifier** (`subagent_type: general-purpose`). Runs `make cover-check`, `make test`, `make e2e`, `make playtest-replay TRANSCRIPT=internal/playtest/testdata/sample.jsonl`. Captures failures with logs. Spawned after every Critical and High batch closes, and again after Medium and Low close.
- **Playtest-quickstart verifier** (`subagent_type: general-purpose`). Walks through `docs/playtest-quickstart.md` from a clean state on a throwaway branch / worktree. Reports each step's outcome to `playtest_ready.md`. If a step requires a real Discord guild it cannot reach, it must say so explicitly and propose either a doc fix or a harness substitute, then loop back to the orchestrator.

Spawn implementers in parallel within a batch when their file lists do not overlap. Reviewers run in parallel after their implementer reports. Use a single `Agent` message with multiple tool calls when launching parallel work.

## Ordering

Process severity tiers in order: Critical → High → Medium → Low. Within a tier, batch by domain to minimize merge conflicts:

- Critical batch A — slash-command wiring (items 1, 2 from SUMMARY): one task per command family, all editing `cmd/dndnd/main.go` + `cmd/dndnd/discord_handlers.go` + the matching `internal/discord/*_handler.go`.
- Critical batch B — pipeline plumbing (items 3, 4, 5): damage pipeline integration, FES context plumbing, turn-lock enforcement. Each is its own task because each touches a different surface.
- Critical batch C — auth/notifier nils (items 6, 7).
- High, Medium, Low — group by package: character cards, roll history, combat map, spells, magic items, dashboard surface, FoW, leveling, etc.

Phase 121.4 transcript capture is the *last* item — it depends on a working playtest, so it slots in after the playtest-quickstart verifier passes the rest of the system.

## Loop

1. On startup, build `tasks.md` from `SUMMARY.md`. One row per numbered finding (1–48) plus a row for "Phase 121.4 transcripts". Append to `log.md`: counts per tier.
2. Pick the next non-empty tier. Choose a batch of compatible tasks (no overlapping file edits). Mark them `claimed` and write `batches.md` row.
3. Launch implementers in parallel (one per task). Wait for completion notifications.
4. Launch reviewers in parallel (one per task). Wait.
5. For any `REVISIT`: re-launch the same implementer with a follow-up brief that points at the reviewer's reasons; re-launch the reviewer after. Loop until `PASS`.
6. Mark all batch tasks `done` in `tasks.md`. Commit the batch with a conventional-commit message that lists task IDs.
7. After every Critical and High batch closes, launch the integration verifier. If it fails, file new tasks for the failures (`integ-NN`) and append to the current tier.
8. When all tiers report `done` and the integration verifier is green, launch the playtest-quickstart verifier.
9. If the verifier reports any showstopper, file new tasks (`playtest-NN`), loop back to step 2 at the appropriate tier, and re-run the verifier afterward.
10. Stop only when `playtest_ready.md` contains a `Verdict: READY` from the verifier and a transcript proving every quickstart step succeeded.

## Constraints

- **TDD strictly**. Red test first, then minimal implementation. Never paste a fix without a failing test.
- **No scope creep**. Match each fix to the cited finding. If the worker spots a related defect, it files a new task in `tasks.md` rather than expanding scope.
- **No new abstractions for hypothetical futures**. Three similar lines beat a premature helper.
- **No new doc files** unless the finding explicitly calls for one. Edit existing docs.
- **No destructive ops**. If a fix needs a migration rollback or a force-push, ask the user first via a `BLOCKED` row in `tasks.md` and stop that task.
- **Do not skip hooks** (`--no-verify`, `--no-gpg-sign`). Fix the underlying issue.
- **Do not bypass coverage**. If a fix drops coverage below the per-package floor, the reviewer must `REVISIT` it.
- **Trust but verify subagent reports**. After every PASS, the orchestrator independently `git diff`s the touched files before marking `done`.
- **Honor the project's existing memory** at `/home/ab/.claude/projects/-home-ab-projects-DnDnD/memory/` — particularly the deferred-character-card-fields note.
- **Use the `caveman` mode the session already configured** for orchestrator status updates (drop articles, fragments OK, code/commits/security normal). User can disable.

## Stop condition

All of the following must hold simultaneously:

- Every row in `tasks.md` has `status: done`.
- `make cover-check`, `make test`, `make e2e`, `make playtest-replay` all green on the final commit.
- `/home/ab/projects/DnDnD/.fix-state/playtest_ready.md` ends with `Verdict: READY` and a transcript covering every step in `docs/playtest-quickstart.md`.
- A final `SUMMARY.md` in `.fix-state/` lists every task ID, its commit SHA, the spec section it satisfied, and the test that proves it.

Do not declare done until all four hold. Resume from disk if interrupted.
