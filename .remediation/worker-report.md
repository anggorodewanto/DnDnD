# Worker Report: H-H11

**Finding:** DDB class names not normalised to internal IDs  
**Worker:** worker-H-H11  
**Status:** ✅ FIXED

## Changes Made

### 1. `internal/ddbimport/parser.go` (line 177)
- Wrapped `c.Definition.Name` with `strings.ToLower()` when assigning to `ClassEntry.Class`.

### 2. `internal/ddbimport/parser_test.go`
- Added `TestParseDDBJSON_ClassNameLowercased`: asserts DDB class "Fighter" is stored as "fighter".
- Updated `TestParseDDBJSON_Classes` to expect `"fighter"` instead of `"Fighter"`.

### 3. `internal/ddbimport/service_test.go`
- Updated `TestService_Import_WizardCureWoundsAdvisoryAndPersistedTag` advisory assertion from `"Wizard spell list"` to `"wizard spell list"` (downstream effect of the fix).

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
