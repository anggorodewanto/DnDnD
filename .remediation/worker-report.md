# Worker Report: D-H03

**Status:** ✅ Complete  
**Worker:** worker-D-H03  
**Date:** 2026-05-16

## Problem

`attackAbilityUsed` picks the higher of STR/DEX for finesse weapons. A raging barbarian with higher DEX gets "dex", causing the rage damage bonus (which requires `ability_used: str`) to silently drop.

## Fix Applied

Added `isRaging bool` parameter to `attackAbilityUsed`. When raging and weapon is finesse, always returns "str" before the DEX>STR comparison.

### Files Modified

- `internal/combat/attack.go` — Added `isRaging` param; early-return "str" for raging+finesse; updated call site in `populateAttackFES`.
- `internal/combat/attack_fes_test.go` — Updated existing calls with `false`; added test case: raging barbarian STR 14 / DEX 16 with rapier expects "str".

## TDD Evidence

1. **Red:** Added test `"raging finesse forces str even with higher dex"` — would fail without the fix (function previously had no `isRaging` param).
2. **Green:** Added the override logic. All tests pass.
3. **Tests run:** `go test $(go list ./... | grep -v internal/database)` — all 45 packages OK.

## Acceptance Criterion

> A raging barbarian with higher DEX than STR using a finesse weapon gets "str" as the ability used. A test demonstrates this.

✅ Met.
