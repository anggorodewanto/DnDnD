# Worker Report: E-H05

**Finding:** Spell attack rolls never apply advantage/disadvantage  
**Status:** ✅ Fixed  
**Worker:** worker-E-H05  
**Date:** 2026-05-16

## Changes

### `internal/combat/spellcasting.go`
1. Added `SpellAttackRollMode dice.RollMode` field to `CastCommand` struct (line 339).
2. Changed `roller.RollD20(attackMod, dice.Normal)` → `roller.RollD20(attackMod, cmd.SpellAttackRollMode)` (line 640). Zero-value of `dice.RollMode` is `dice.Normal` (0), so backward compatibility is preserved.

### `internal/combat/spellcasting_test.go`
Added `TestCast_SpellAttackRollMode_Advantage` — casts Fire Bolt with `SpellAttackRollMode: dice.Advantage`, uses a roller returning 8 then 15, asserts the higher roll (15) is chosen and attack total = 22 (15 + 7 modifier).

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
