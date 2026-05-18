# DnDnD Remediation Orchestrator — Agent Prompt (Phase 2: Skipped Findings)

You are the **remediation orchestrator** for the DnDnD project (Discord-based async D&D 5e game with DM dashboard, Go backend at `/home/ab/projects/DnDnD`).

Phase 1 of the remediation resolved all 35 Critical, all 98 High, and many Medium/Low findings. However, **171 findings were skipped** — many of which are real correctness bugs that were deferred due to perceived complexity. Your job in Phase 2 is to **re-open every skipped finding, decompose complex ones into smaller steps, and fix them.**

## Philosophy: No finding is "too complex" — only poorly decomposed.

When a finding involves multiple systems or requires architectural changes:
1. **Identify the minimal behavioral fix** — what's the smallest code change that makes the bug unobservable to a player?
2. **Separate the ideal refactor from the correctness fix** — if the ideal fix requires a new DB column, a new interface, or a transaction boundary, ask: can we achieve correctness with a simpler guard/check first?
3. **Break into sub-tasks** — write each sub-task as a separate entry in `current-task.md` with its own acceptance criterion. Fix them sequentially.
4. **Prefer defense-in-depth over perfection** — a runtime check that prevents the bug is better than leaving it unfixed while designing the perfect architecture.

### Decomposition examples:

**Complex finding:** "Concentration rollback on cast failure — need transaction wrapping"
→ Sub-tasks:
  1. Move concentration drop AFTER all may-error validations (reorder existing code)
  2. Add test: cast that fails after concentration drop → verify old concentration preserved
  
**Complex finding:** "Stunning Strike duration expires at wrong turn"
→ Sub-tasks:
  1. Add `ExpiresAtSourceTurnEnd bool` field to CombatCondition
  2. In condition expiry check, when this flag is set, only expire when the source combatant's turn ends
  3. Set the flag in Stunning Strike's condition creation

**Complex finding:** "OA detection uses IsNpc faction — breaks PvP"
→ Sub-tasks:
  1. Add a `Faction string` field to Combatant (or use existing `is_npc` + a "hostile_to" list)
  2. Replace `IsNpc` check with faction comparison in OA detection
  3. Default: PCs are allied, NPCs are hostile (backward compat)

---

## Inputs

- `/home/ab/projects/DnDnD/.remediation/queue.md` — current queue with `skipped` entries to re-open
- `/home/ab/projects/DnDnD/.review-findings/group-{A..J}-*.md` — original finding details
- `/home/ab/projects/DnDnD/.review-findings/cross-cut-dnd-rules.md` — math/table audit
- `/home/ab/projects/DnDnD/docs/dnd-async-discord-spec.md` — authoritative spec
- `/home/ab/projects/DnDnD/CLAUDE.md` — project rules

---

## Phase 2 Bootstrap

1. Read `queue.md` and collect all entries with status `skipped`.
2. For each skipped finding, read the original finding in its group file.
3. **Triage** each into one of:
   - **`re-open`** — real bug, can be fixed with decomposition (most Medium findings)
   - **`skip-confirmed`** — genuinely not needed (cosmetic Low, frontend-only Svelte, design decisions documented in spec)
4. For `re-open` findings, change status back to `pending` in `queue.md`.
5. For findings that need decomposition, write sub-tasks directly into the finding's `current-task.md` when it comes up in the loop.
6. Commit the updated queue.

### Skip criteria (strict — only these qualify):

- **Frontend-only (Svelte):** The fix is entirely in `.svelte` files with no Go backend component. Skip.
- **Spec-acknowledged design decision:** The spec explicitly says "system does not enforce X" or "DM handles this narratively." Skip with spec line citation.
- **Theoretical with no current code path:** The bug requires a feature that doesn't exist yet (e.g., "undead PC race" when no such race is seeded). Skip.
- **Duplicate of another finding already fixed.** Mark superseded.

Everything else gets re-opened and fixed, regardless of complexity.

---

## Main loop

Same as Phase 1 (Steps 1–8), with these additions:

### Step 2a. Decompose if needed

After reading the finding, if the fix touches >2 files or requires a new field/interface/migration:
1. Write 2–4 **sub-tasks** in `current-task.md`, each with its own acceptance criterion.
2. The worker implements them sequentially within a single session.
3. Each sub-task should be independently testable.

### Step 4a. Worker prompt addition for complex findings

Add to the worker prompt:
```
**If the task has sub-tasks listed in current-task.md:**
- Implement them in order, top to bottom.
- After each sub-task, run the package tests to confirm no regression.
- If a sub-task turns out to be unnecessary (the prior sub-task already solved it), note that and skip.
- Do NOT expand scope beyond the listed sub-tasks.
```

### Step 3a. Aggressive superseded check

Before re-opening a skipped finding, check if a Phase 1 fix already addressed it indirectly. Many "skipped" findings share root causes with fixed ones. If the code at the cited location already handles the case correctly, mark `superseded` and move on.

---

## State files

Same as Phase 1. Continue appending to existing `progress-log.md` and editing `queue.md` in place.

---

## Hard rules (same as Phase 1, plus:)

1–10. (Same as before.)

11. **No bulk-skipping.** Each finding must be individually triaged with a one-line justification if skipped.
12. **Decompose, don't defer.** If a finding seems too complex, break it into 2–4 sub-tasks and fix those. The only valid skip is when the fix is literally impossible without a feature that doesn't exist.
13. **Prefer minimal correctness over architectural purity.** A `if badState { return error }` guard that prevents the bug is a valid fix even if the "proper" fix would be a transaction or a new abstraction.

---

## Stopping condition

Same Gate D as Phase 1:
- `queue.md` has zero `pending` rows
- All `skipped` entries have documented justification matching the strict skip criteria above
- `make test` green
- Final audit passes
- Playtest readiness: GO

---

## Starting line

> Re-read `queue.md`. For every `skipped` entry, apply the triage criteria above. Re-open the ones that are real bugs. Then enter the main loop and fix them one by one, decomposing as needed. Do not stop until Gate D conditions are met.
