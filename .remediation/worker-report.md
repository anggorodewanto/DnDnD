# Worker Report: F-H04

**Worker:** worker-F-H04
**Status:** ✅ Complete
**Date:** 2026-05-16

## Finding

Auto-resolve fired on prefix match (e.g. "open") even when the description contained words implying DM adjudication ("locked", "trapped", "stuck", "barred", "sealed").

## Fix Applied

Added a `dmRequiredKeywords` blocklist in `internal/combat/interact.go`. After a prefix match succeeds, the description is checked for any DM-required keyword. If found, `isAutoResolvable` returns `false`, routing the interaction to the DM queue.

## Files Changed

- `internal/combat/interact.go` — added `dmRequiredKeywords` slice and blocklist check inside `isAutoResolvable`.
- `internal/combat/interact_test.go` — added 5 test cases to `TestInteract_AutoResolvablePatterns` covering blocked keywords ("locked chest", "barred gate", "trapped handle", "sealed vault", "stuck door").

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
