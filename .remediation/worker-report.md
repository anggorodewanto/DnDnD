# Worker Report: cross-cut-H03

## Finding
AttackModifier always adds proficiency bonus regardless of weapon proficiency.

## Fix Applied
Added `proficient bool` parameter to `AttackModifier`. The function now only adds `profBonus` when `proficient` is true.

### Files Changed
1. **internal/combat/attack.go** (line 104): Changed signature from `AttackModifier(scores AbilityScores, weapon refdata.Weapon, profBonus int, monkLevel ...int)` to `AttackModifier(scores AbilityScores, weapon refdata.Weapon, profBonus int, proficient bool, monkLevel ...int)`. Body gates `profBonus` addition on `proficient`.
2. **internal/combat/attack.go** (line ~490): Updated caller in `ResolveAttack` to pass `true` for backward compat.
3. **internal/combat/attack_test.go**: Added `proficient bool` field to all existing test cases (set to `true`) and added new test case `"not proficient omits proficiency bonus"` that asserts only ability mod is returned when `proficient=false`.

### TDD Workflow
- **Red**: Added test with `proficient=false` expecting only ability mod → compile error (bool passed where int expected).
- **Green**: Added `proficient bool` parameter, gated addition, updated caller → all tests pass.
- **Verify**: `make test` ✅ | `make cover-check` ✅

### Callers Updated
- `internal/combat/attack.go:ResolveAttack` — passes `true` (proficiency gating already handled upstream via `IsImprovised`/`HasTavernBrawler` zeroing `profBonus`).

No other production callers of `AttackModifier` exist (only `SpellAttackModifier` which is a separate function).

## Status: COMPLETE
