# Resolve Open Question

Resolve an open question from `docs/open-questions.md` through discussion with the user, then update the spec.

## Arguments

- `$ARGUMENTS` — either a question number (e.g., `12`) or the word `next`

## Workflow

1. **Read** `docs/open-questions.md` and parse all questions with their checkbox status.

2. **Select the question:**
   - If `$ARGUMENTS` is a number, select that question (error if already resolved).
   - If `$ARGUMENTS` is `next`, select the lowest-numbered unresolved question.

3. **Present the question** to the user. Include:
   - The question number and title
   - The full question text
   - 2-3 concrete options or a suggested default, informed by the spec context. Read relevant sections of `docs/dnd-async-discord-spec.md` before proposing options.

4. **Discuss** with the user until they choose a resolution. Ask clarifying follow-ups if their answer is ambiguous. Keep it conversational — one question at a time.

5. **Once resolved:**
   a. Update `docs/dnd-async-discord-spec.md` with the decision (add to the appropriate section, matching existing style and detail level).
   b. Mark the question as `[x]` in `docs/open-questions.md`.
   c. Stage both files, commit with message: `Resolve open question #N: <short summary>`
   d. Push to remote.

6. **Show the count** of remaining unresolved questions (e.g., "37/48 questions remaining").
