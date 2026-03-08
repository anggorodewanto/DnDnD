# Open Question Resolution Prompt

You are resolving open questions from `docs/open-questions.md` against the spec at `docs/dnd-async-discord-spec.md`.

## Input

The user will provide one of:
- **A question number** (e.g., `#1`, `#42`) — resolve that specific question
- **`next`** — pick the highest-priority unresolved question yourself, prioritizing: spec contradictions > missing core mechanics > player UX gaps > nice-to-haves

## Workflow

### 1. Load context
- Read `docs/open-questions.md` to find the target question (by number, or scan for unresolved ones if `next`)
- Read the relevant section(s) of `docs/dnd-async-discord-spec.md`

### 2. Discuss with the user
Present the question clearly, then:
- Explain **why it matters** from a player's perspective (1-2 sentences)
- Offer **2-3 concrete options** with tradeoffs, labeling them A/B/C
- State which option you'd recommend and why
- Ask the user to pick an option or propose their own

Do NOT proceed until the user has decided.

### 3. Apply the decision
Once the user decides:
- Edit `docs/dnd-async-discord-spec.md` to incorporate the decision in the appropriate section(s)
- Keep the spec's existing style and structure — don't reorganize or reformat unrelated sections
- Mark the question as resolved in `docs/open-questions.md` by changing `- [ ]` to `- [x]` and replacing the question body with: `N. **Title.** — Resolved: brief summary of decision`

### 4. Commit and push
- Stage both changed files
- Commit with message: `Resolve open question #N: <short description>`
- Push to current branch

## Rules
- Only modify spec sections relevant to the question — do not refactor or rewrite other parts
- If a question turns out to already be addressed in the spec, change `- [ ]` to `- [x]` with a note and skip the edit
- If a question requires splitting into sub-questions, tell the user and add numbered sub-items (e.g., 41a, 41b) to `docs/open-questions.md`
- If the user wants to defer a question, change the line to `- [-] N. **Title.** — Deferred: reason` and move on
- One question per invocation — keep it focused

## After completion

After committing (or marking as resolved/deferred), count the remaining unresolved questions in `docs/open-questions.md` (lines starting with `- [ ]`) and tell the user: **"X unresolved open questions remaining."**
