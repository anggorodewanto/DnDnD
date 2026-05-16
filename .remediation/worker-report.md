# Worker Report: E-H02

**Finding:** AoE pending save DC subtraction loses cover information  
**Status:** ✅ Fixed  
**Worker:** worker-E-H02

## Problem

In `internal/combat/aoe.go:592`, the pending save was created with `Dc: int32(ps.DC - ps.CoverBonus)`, pre-subtracting cover from DC. This meant:
- The saver never sees the bonus in their roll log
- The DC displayed is artificially lowered
- Cover bonus should be added to the saver's roll, not subtracted from DC

## Fix Applied

### 1. Database schema (new migration)
- `db/migrations/20260516120000_add_pending_saves_cover_bonus.sql`: adds `cover_bonus INT NOT NULL DEFAULT 0` column to `pending_saves`.

### 2. SQL query update
- `db/queries/pending_saves.sql`: `CreatePendingSave` now inserts `cover_bonus` as a 6th parameter.

### 3. sqlc regeneration
- `internal/refdata/models.go`: `PendingSafe` struct now has `CoverBonus int32` field.
- `internal/refdata/pending_saves.sql.go`: `CreatePendingSaveParams` now has `CoverBonus int32` field; all scan calls updated.

### 4. Storage fix (`internal/combat/aoe.go`)
- Changed `Dc: int32(ps.DC - ps.CoverBonus)` → `Dc: int32(ps.DC)` 
- Added `CoverBonus: int32(ps.CoverBonus)` to `CreatePendingSaveParams`

### 5. Resolution fix (`internal/combat/aoe.go` — `RecordAoEPendingSaveRoll`)
- Changed `success := !autoFail && total >= int(r.Dc)` → `success := !autoFail && total+int(r.CoverBonus) >= int(r.Dc)`
- Changed `RollResult: sql.NullInt32{Int32: int32(total), Valid: true}` → `RollResult: sql.NullInt32{Int32: int32(total + int(r.CoverBonus)), Valid: true}`

### 6. Tests updated (`internal/combat/aoe_test.go`)
- **New test:** `TestRecordAoEPendingSaveRoll_EH02_CoverBonusAddedToRoll` — verifies roll 13 + cover bonus 2 = 15 >= DC 15 succeeds.
- **Updated:** `TestCastAoE_F05_CoverBonusReducesStoredDC` — now asserts stored DC=15 (original) and stored cover_bonus=2.
- **Updated:** `TestRecordAoEPendingSaveRoll_F05_CoverBonusMakesSaveSucceed` — now asserts roll+cover logic instead of reduced-DC logic.

## Verification

- `go test ./internal/combat/` — ✅ all pass (92.4% coverage)
- `make test` — ✅ passes (pre-existing `TestIntegration_MigrateDown` failure is unrelated)
- `make cover-check` — ✅ passes (same pre-existing failure only)

## Files Changed

- `db/migrations/20260516120000_add_pending_saves_cover_bonus.sql` (new)
- `db/queries/pending_saves.sql`
- `internal/refdata/models.go` (sqlc-generated)
- `internal/refdata/pending_saves.sql.go` (sqlc-generated)
- `internal/combat/aoe.go`
- `internal/combat/aoe_test.go`
