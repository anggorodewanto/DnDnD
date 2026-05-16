# Worker Report: E-H03

**Finding:** Pact-magic upcast respects pact level but silently ignores --slot requests  
**Status:** ✅ Fixed  
**Worker:** worker-E-H03  
**Date:** 2026-05-16

## Summary

When `cmd.SlotLevel` was set and the pact path was taken, the requested slot level was silently overridden to the pact slot level. Players could not detect that their `--slot` flag was being ignored.

## Changes

### `internal/combat/spellcasting.go` (line ~451)

Added validation in the pact-magic branch: if `cmd.SlotLevel > 0 && cmd.SlotLevel != pactSlots.SlotLevel`, return an error:

```
"Pact slots always cast at level %d; cannot use --slot %d"
```

### `internal/combat/spellcasting_test.go`

Added `TestCast_PactSlot_RejectsSlotLevelMismatch`: a warlock with pact level 3 requests `--slot 5`, expects an error containing the descriptive message.

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
