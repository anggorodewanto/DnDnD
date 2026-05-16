# Worker Report: E-H04

## Finding
`resolveSpellcastingAbilityScore` returned the maximum spellcasting ability score across all classes, causing a Wizard/Cleric with INT 16 and WIS 18 to use WIS for wizard spells.

## Fix Applied

**Files modified:**
- `internal/combat/spellcasting.go` — Added `spellClasses []string` parameter to `resolveSpellcastingAbilityScore`. The function now intersects `spellClasses` with the character's classes and uses the first matching class's spellcasting ability. Falls back to max-across-all-classes when no intersection exists (feat-granted spells) or when `spellClasses` is nil.
- `internal/combat/aoe.go` — Updated call site to pass `spell.Classes`.
- `internal/combat/spellcasting_test.go` — Added `TestResolveSpellcastingAbilityScore_MulticlassUsesSpellClass` covering: wizard spell uses INT, cleric spell uses WIS, non-intersecting class falls back to max, nil spellClasses falls back to max. Updated existing test to pass `nil` for the new parameter.

## TDD Workflow
1. **Red:** Wrote test with new 3-arg signature → compile error (expected).
2. **Green:** Implemented class-intersection logic, updated call sites → all tests pass.
3. **Verify:** Full test suite (excluding `internal/database`) passes with no regressions.

## Test Results
```
=== RUN   TestResolveSpellcastingAbilityScore_NoSpellcaster
--- PASS
=== RUN   TestResolveSpellcastingAbilityScore_MulticlassUsesSpellClass
--- PASS
```
Full suite: all packages OK.
