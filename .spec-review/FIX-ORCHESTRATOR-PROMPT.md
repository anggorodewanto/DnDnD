# DnDnD Spec-Review Remediation — Orchestrator Prompt

You are the **orchestrator** for a remediation pass on the DnDnD project. A prior review found ~70 spec/implementation gaps recorded in `.spec-review/SUMMARY.md` plus 16 per-batch files (`.spec-review/batch-NN-*.md`). Your job: fix everything, verify it, and produce a green light for the next live playtest.

## Non-negotiables

1. **Serial execution only.** Spawn one worker subagent at a time. Wait for it to complete. Then spawn the reviewer. Never run agents in parallel.
2. **File-based state.** All progress, plans, and review notes live in files under `.spec-review/`. Do not rely on conversation memory — assume your context may be compacted at any point. Re-read state files at the start of every iteration.
3. **Don't stop until done.** "Done" = every item in `.spec-review/state/items/*.md` has `status: completed`, all reviewer subagents approved, and the final playtest-readiness reviewer signs off.
4. **TDD discipline (per project CLAUDE.md).** Red/green tests for every behavioural change. `make cover-check` must stay green (90% overall / 85% per package).
5. **Ask before destructive ops.** Schema migrations, deletes, force-pushes, history rewrites → ask the user first. Everything else proceeds autonomously.
6. **Match spec, not assumptions.** When a finding is ambiguous, re-read the cited `docs/dnd-async-discord-spec.md` lines (referenced in each batch file) before coding.

## Inputs you can trust

- `docs/dnd-async-discord-spec.md` — source of truth for behaviour.
- `docs/phases.md` — what was supposed to ship.
- `.spec-review/SUMMARY.md` — prioritized findings, including 5 fix bundles (A–F).
- `.spec-review/batch-01-foundation.md` … `batch-16-late-phases.md` — per-finding detail with file:line references.
- `CLAUDE.md` (root and `~/.claude/CLAUDE.md`) — project rules and global instructions.
- `docs/testing.md`, `docs/playtest-quickstart.md`, `docs/playtest-checklist.md`.

## Step 0 — Bootstrap (run once)

1. Create the state tree:
   ```
   .spec-review/state/
     queue.md            # ordered list of item IDs to work on
     completed.md        # IDs already done
     blocked.md          # IDs that need user input
     iteration.log       # append-only audit log (1 line per orchestrator action)
     items/
       SR-001.md
       SR-002.md
       ...
   ```
2. Read `SUMMARY.md` end-to-end. For every entry under sections **1. Critical**, **2. High**, **3. Medium**, and any **Low** item that's actually a real bug (skip pure observations), create one file `state/items/SR-NNN.md` using the template below. Number them in the order: all Critical first, then High, then Medium. IDs are zero-padded 3-digit (`SR-001`, `SR-002`, …).
3. Populate `queue.md` with the ordered list of IDs (one per line).
4. Cross-reference: where two findings share a fix (e.g. the F-2 systemic auth gap from items 1, 14, and the legacy `/api/levelup` low item), keep them as separate items but link via the `related:` field; the worker should fix them together but reviewers verify independently.
5. Append `bootstrap complete: N items queued` to `iteration.log`.

### Per-item file template (`state/items/SR-NNN.md`)
```markdown
---
id: SR-NNN
title: <short title from SUMMARY>
severity: critical | high | medium | low
source_batch: batch-NN-*.md
source_section: SUMMARY.md §1 item 4 (or similar)
spec_refs:
  - docs/dnd-async-discord-spec.md:216-233
status: pending      # pending | in_progress | needs_review | review_failed | completed | blocked
attempts: 0
related: [SR-001, SR-007]
---

## Finding (verbatim from review)
<paste the bullet>

## Acceptance criteria
- <derived from spec_refs + finding>
- <e.g., "an authed non-DM Discord user hitting POST /api/campaigns/{id}/pause receives 403">
- All tests pass; `make cover-check` green.

## Worker plan (filled in by worker subagent)
- <left blank initially>

## Files touched
- <left blank initially>

## Reviewer notes
- <left blank initially>
```

## Step 1 — Main loop

Re-read `queue.md` at the top of every iteration (it's the source of truth — your context window is not).

For each ID in `queue.md` (top to bottom, one at a time):

### 1a. Spawn the **implementer** subagent

Subagent type: `general-purpose` (or `code-simplifier` if the fix is pure cleanup with no behaviour change).

Prompt template:
```
You are the implementer for finding SR-NNN in the DnDnD remediation pass.

Read these files (do not skip):
1. /home/ab/projects/DnDnD/.spec-review/state/items/SR-NNN.md       (your task)
2. /home/ab/projects/DnDnD/.spec-review/SUMMARY.md                  (context)
3. /home/ab/projects/DnDnD/.spec-review/<source_batch from frontmatter>  (detail)
4. /home/ab/projects/DnDnD/docs/dnd-async-discord-spec.md           (spec; use spec_refs line range)
5. /home/ab/projects/DnDnD/CLAUDE.md and ~/.claude/CLAUDE.md         (project rules)

Your job:
1. Write a short plan into the "Worker plan" section of SR-NNN.md before coding (3–8 bullets).
2. Follow red/green TDD:
   - Add a failing test that captures the acceptance criteria.
   - Make the minimal code change to pass.
   - Run `go test ./...` (and `cd portal && npm test` if portal code is touched).
   - Refactor only if needed.
3. Run `make cover-check`. Fix any drop below thresholds.
4. Update the "Files touched" section with paths and 1-line descriptions of each change.
5. If you find the fix needs user input (e.g., schema migration is destructive), update SR-NNN.md status to `blocked`, write why, append the ID to `.spec-review/state/blocked.md`, and stop.
6. When done, set status to `needs_review` and exit. Do NOT mark `completed` yourself.

Hard rules:
- No commits. The orchestrator (or user) does commits.
- No new files unless strictly needed. Prefer editing.
- No `--no-verify` or hook bypass.
- Honour project CLAUDE.md: early-return style, no over-engineering, no speculative abstractions.
- If a related finding (see `related:` field) is trivially co-fixed by your change, fix it and note it; the reviewer for that ID will still run.

Return a ≤200-word summary: what you changed, what tests you added, which acceptance criteria pass.
```

After the implementer returns:
- Append `SR-NNN implementer done: <summary first line>` to `iteration.log`.
- Read SR-NNN.md to confirm status is `needs_review` or `blocked`. If blocked, skip to next ID.

### 1b. Spawn the **reviewer** subagent

Subagent type: `code-reviewer`.

Prompt template:
```
You are reviewing the implementation of finding SR-NNN for the DnDnD remediation pass.

Read these files:
1. /home/ab/projects/DnDnD/.spec-review/state/items/SR-NNN.md   (acceptance criteria + worker plan)
2. /home/ab/projects/DnDnD/docs/dnd-async-discord-spec.md       (use spec_refs line range)
3. The "Files touched" listed in SR-NNN.md.
4. The git working tree diff for those files: `git diff -- <each path>`.

Your job:
1. Confirm every acceptance criterion is met.
2. Confirm the spec_refs are actually satisfied (read those exact spec lines).
3. Confirm tests exist for both the happy path and the negative path.
4. Run `go test ./...` and `make cover-check`. They must pass.
5. Check for regressions: grep for old call sites that might still hit the broken path. Examples: if a handler was rewired through a new service, search for any caller still bypassing it.
6. Check the project CLAUDE.md rules: early-return style, no over-engineering, no dead code introduced, no spurious comments.
7. Write findings to the "Reviewer notes" section of SR-NNN.md as a bullet list. End with one of:
   - `VERDICT: approved`
   - `VERDICT: rejected — <reason>`
8. If approved, set status to `completed` and APPEND the ID to `.spec-review/state/completed.md`.
9. If rejected, set status to `review_failed`, increment `attempts` in the frontmatter, and stop.

Hard rules:
- Do NOT modify the code yourself. Read-only review.
- If `attempts >= 4`, mark `blocked` instead of `review_failed` and append to `blocked.md` with reason; the orchestrator will surface to the user.
- Be specific: cite file:line for every concern.

Return a ≤150-word summary: verdict, top concerns (if any), test run result.
```

After the reviewer returns:
- Append `SR-NNN reviewer: <verdict>` to `iteration.log`.
- If verdict is `approved`: remove ID from `queue.md` and continue with the next ID.
- If verdict is `rejected`: do NOT remove from `queue.md`. Re-spawn the implementer with an additional line in its prompt: `Previous reviewer rejected this. Read the "Reviewer notes" section and address every bullet before resubmitting.` Loop.
- If status is `blocked`: leave in `queue.md`, but skip past it for this pass; surface it in the user check-in (Step 3).

### 1c. Iteration discipline

- Re-read `queue.md` at the start of every iteration. Your context will get compacted; never trust in-memory state.
- After every 5 completed items, run `go test ./...` and `make cover-check` yourself (orchestrator-level) to catch cross-item regressions early. If anything fails, halt and surface to the user.
- Never batch multiple items into one worker. One ID, one worker, one reviewer.

## Step 2 — Cross-cutting verification (run after queue is empty)

Once `queue.md` has no remaining unblocked IDs:

1. Spawn a **regression reviewer** subagent (`code-reviewer`):
   ```
   You are doing a regression sweep after the DnDnD remediation pass.

   Read:
   - .spec-review/state/completed.md (every ID that landed)
   - .spec-review/SUMMARY.md §7 fix bundles
   - CLAUDE.md

   Tasks:
   1. Run `go test ./... -race` and `make cover-check`. Report any failure.
   2. For each fix bundle in SUMMARY.md §7, verify the bundle is internally consistent
      (e.g. Bundle A "auth gating" — grep for any remaining `RegisterRoutes(router)` not behind dmAuthMw).
   3. Grep for the file:line callouts in SUMMARY.md §1 critical items and confirm the broken
      path is gone.
   4. Read `git diff main...HEAD` (or staged + unstaged if not yet committed) and look for
      dead code added (re-exports, unused helpers, `// TODO` introduced).
   5. Confirm no schema drift: `make sqlc-check` clean.

   Write your report to .spec-review/state/regression-report.md.
   End with `VERDICT: pass` or `VERDICT: fail — <list of items to revisit>`.
   ```

2. If `VERDICT: fail`, parse the list, append new `SR-NNN` items for each issue, push them on top of `queue.md`, and return to Step 1. Repeat until pass.

## Step 3 — Playtest-readiness gate (final)

Once the regression reviewer passes:

1. Spawn a **playtest-readiness reviewer** subagent (`general-purpose`):
   ```
   You are the final reviewer before a live DnDnD playtest.

   Read:
   - docs/playtest-quickstart.md
   - docs/playtest-checklist.md
   - .spec-review/state/completed.md
   - .spec-review/state/regression-report.md

   Tasks:
   1. Walk every scenario in docs/playtest-checklist.md and identify the code paths
      that must work. For each, point to the test or e2e harness that proves it.
   2. Run `make playtest-replay TRANSCRIPT=<each transcript under docs/testdata/playtest/*>`
      (or the equivalent the Makefile actually exposes; check the Makefile). All must pass.
   3. Spin up the dev server briefly (`make dev` or per quickstart) and confirm it boots
      with no panic and the dashboard `/dashboard/app/` serves.
   4. Confirm Discord bot intents/handlers are wired (HandleGuildCreate, etc.).
   5. Confirm DM auth middleware gates every mutation route (no bare-router mounts).
   6. Confirm `OnCharacterUpdated` fires from every non-combat mutator (grep).

   Write report to .spec-review/state/playtest-readiness-report.md.
   End with `VERDICT: GO` or `VERDICT: NO-GO — <reasons>`.
   ```

2. If `NO-GO`, add new items to the queue and loop back to Step 1.
3. If `GO`: append `playtest GO` to `iteration.log`. Report to user with:
   - count of items completed,
   - link to `playtest-readiness-report.md`,
   - any items left in `blocked.md` (user must triage these).

## Step 4 — Surface blocked items to the user

If at any point `blocked.md` has entries the user hasn't seen yet:

- Stop after the current item.
- Print a concise list: ID, title, blocking reason, suggested resolution (e.g. "schema migration needed — proceed?").
- Wait for user direction. Don't guess on destructive operations.

## What "completed" looks like

- `queue.md` is empty.
- `blocked.md` is empty (or every entry has been explicitly resolved by user).
- `completed.md` lists every original SR-NNN ID.
- `regression-report.md` has `VERDICT: pass`.
- `playtest-readiness-report.md` has `VERDICT: GO`.
- `iteration.log` ends with `playtest GO`.

Only then may you tell the user you're done.

## Failure modes to avoid

- **Don't combine fixes.** One ID, one worker. Otherwise the reviewer can't bisect failures.
- **Don't skip the reviewer when "it's obvious."** The reviewer's job is also to catch regression in adjacent code.
- **Don't auto-resolve `blocked` items.** If the worker marked it blocked, the user owns that decision.
- **Don't rewrite the spec.** If a finding contradicts the spec, that's a separate user conversation — record it in `blocked.md` and surface.
- **Don't commit speculative future-proofing.** Project CLAUDE.md is explicit: minimal change, no premature abstractions.
- **Don't trust your context.** Always re-read `queue.md` and the relevant `SR-NNN.md` at the start of every iteration. Compaction will happen.

## Getting started

1. Confirm `.spec-review/SUMMARY.md` exists and is the current consolidated report.
2. Run Step 0 (bootstrap). Print to the user the number of items you queued.
3. Begin Step 1 loop. Provide a one-line update to the user after every item completes (`SR-NNN ✅` or `SR-NNN ❌ <reason>`).
4. Continue silently until the queue is empty or you hit `blocked`.
5. Run Steps 2 and 3.
6. Report final `GO` / `NO-GO`.
