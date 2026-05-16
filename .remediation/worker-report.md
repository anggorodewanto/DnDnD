# Worker Report: F-H07

**Finding:** TriggerCounterspell doesn't validate distance between declarant and enemy caster.

**Status:** ✅ Fixed

## Changes Made

### 1. `internal/combat/counterspell.go`
- Added `ErrCounterspellOutOfRange` sentinel error.
- Added `distanceFt int` parameter to `TriggerCounterspell`.
- Added range check: rejects with `ErrCounterspellOutOfRange` if `distanceFt > 60`.

### 2. `internal/combat/counterspell_test.go`
- Added `TestTriggerCounterspell_RejectsDistanceOver60` — verifies 65ft returns error (RED→GREEN).
- Added `TestTriggerCounterspell_AllowsDistanceAt60` — verifies 60ft succeeds (boundary).
- Updated all existing `TriggerCounterspell` calls to pass `distanceFt: 30` (valid distance).
- Updated handler test JSON bodies to include `"distance_ft": 30`.

### 3. `internal/combat/handler.go`
- Added `DistanceFt int` field to `triggerCounterspellRequest`.
- Passes `req.DistanceFt` to `TriggerCounterspell`.

### 4. `internal/discord/counterspell_prompt.go`
- Added `DistanceFt int` to `CounterspellPromptArgs` struct.
- Updated `CounterspellService` interface signature.
- Passes `args.DistanceFt` to `TriggerCounterspell`.

### 5. `internal/discord/counterspell_prompt_test.go`
- Updated mock to match new interface signature.

## Verification

- `go test -count=1 ./internal/combat/` — PASS
- `go test ./internal/discord/...` — PASS
- `make cover-check` — only pre-existing unrelated failure (`TestIntegration_MigrateDown` in `internal/database`)
- New tests confirm: 65ft → error, 60ft → success
