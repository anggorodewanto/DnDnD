# Worker Report: H-H10 — DeriveSpeed ignores race

## Status: FIXED ✅

## Summary

`DeriveSpeed` in `internal/portal/builder_store_adapter.go` always returned 30 regardless of race. Now it looks up the race in a speed map and returns the correct value (25 for dwarf/halfling/gnome, 30 for all others).

## Changes

### `internal/portal/builder_store_adapter.go`
- Added `raceSpeed` map with entries for dwarf (25), halfling (25), gnome (25).
- `DeriveSpeed` now does a case-insensitive lookup; falls back to 30 if race not in map.

### `internal/portal/builder_store_adapter_test.go`
- Added `TestDeriveSpeed_RaceLookup` covering dwarf=25, halfling=25, gnome=25, elf=30, human=30, unknown=30.

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met (portal at 88.07%).
