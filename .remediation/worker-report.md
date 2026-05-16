# Worker Report: G-H02

**Finding:** Long-rest hit-dice restoration order is non-deterministic for multiclass  
**Status:** ✅ Fixed  
**Worker:** worker-G-H02  
**Date:** 2026-05-16

## What was done

### 1. Red — Failing test added

`TestLongRest_HitDiceRestore_MulticlassDeterministic` in `internal/rest/rest_test.go`:
- Fighter 3 / Wizard 2 (d10 + d6), all hit dice spent, budget = 2.
- Runs 100 iterations to detect non-determinism.
- Asserts d10 is always restored first (2 dice), d6 stays at 0.
- Confirmed failure before fix (failed at iteration 10).

### 2. Green — Fix applied

`internal/rest/rest.go` (hit dice restoration loop ~line 422):
- Extracted map keys into a sorted slice (largest die value first: d12 > d10 > d8 > d6 > d4).
- Added `dieValue()` helper to parse numeric value from die string.
- Added `"sort"` and `"strconv"` imports.

### 3. Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met (rest package: 93.46%).

## Files modified

- `internal/rest/rest.go` — sorted die keys before iteration; added `dieValue` helper.
- `internal/rest/rest_test.go` — added determinism test.
