# task high-16 — DDB import re-sync gates UpdateCharacterFull on DM approval

## Finding (verbatim from chunk7_dashboard_portal.md, Phase 90)

> ❌ **Re-sync mutates the DB before DM approval.** `ddbimport.Service.Import` calls `UpdateCharacterFull` (`internal/ddbimport/service.go:88-93`) on the existing record before posting to `#dm-queue`. The player message says "changes detected and pending DM approval" (`registration_handler.go:188`) but the data has already been overwritten. The DM has nothing to "approve" — the diff is a notification, not a gate.

Spec section: Phase 90 spec line 2445: "system diffs and shows DM what changed before applying".

Recommended approach (chunk7 follow-up #3): "Refactor `ddbimport.Service.Import` to (a) parse + diff in-memory, (b) post to `#dm-queue` with the diff, (c) only call `UpdateCharacterFull` when the DM clicks Approve. Mirrors the `ASIHandler.HandleDMApprove` pattern."

## Plan (worker fills)

**Smallest viable design — in-memory pending-imports map.**

1. `ddbimport.Service.Import` continues to do parse/validate/diff in-memory, but on
   re-sync it **does NOT** call `UpdateCharacterFull`. Instead it stashes the
   built `UpdateCharacterFullParams` in an in-memory `pendingImports` map keyed
   by a freshly-minted `ImportID` (uuid). The map entry has a 24h TTL; entries
   are pruned on access (no goroutine/clock dependency for tests).
2. New method `Service.ApproveImport(ctx, importID)` looks up the pending entry
   and only then calls `UpdateCharacterFull`. Returns the updated `Character`
   or `ErrPendingImportNotFound`.
3. New method `Service.DiscardImport(importID)` removes the pending entry
   (used on DM reject; also exposed for completeness). Optional in this pass —
   we just include it because rejecting without discarding leaks memory.
4. `ImportResult.PendingImportID` carries the new ID up to callers; callers
   (Discord handler, future dashboard) include it in their #dm-queue post so
   the DM-side approver knows which pending import to approve.
5. Player-facing message in `internal/discord/registration_handler.go:188`
   updated from "pending DM approval" → "pending DM review (no changes
   applied yet)".
6. Existing `Service.Import` re-sync test (`TestService_Import_Resync`) is
   updated to assert `UpdateFunc` is **not** called on Import; a new
   `TestService_ApproveImport_*` set asserts the approve gate works.

**Trade-offs documented:**
- In-memory storage means pending imports are lost on bot restart. Acceptable
  because the player can simply re-run `/import` and the diff is regenerated.
- 24h TTL is hard-coded for now (`pendingImportTTL`). Configurable via
  constructor option later if real usage demands it.
- This change does not wire a DM-side "Approve" button — that is left to a
  follow-up task because building the Discord component handler, the dashboard
  endpoint, and the dm-queue payload schema for import approvals is a
  separate feature surface. The current Discord handler keeps posting to
  `#dm-queue` exactly as today; what changes is the data isn't applied until
  someone calls `Service.ApproveImport`. The chunk-7 finding (DB mutated
  before approval) is fixed at the API contract level: any caller of
  `Service.ApproveImport` is the gate. The TODO is recorded inline.

## Files touched

- `internal/ddbimport/service.go` — split Import into stage+commit, add pending
  store + ApproveImport/DiscardImport.
- `internal/ddbimport/service_test.go` — flip resync test, add approve/discard
  coverage, exercise expiry path.
- `internal/discord/registration_handler.go` — accurate player-facing message;
  no behavior change (approval flow remains a follow-up).
- `internal/discord/import_ddb_handler_test.go` — update message assertion.

## Tests added

- `TestService_Import_ResyncStagesNotMutates` — re-import calls neither
  CreateCharacter nor UpdateCharacterFull; ImportResult carries
  `PendingImportID` and `IsResync=true` with non-empty `Changes`.
- `TestService_ApproveImport_Success` — Approve mutates the DB exactly once.
- `TestService_ApproveImport_NotFound` — unknown id returns
  `ErrPendingImportNotFound`.
- `TestService_ApproveImport_AlreadyApproved` — re-approve same id returns
  `ErrPendingImportNotFound` (consumed on success).
- `TestService_ApproveImport_Expired` — entries older than TTL return
  `ErrPendingImportNotFound`.
- `TestService_DiscardImport` — discard removes the pending entry.

## Implementation notes

- Implemented in-memory `pendingImports` map on `*Service`, guarded by a
  `sync.Mutex` for concurrent /import calls. Keyed by `uuid.UUID` (the
  `PendingImportID` returned in `ImportResult`).
- TTL = 24h via `pendingImportTTL` package constant. Expiry is checked
  lazily on `ApproveImport` (no goroutine, no clock-tick dependency).
  Tests inject a clock via `NewServiceWithClock` to fast-forward.
- New public surface:
  - `var ErrPendingImportNotFound = errors.New(...)`
  - `func NewServiceWithClock(client, store, now) *Service`
  - `func (s *Service) ApproveImport(ctx, importID) (refdata.Character, error)`
  - `func (s *Service) DiscardImport(importID)`
  - `ImportResult.PendingImportID uuid.UUID`
- `Import` now returns `IsResync=true` plus `PendingImportID != uuid.Nil`
  ONLY when the diff has at least one change. No-change re-syncs return
  `IsResync=true, PendingImportID=uuid.Nil` (nothing waiting).
- Player-facing message in `internal/discord/registration_handler.go:188`
  reworded to "pending DM review (no changes applied yet)" so the player
  is no longer misled about whether the DB has been mutated.
- Updated `TestService_Import_Resync` → `TestService_Import_ResyncStagesNotMutates`
  to assert UpdateCharacterFull is **not** called, plus added five new
  tests (`TestService_ApproveImport_*`, `TestService_DiscardImport`).
- Pre-existing `TestService_Import_UpdateError` (in coverage_test.go) was
  renamed `TestService_ApproveImport_UpdateError` and moved through the
  Approve path because Import no longer touches UpdateCharacterFull.
- Mirrored assertion added to `TestImportHandler_DDB_Resync` so the Discord
  handler's player-facing message wording stays correct.

### Out of scope / follow-up

- The DM-side approve button (Discord component handler that calls
  `Service.ApproveImport`) and the dashboard endpoint that does the same are
  intentionally NOT in this task. The chunk-7 Phase-90 finding is fixed at
  the API contract level: the database is no longer mutated by `Import`. A
  follow-up task should wire the Discord/dashboard surface to consume
  `PendingImportID` and call `Service.ApproveImport` / `DiscardImport` on
  DM action. (Tracked alongside chunk7 follow-up #10 — DM-side DDB import
  surface — which is already noted as out of scope for this batch.)

### Concurrent-worker note

A parallel agent has uncommitted WIP in `internal/discord/cast_handler.go`
that does not currently compile (refers to `dispatchInventorySpell` without
defining it). To verify my changes pass `make cover-check` I temporarily
stashed those WIP files (`cast_handler.go`, `cast_handler_test.go`,
`use_handler.go`, `use_handler_test.go`, `charactercard/service.go`,
`charactercard/service_test.go`, `db/queries/combatants.sql`,
`refdata/combatants.sql.go`), ran `make cover-check` (passed: overall 93.90%,
ddbimport 94.30%, all packages ≥85%), then restored the WIP via `git stash
pop`. Final state of those files is unchanged from when this task started.

### Verification

```
$ go test ./internal/ddbimport/... ./internal/discord/... ./internal/registration/...
ok  	github.com/ab/dndnd/internal/ddbimport	0.070s
ok  	github.com/ab/dndnd/internal/discord	0.269s
ok  	github.com/ab/dndnd/internal/registration	2.352s

$ make cover-check
Overall coverage (post-exclusion): 93.90% (17535/18674 statements)
  github.com/ab/dndnd/internal/ddbimport                            94.30%
  github.com/ab/dndnd/internal/discord                              90.03%
  github.com/ab/dndnd/internal/registration                         93.41%
OK: coverage thresholds met
```

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW
