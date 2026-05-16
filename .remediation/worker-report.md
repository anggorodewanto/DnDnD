# Worker Report: F-H03

**Finding:** Hidden combatants (IsVisible=false) still render on the map  
**Worker:** worker-F-H03  
**Status:** ✅ FIXED

## Changes Made

### 1. `internal/gamemap/renderer/types.go`
- Added `IsVisible bool` field to the `Combatant` struct (between `IsStable` and `InFog`).

### 2. `internal/gamemap/renderer/fog.go` (filterCombatantsForFog)
- Added check: after the `IsPlayer` early-continue, non-player combatants with `IsVisible == false` are skipped.
- The `DMSeesAll` path (early return at top of function) already returns all combatants unmodified, so DM view still shows hidden combatants.

### 3. `internal/gamemap/renderer/fog_hidden_test.go` (NEW)
- `TestFilterCombatantsForFog_HiddenExcluded`: enemy with `IsVisible=false` excluded from player view; player with `IsVisible=false` still shown.
- `TestFilterCombatantsForFog_HiddenShownForDM`: enemy with `IsVisible=false` included when `DMSeesAll=true`.

### 4. `internal/gamemap/renderer/fow_test.go` (existing test fix)
- Added `IsVisible: true` to enemy combatants in `TestFilterCombatantsForFog` that are expected to pass the visibility filter (they test fog-tile filtering, not hide filtering).

## Verification

- `make test` — PASS (all packages)
- `make cover-check` — PASS (all thresholds met)
- TDD workflow followed: Red → Green → verified no regressions.
