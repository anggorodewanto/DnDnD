# Worker Report: cross-cut-H05

**Status:** ✅ Complete  
**Worker:** worker-crosscut-H05  
**Date:** 2026-05-16

## Finding

Action Surge max uses never scaled to 2 at Fighter level 17+. The logic was inlined in `init_feature_uses.go` but no reusable function existed.

## Changes Made

### 1. `internal/combat/action_surge.go`
Added `ActionSurgeMaxUses(fighterLevel int) int` — returns 2 at level ≥ 17, 1 otherwise.

### 2. `internal/combat/action_surge_test.go`
Added `TestActionSurgeMaxUses` with table-driven cases:
- Level 16 → expects 1
- Level 17 → expects 2

### 3. `internal/portal/init_feature_uses.go`
Replaced hardcoded inline logic with a call to `combat.ActionSurgeMaxUses(ce.Level)`.

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
- TDD workflow followed: Red (undefined function) → Green (function implemented) → Refactor (init_feature_uses.go updated)
