# DM Character Review & Edit Diff

**Status:** Implemented (2026-06-27)
**Date:** 2026-06-27
**Owner:** dashboard / approval flow

## Problem

The DM approval page (`dashboard/svelte/src/CharacterApprovalQueue.svelte`) shows only the
character **name + status** and the Approve / Request-changes / Reject buttons. The DM has
no way to see *what* they are approving:

- For a **new** submission, the DM cannot review the character's stats, class, skills,
  equipment, or spells before approving.
- For an **edited** character, the DM cannot see **what changed** since they last approved
  it — there is no diff.

We want the DM to review new characters in full, and to see a real **before → after diff**
for edits.

## Goals

1. On the approval page, show the **full character** (scores, saves, skills, equipment,
   spells, features, appearance/backstory) for every pending submission.
2. For an **edited** character, show a **before → after diff** of the reviewable fields,
   highlighting exactly what the player changed since the last approved version.
3. Keep the existing approve / request-changes / reject actions unchanged.

## Non-goals

- Field-level approval (approve some changes, reject others). Approval stays all-or-nothing.
- Versioned history / audit trail beyond the single most-recent approved baseline.
- Changing the existing "edits apply to the live record immediately" behavior (see
  [Background](#background-key-constraint)). The diff is a review aid, not a staging gate.

## Background: key constraint

**No pre-edit state is stored today.** A player edit overwrites the `characters` row in
place via `BuilderStoreAdapter.UpdateCharacterRecord`
(`internal/portal/builder_store_adapter.go:335`) and flips the `player_characters` row back
to `pending` (`BuilderService.UpdateCharacter`, `internal/portal/builder_service.go:664`,
the `if !isDM && PlayerCharacterID != ""` block ~`:700`). There is **no** `previous_data`
column, history table, or version row. `character_drafts` is an opaque WIP builder blob, not
an approved baseline.

Therefore a real edit diff requires **capturing a baseline snapshot** at edit time. The good
news: `UpdateCharacterRecord` already loads the full pre-edit character into `existing`
(`builder_store_adapter.go:340`) *before* overwriting — that is the natural snapshot point.

## What already exists (reuse)

| Asset | Location | Reuse |
|---|---|---|
| Full read-only character sheet (HTML) | `GET /portal/character/{characterID}` → `CharacterSheetHandler.ServeCharacterSheet` (`internal/portal/character_sheet_handler.go:49`); auth `CanViewCharacter` already allows the campaign DM, incl. pending (`internal/portal/character_sheet.go:130-163`) | Optional "open full sheet" deep-link from each approval entry. Zero new render code. |
| Approval detail endpoint (JSON, **already exists, unused by UI**) | `GET /dashboard/api/approvals/{id}` → `ApprovalHandler.GetApproval` (`internal/dashboard/approval_handler.go:189`); store `GetApprovalDetail` (`internal/dashboard/approval_store.go:85`); shape `ApprovalDetail` (`internal/dashboard/approval.go:22-35`) | Extend to carry the full review projection + the before-snapshot. |
| Pure diff helper | `diffStates(before, after)` → `[]{field, before, after}` (`dashboard/svelte/src/lib/diff.js:11`) | Render the edit diff in the panel. |
| Approval queue component | `dashboard/svelte/src/CharacterApprovalQueue.svelte` (entries already carry `character_id` and the `player_characters` id) | Add an expandable review/diff panel per entry. |
| Reverse projection (character → builder submission) | `submissionFromCharacter` (`internal/portal/builder_store_adapter.go:457`); `CharacterSubmission` (`internal/portal/builder_service.go:39-63`) | Reference for which fields are user-meaningful. |

## Design

### Reviewable projection

Define a single normalized projection of a character that captures exactly the fields a DM
reviews, and build it the **same way** for both before and after so `diffStates` compares
like-for-like.

```
type ReviewCharacter struct {
    Name        string
    Race        string
    Subrace     string
    Background   string
    Classes      []character.ClassEntry   // class/subclass/level (multiclass)
    Level        int
    AbilityScores character.AbilityScores  // final post-racial scores
    HPMax        int
    AC           int
    SpeedFt      int
    Skills       []string
    Expertise    []string
    Saves        []string
    Languages    []string
    Equipment    []string                 // names; from inventory/equipped
    Spells       []string
    Features     []string                 // feature names
    Appearance   string
    Backstory    string
}

func projectReview(ch refdata.Character) ReviewCharacter   // pulls from columns + character_data bag
```

Source fields: most come off `refdata.Character` columns; `subrace`, `background`,
`appearance`, `backstory`, `spells`, `weapon_masteries` live in the `character_data` JSONB
bag (same place `submissionFromCharacter` reads them). Lists are **sorted** before storage so
re-ordering never shows as a spurious diff.

### Part A — Full review (new + edit)

Extend `ApprovalDetail` (`internal/dashboard/approval.go`) to include the full
`ReviewCharacter` projection of the **current** character (the "after" / current state).
Render it in an expandable panel in `CharacterApprovalQueue.svelte`, fetched lazily from
`GET /dashboard/api/approvals/{id}` when the DM expands an entry. Also surface an
"Open full sheet ↗" link to `GET /portal/character/{character_id}` for the exhaustive
rendered view.

### Part B — Edit diff (before → after)

1. **Snapshot baseline at edit time.** Add a nullable JSONB column
   `player_characters.review_before`. In `BuilderService.UpdateCharacter`, when a player edit
   transitions an **approved** character to pending, capture `projectReview(existing)` of the
   **pre-edit** character and persist it into `review_before`.
   - The pre-edit character is `existing` in `UpdateCharacterRecord`
     (`builder_store_adapter.go:340`). Plumb it out (return it, or capture in a dedicated
     store call) so the service can store the baseline alongside `SetPlayerCharacterPending`.
   - **Only set the baseline on the `approved → pending` transition.** On a `pending → pending`
     re-edit (e.g. after `changes_requested`), do **not** overwrite `review_before` — keep the
     original last-approved baseline so the diff still reflects everything changed since the
     last approval.
2. **Clear on resolution.** Set `review_before = NULL` on **approve** (the new state becomes
   the implicit baseline; the next edit re-snapshots from the then-current record). Leave it
   set through reject / request-changes so the diff persists across resubmits until an approve.
3. **Return both states.** `GetApprovalDetail` returns `review_before` (the baseline, or null)
   plus the current `ReviewCharacter` (after). New characters have `review_before = null` →
   panel shows full review only, no diff.
4. **Render the diff.** In the panel, when `review_before` is present, run
   `diffStates(review_before, after)` and show a "Changes since last approval" section
   (CHA 16 → 18, Skills +Stealth −Acrobatics, Spells +Mage Armor, …). Always show the full
   current projection beneath it.

### Data flow summary

```
player edit (approved→pending):
  UpdateCharacterRecord loads `existing` (pre-edit)  ──► projectReview(existing) ──► player_characters.review_before
  characters row overwritten with new build (after)  ──► current state

DM opens approval:
  GET /dashboard/api/approvals/{id}
    ├─ after  = projectReview(current characters row)
    └─ before = review_before (or null)
  UI: diffStates(before, after) + full `after` projection + "open full sheet" link

DM approves: review_before = NULL  (baseline resets; next edit re-snapshots)
```

## Schema change

Migration: `ALTER TABLE player_characters ADD COLUMN review_before JSONB;` (nullable, default
null) with a matching down migration that drops it.

**Migration gotchas** (see memory `project_new_migration_test_hooks.md`):
- Adding a *column* (not a table) should not change the table list in `internal/testutil/testdb.go`,
  but **verify** — update it if the truncation/reset helper enumerates anything column-aware.
- Update the `database` package **MigrateDown** round-trip test so down-migration reversibility
  still passes.
- Run `make sqlc-check` after editing `db/queries/player_characters.sql` (the approval queries
  are sqlc-generated; never hand-edit `*.sql.go`).

## Backend changes

| Area | File(s) | Change |
|---|---|---|
| Migration | `db/migrations/` (new up+down pair) | add/drop `player_characters.review_before JSONB` |
| Queries | `db/queries/player_characters.sql` | `GetPlayerCharacterWithCharacter` also selects `pc.review_before`; new `SetReviewBefore` / clear; ensure approve clears it. `make sqlc-check`. |
| Projection | new `internal/dashboard/review.go` (or `internal/portal`) | `ReviewCharacter` + `projectReview(refdata.Character)` (+ unit tests) |
| Snapshot capture | `internal/portal/builder_service.go` (`UpdateCharacter`), `builder_store_adapter.go` (expose pre-edit `existing`) | set baseline on approved→pending only |
| Approval API | `internal/dashboard/approval.go`, `approval_store.go`, `approval_handler.go` | `ApprovalDetail` gains `review` (after projection) + `review_before`; `GetApprovalDetail` populates both |
| Approve action | `internal/dashboard/approval_store.go` (`ApproveCharacter`/`transitionStatus`) | clear `review_before` on approve |

## Frontend changes

| Area | File(s) | Change |
|---|---|---|
| Review/diff panel | `dashboard/svelte/src/CharacterApprovalQueue.svelte` (+ maybe a new `CharacterReviewPanel.svelte`) | expand entry → lazy `GET /dashboard/api/approvals/{id}`; render full projection; when `review_before` present, render `diffStates(before, after)`; "Open full sheet ↗" → `/portal/character/{character_id}` |
| Diff helper | `dashboard/svelte/src/lib/diff.js` | reuse `diffStates`; add light value-formatting for arrays/objects if needed (tested) |
| Build | `dashboard/svelte` (vite) | `npm run build` outputs to the embedded assets dir; rebuild Go binary to embed (see [build note](#builddeploy-note)) |

## Auth

All approval endpoints are already DM-only and campaign-scoped (`requireAuth`,
`resolveCampaign`, `checkCampaignOwnership` in `approval_handler.go`). The optional
full-sheet link relies on `CanViewCharacter`, which already authorizes the campaign DM for
pending characters. No new auth work.

## Implementation phases (TDD; red → green each)

1. **Projection** — `ReviewCharacter` + `projectReview` with unit tests (incl. character_data
   bag fields, list sorting). Pure, no I/O.
2. **Migration + queries** — add `review_before`; extend/adjust sqlc queries; update
   `testutil/testdb.go` if needed + the MigrateDown test; `make sqlc-check`.
3. **Snapshot capture** — `UpdateCharacter` sets baseline on approved→pending only; not on
   pending→pending; cleared on approve. Service + adapter tests covering each transition and
   the new-character (no baseline) case.
4. **Approval API** — `ApprovalDetail` returns `review` + `review_before`; handler/store tests.
5. **Frontend panel** — expandable review + diff; vitest for any new lib logic; manual
   browser verify via the live approval page.
6. **Verify** — `make cover-check` (90% overall / 85% pkg), `npx vitest run`, and an
   end-to-end browser check: have a non-DM player edit an approved character, confirm the DM
   sees the diff; confirm a new submission shows full review with no diff.

## Open questions / decisions

- **Q1 — Panel vs deep-link for the full sheet.** Decision: build the in-dashboard projection
  panel (needed for the diff anyway) and *also* deep-link the full `/portal/character/{id}`
  sheet for the exhaustive view. Rationale: the projection drives the diff; the existing sheet
  gives a zero-cost complete render.
- **Q2 — Baseline semantics.** Decision: `review_before` = the last **approved** state; set on
  approved→pending, preserved across changes_requested resubmits, cleared on approve. This
  answers "what changed since I last approved," which is the DM's question.
- **Q3 — DM self-edits.** When the requester is the campaign DM, the edit applies instantly and
  never goes pending (`isDM` path), so no baseline is captured and it never appears in the
  queue — unchanged, intended.
- **Q4 — Equipment/spell naming.** Project to stable display names and sort, so the diff shows
  meaningful adds/removes rather than ID churn. Confirm against how `submissionFromCharacter`
  reconstructs these lists.

## Build/deploy note

The dashboard SPA is embedded via `//go:embed`. `vite build` writes to the embedded assets
dir; the Go binary must be rebuilt to pick up frontend changes. Local redeploy is detached:
`docker compose up -d --build` (see memory `project_local_redeploy_detached.md`).

## Reference index (anchors)

- Approval queue UI: `dashboard/svelte/src/CharacterApprovalQueue.svelte`
- Diff helper: `dashboard/svelte/src/lib/diff.js:11`
- Approval API: `internal/dashboard/approval_handler.go` (routes `:45-53`), `approval.go`, `approval_store.go:85`
- Edit overwrite / snapshot point: `internal/portal/builder_store_adapter.go:335` (`existing` at `:340`)
- Pending transition: `internal/portal/builder_service.go:664` (`!isDM` block ~`:700`)
- Full sheet: `internal/portal/character_sheet_handler.go:49`, auth `internal/portal/character_sheet.go:130-163`
- Char model: `internal/refdata/models.go:67-101`; domain types `internal/character/types.go`
- Migration test hooks: memory `project_new_migration_test_hooks.md`
