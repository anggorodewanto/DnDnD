# Worker Report: C-H07

**Finding:** Pre-clamp HP overflow excludes temp-HP absorbed damage from instant-death check  
**Verdict:** Code is correct; test added to lock the invariant.

## What was done

Added `TestApplyDamage_AtZeroHP_TempHPAbsorbedStillInstantDeath` to `internal/combat/deathsave_integration_test.go`.

The test asserts: a PC at 0 HP with 5 temp HP taking 25 slashing damage (maxHP 18) results in instant death. Temp HP absorbs 5, leaving 20 adjusted damage which is passed to `CheckInstantDeath(20, 18)` → true.

## Results

- **Test:** PASS on first run — code already handles this correctly.
- **`make test`:** PASS (all packages).
- **`make cover-check`:** PASS (all thresholds met).

## File changed

- `internal/combat/deathsave_integration_test.go` — appended one test function (26 lines).
