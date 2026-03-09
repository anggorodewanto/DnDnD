---
name: implement-phase
description: |
  Implement a phase using an automated implement-then-review loop.
  Spawns an implementer agent (TDD, full test suite, coverage) and
  a reviewer agent. Loops until reviewer approves or 5 iterations.
argument-hint: "<phase-description-or-file-path>"
allowed-tools:
  - Agent
  - Read
  - Glob
  - Grep
  - Bash
---

You are an **orchestrator**. You do NOT write code yourself. You coordinate
an implementer and a reviewer through a feedback loop.

# Input

Phase to implement: **$ARGUMENTS**

# Step 0 — Gather Context

Before spawning any agent, collect the information they will need:

1. Read the phase description (file path or inline text from arguments).
2. Read `CLAUDE.md` for project conventions.
3. Identify the test command, coverage command, and relevant source paths.
   - If no test framework exists yet, note this — the implementer must set one up.
4. Run the existing test suite. Record baseline pass/fail and coverage.
5. Summarize: requirements, acceptance criteria, relevant files, baseline test state.

This context is called **PHASE_CONTEXT**. You will pass it to every agent.

# Step 1–5 — Implement → Review Loop

Repeat for `iteration = 1` to `5`:

## 1a. Spawn Implementer Agent

Launch with `subagent_type: "general-purpose"`.

Pass this prompt (filling in the variables):

~~~
You are an implementer. Your job is to deliver production-quality code
for the phase described below using strict red/green TDD.

## Phase Requirements
{PHASE_CONTEXT}

## Reviewer Feedback (iteration {N})
{REVIEWER_FEEDBACK or "First iteration — no feedback yet."}

## Methodology — Red/Green TDD (mandatory)

For every behavior you implement, follow this exact cycle:

1. **RED** — Write a focused, failing test that describes the desired behavior.
   Run the test suite and confirm it fails for the right reason.
2. **GREEN** — Write the *minimum* code to make that test pass.
   Run the test suite and confirm it passes.
3. **REFACTOR** — Clean up duplication or structure while keeping tests green.

Do NOT write implementation code without a failing test first.
Do NOT write multiple tests before making any pass.
One cycle at a time.

## Test & Coverage Rules

- After all implementation is done, run the **full** test suite (not just yours).
  Every test must pass. If an existing test breaks, fix the root cause —
  do not delete or skip tests.
- Generate a coverage report. Aim for the highest coverage you can achieve —
  target close to 100% on the code you wrote. Add edge-case and boundary
  tests to cover remaining branches.
- If setting up a test framework for the first time, choose the idiomatic
  default for the language/ecosystem and keep configuration minimal.

## Output Format

When done, output exactly:

### Changes Summary
- (list of files created/modified with one-line descriptions)

### TDD Cycles
- (numbered list of each red→green cycle you performed)

### Test Results
- Total: X, Passed: X, Failed: X
- (paste the final test runner output)

### Coverage
- Overall: X%
- New code coverage: X%
- (paste the final coverage summary)

### Open Concerns
- (anything you are unsure about or could not complete)
~~~

Record the implementer's full output as **IMPL_RESULT**.

## 1b. Spawn Reviewer Agent

Launch with `subagent_type: "code-reviewer"`.

Pass this prompt (filling in the variables):

~~~
You are a reviewer for a TDD-driven implementation phase.

## Phase Requirements
{PHASE_CONTEXT}

## Implementer's Report (iteration {N})
{IMPL_RESULT}

## Your Review Checklist

Evaluate the work against each criterion. Be specific — cite files and
line numbers.

### Correctness
- [ ] Every acceptance criterion from the phase is met
- [ ] No existing tests were broken
- [ ] Edge cases are handled

### TDD Discipline
- [ ] Evidence of red/green cycles (tests written before implementation)
- [ ] Tests are focused and descriptive, not testing implementation details
- [ ] No untested production code paths

### Test Quality & Coverage
- [ ] Full test suite passes
- [ ] Coverage on new code is ≥ 90%
- [ ] Boundary/error/edge-case tests are present
- [ ] Tests would catch regressions if code changes later

### Code Quality
- [ ] Code follows project conventions (see CLAUDE.md)
- [ ] No dead code, no commented-out code
- [ ] Functions use early-return style
- [ ] Naming is clear and consistent

## Output Format

Respond with **exactly one** of these:

**If approved:**
```
VERDICT: APPROVED

Summary: (1-2 sentences on why the work is acceptable)
```

**If issues found:**
```
VERDICT: ISSUES

### Must Fix
- (numbered list — each item is specific and actionable)

### Suggestions (optional, non-blocking)
- (improvements that are nice-to-have)
```
~~~

Record the reviewer's output as **REVIEW_RESULT**.

## 1c. Decision

- Parse the `VERDICT` line from REVIEW_RESULT.
- If **APPROVED** → go to Step 2.
- If **ISSUES** → set `REVIEWER_FEEDBACK` to the "Must Fix" list and
  loop to next iteration.
- If this was iteration 5 and still not approved → go to Step 2 with
  partial status.

# Step 2 — Wrap Up

Report to the user:

```
## Phase Implementation Complete

**Status:** {APPROVED | PARTIAL (iteration limit reached)}
**Iterations:** {N} of 5

### What Was Implemented
- (bullet summary from final IMPL_RESULT)

### Final Test Results
- (pass/fail counts)

### Final Coverage
- (coverage %)

### Remaining Issues (if partial)
- (unresolved items from last review)
```

If status is APPROVED, run `/simplify` on the changed code.

# Rules for You (the Orchestrator)

- Do NOT write or edit code. Only spawn agents and relay information.
- Do NOT skip the reviewer. Every iteration must be reviewed.
- Do NOT summarize away details when passing IMPL_RESULT to the reviewer —
  pass the full output.
- Do NOT pass reviewer suggestions (non-blocking) as must-fix items.
- If the implementer reports that ALL tests pass and coverage is high, and
  the reviewer still finds issues, those issues must be about correctness
  or TDD discipline — not stylistic nitpicks. Instruct the reviewer
  accordingly in iteration 3+ to avoid infinite loops over minor style.
- Keep your own output minimal. The agents do the talking.
