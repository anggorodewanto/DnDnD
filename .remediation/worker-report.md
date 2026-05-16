# Worker Report: J-H01

**Finding:** Campaign Home cards show player-facing display_name, not the spoilery internal name  
**Status:** FIXED  
**Worker:** worker-J-H01  

## Changes Made

### 1. `cmd/dndnd/main.go`

- Introduced `encounterListerQueries` interface (narrow, 2 methods) to decouple the adapter from `*refdata.Queries` — enables unit testing without a live DB.
- Changed `encounterListerAdapter.queries` field type from `*refdata.Queries` to `encounterListerQueries`. Production wiring is unchanged (`*refdata.Queries` satisfies the interface).
- `ListActiveEncounterNames`: removed the `if e.DisplayName.Valid` preference logic; now returns `e.Name` unconditionally.
- `ListSavedEncounterNames`: same fix for templates.

### 2. `cmd/dndnd/main_wiring_test.go`

- Added `stubEncounterListerQueries` (implements the new interface with canned data).
- Added `TestEncounterListerAdapter_ReturnsInternalName_NotDisplayName`: an encounter with `Name="Dragon's Lair (spoiler)"` and `DisplayName="Mysterious Cave"` must return `"Dragon's Lair (spoiler)"` for both active and saved lists.

## TDD Sequence

1. **Red:** Test written asserting `Name` is returned. Test failed — adapter returned `"Mysterious Cave"` (DisplayName).
2. **Green:** Removed DisplayName-preference logic in both methods. Test passed.
3. **Verify:** `make test` — all tests pass. `make cover-check` — all coverage thresholds met.

## No Commit

Per instructions, no commit was made.
