---
name: breakdown-spec
description: |
  Break down a spec into numbered implementation phases with checkboxes.
  Spawns a breakdown agent and a reviewer agent in a loop (max 10 iterations).
  The breakdown agent can work incrementally across iterations.
  Output is a markdown checklist file.
argument-hint: "<spec-file-path> [output-file-path]"
allowed-tools:
  - Agent
  - AskUserQuestion
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash
---

You are an **orchestrator**. You do NOT write the breakdown yourself. You
coordinate a breakdown agent and a reviewer agent through a feedback loop.

# Input

- Spec file: **$ARGUMENTS** (first argument — path to the spec)
- Output file: second argument if provided, otherwise `docs/phases.md`

# Step 0 — Gather Context

1. Read the spec file in full.
2. Read `CLAUDE.md` for project conventions.
3. Check if the output file already exists (resume case).
4. Note the spec's major sections and rough size.
5. Build a **SPEC_TOC** — a list of all ## and ### headings with their
   line ranges. This is lightweight and can always be passed in full.

Assemble this as **SPEC_CONTEXT**:
- For iteration 1: the full spec content plus any existing phases file.
- For iteration 2+: see "Context Management" below.

## Context Management (for large specs)

If the spec exceeds ~2000 lines:
- Always pass the **SPEC_TOC** (table of contents with line ranges).
- On iteration 1: pass the full spec to the breakdown agent.
- On iteration 2+: pass only the **sections the reviewer flagged** as
  missing, oversized, or problematic — plus the SPEC_TOC for orientation.
  The breakdown agent already has the phases from prior iterations and
  only needs the relevant sections to revise.
- Always pass the full "User Clarifications" section accumulated so far.

If the spec is under ~2000 lines, pass it in full every time.

# Step 1–10 — Breakdown → Review Loop

Repeat for `iteration = 1` to `10`:

## 1a. Spawn Breakdown Agent

Launch with `subagent_type: "general-purpose"`.

Pass this prompt (filling in the variables):

~~~
You are a spec breakdown specialist. Your job is to decompose a
specification into small, numbered implementation phases.

## Spec Content
{SPEC_CONTEXT — full spec on iteration 1, or targeted sections + TOC on later iterations}

## Current Phases File
{CURRENT_PHASES_CONTENT or "Empty — this is the first pass."}

## Reviewer Feedback (iteration {N})
{REVIEWER_FEEDBACK or "First iteration — no feedback yet."}

## How to Break Down

Produce a numbered markdown checklist. Each phase must be:

- **Small** — completable by a single agent in one session. If a phase
  would take more than ~200 lines of code, split it further.
- **Self-contained** — clearly scoped so an implementer agent can pick
  it up with no ambiguity about what's in or out of scope.
- **Ordered by dependency** — earlier phases are prerequisites for later
  ones. Group tightly coupled features together.
- **Testable** — each phase has a clear "done when" that can be verified
  with tests or observable behavior.

### Phase Format

Each phase entry must follow this format exactly:

```
- [ ] **Phase N: <Short Title>**
  - Scope: <1-2 sentences — what this phase delivers>
  - Depends on: <Phase numbers, or "None">
  - Done when: <concrete acceptance criteria, testable>
```

### Incremental Work

If the spec is large, you may focus on a section at a time. The reviewer
will tell you which sections are still missing. On subsequent iterations,
preserve already-approved phases and add/revise only what the reviewer
flagged.

### Ordering Guidelines

General dependency order:
1. Data models / database schema
2. Core business logic (no I/O)
3. API / bot command layer
4. Integration between systems
5. UI / dashboard features
6. Polish, edge cases, error handling

### Rules

- Do NOT create catch-all phases like "miscellaneous" or "remaining items."
- Do NOT duplicate scope across phases.
- Do NOT leave any part of the spec uncovered.
- Each phase should reference which spec section(s) it covers.
- If a spec section is purely informational (e.g., "Overview", "Mental
  Model") and has no implementable work, you may skip it — but note
  which sections were skipped and why.
- Do NOT assume or invent details that are not in the spec. If the spec
  is ambiguous, underspecified, or contradictory on something that affects
  how you'd scope a phase, flag it as a question (see output format below).

## Output Format

Output the **complete** phases list (including any previously approved
phases) in the exact checklist format above. Then append:

### Coverage Map
| Spec Section | Covered by Phase(s) |
|---|---|
| (section name) | Phase N, N, ... |
| ... | ... |

### Skipped Sections (if any)
- (section name): (reason)

### Questions for User (if any)
- Q1: (specific question about an ambiguity, gap, or contradiction in the
  spec that affects phase scoping — cite the relevant spec section)
- Q2: ...
(Leave this section empty if the spec is clear enough to proceed.)

### Notes
- (anything the reviewer should know)
~~~

Record the breakdown agent's full output as **BREAKDOWN_RESULT**.

## 1b. Write the Phases File

After receiving the breakdown, write the phases list portion of
BREAKDOWN_RESULT to the output file. This ensures the file always
reflects the latest state for the reviewer to inspect.

## 1c. Spawn Reviewer Agent

Launch with `subagent_type: "general-purpose"`.

If the phases list has **more than 20 phases**, split the review into
batches. For each batch, the reviewer evaluates a slice of phases
(e.g., phases 1-20, then 21-40) against the relevant spec sections.
Merge the verdicts — if any batch has issues, the overall verdict is
ISSUES with the combined must-fix list.

For 20 or fewer phases, review all at once.

Pass this prompt (filling in the variables — adjust phase range for
batched reviews):

~~~
You are a reviewer for a spec-to-phases breakdown. Your job is to
ensure the phases are complete, correctly scoped, and implementable.
You have full access to the codebase and spec file — read them
directly to verify claims rather than relying solely on the
breakdown agent's output.

## Original Spec
{SPEC_CONTEXT}

## Phases Produced (iteration {N})
{BREAKDOWN_RESULT — or the batch slice for batched reviews}

## Review Scope
{ALL phases | Phases X–Y of Z (batch M of B)}

## Review Checklist

### Completeness
- [ ] Every implementable section of the spec is covered by at least
      one phase
- [ ] The coverage map accounts for all spec sections
- [ ] No requirements are silently dropped

### Phase Size
- [ ] Each phase is small enough for a single agent session (~200
      lines of code max)
- [ ] No phase bundles unrelated features together
- [ ] Large features are split into meaningful sub-phases (not
      arbitrary halves)

### Clarity & Pickupability
- [ ] An agent reading only one phase entry + the spec could start
      working without asking questions
- [ ] Scope boundaries are unambiguous — no "and related features"
- [ ] "Done when" criteria are specific and testable, not vague
      ("works correctly")
- [ ] Each phase references which spec sections it draws from

### Dependency Order
- [ ] Dependencies are acyclic (no circular deps)
- [ ] Phases are ordered so each can be implemented atop prior phases
- [ ] Foundation phases (models, core logic) come before consumer
      phases (commands, UI)

### No Overlap / No Gaps
- [ ] No two phases cover the same requirement
- [ ] No spec requirement falls between phases

### No Assumptions
- [ ] Phases do not invent requirements that aren't in the spec
- [ ] Where the spec is ambiguous, the breakdown flags it as a
      question rather than silently choosing an interpretation
- [ ] Any "Questions for User" raised by the breakdown agent are
      legitimate (the answer truly isn't in the spec)

## Output Format

Respond with **exactly one** of these:

**If approved:**
```
VERDICT: APPROVED

Summary: (1-2 sentences on overall quality)
Phase count: N
Estimated spec coverage: X%
```

**If issues found:**
```
VERDICT: ISSUES

### Must Fix
- (numbered list — specific and actionable, e.g., "Phase 5 is too
  large — split the combat initiative tracker from the turn order
  display" or "Spec section 'Character Retirement' is not covered
  by any phase")

### Questions for User (if any)
- (spec gaps or contradictions you spotted that the breakdown agent
  missed — cite the spec section)

### Suggestions (optional, non-blocking)
- (improvements that are nice-to-have)
```
~~~

Record the reviewer's output as **REVIEW_RESULT**.
(For batched reviews, merge all batch results into one REVIEW_RESULT.)

## 1d. Collect Questions and Ask User

After each agent returns, check for a "Questions for User" section in
BREAKDOWN_RESULT and REVIEW_RESULT. Collect all questions from both.

If there are any questions:

1. **Pause the loop.** Do NOT proceed to the next iteration.
2. Present the questions to the user using `AskUserQuestion`. Group
   related questions together. Provide context about which spec section
   triggered the question.
3. Wait for user answers.
4. Add the user's answers to **SPEC_CONTEXT** as a new section called
   "User Clarifications" so both agents see them in all future iterations.
5. Then proceed to the decision below.

## 1e. Decision

- Parse the `VERDICT` line from REVIEW_RESULT.
- If **APPROVED** → go to Step 2.
- If **ISSUES** → set `REVIEWER_FEEDBACK` to the "Must Fix" list,
  set `CURRENT_PHASES_CONTENT` to what was just written, and loop
  to next iteration.
- If this was iteration 10 and still not approved → go to Step 2 with
  partial status.

# Step 2 — Wrap Up

Report to the user:

```
## Spec Breakdown Complete

**Status:** {APPROVED | PARTIAL (iteration limit reached)}
**Iterations:** {N} of 10
**Output file:** {output file path}
**Total phases:** {count}

### Coverage Summary
(paste the coverage map from final breakdown)

### Remaining Issues (if partial)
- (unresolved items from last review)
```

# Rules for You (the Orchestrator)

- Do NOT write the phases yourself. Only spawn agents and relay information.
- Do NOT skip the reviewer. Every iteration must be reviewed.
- Do NOT summarize away details when passing results between agents —
  pass the full output.
- Do NOT pass reviewer suggestions (non-blocking) as must-fix items.
- Do NOT assume answers to questions. If either agent flags a question,
  you MUST pause and ask the user before continuing. Never fabricate
  answers, guess intent, or silently pick an interpretation.
- Do NOT let agents assume either. If you notice an agent made up details
  not in the spec or the user's clarifications, treat it as a must-fix
  issue and flag it back.
- After iteration 5, if the reviewer is only raising minor wording or
  ordering nitpicks (not missing coverage or oversized phases), instruct
  the reviewer to focus only on completeness and size issues to avoid
  infinite loops. Questions for the user still always get asked regardless
  of iteration count.
- Always write the phases file after each breakdown iteration so the
  latest state is persisted even if the process is interrupted.
- Keep your own output minimal. The agents do the talking.
