# Worker Report: C-H11

**Finding:** Two separate damage hits must produce two concentration saves.

**Status:** ✅ PASS — code is correct; regression test added.

**What was done:**
- Added `TestMaybeCreateConcentrationSaveOnDamage_TwoHitsTwoSaves` to `internal/combat/concentration_test.go`.
- The test calls `MaybeCreateConcentrationSaveOnDamage` twice on the same concentrating combatant (simulating two hits with different damage values) and asserts that `CreatePendingSave` is invoked exactly twice.
- Test passes immediately — the existing implementation correctly creates independent saves per call.

**Verification:**
- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.

**Files modified:**
- `internal/combat/concentration_test.go` (added one test function)
