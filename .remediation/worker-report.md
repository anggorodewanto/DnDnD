# Worker Report: B-H05 — TilesetRefs request field silently dropped by HTTP handler

## Status: ✅ FIXED

## Problem
`createMapRequest` and `updateMapRequest` in `internal/gamemap/handler.go` had no `TilesetRefs` field. The DB column and service layer supported it, but no API caller could populate it because the HTTP handler never decoded the field from the request body.

## Fix Applied

### Files Modified
- `internal/gamemap/handler.go` — Added `TilesetRefs []TilesetRef \`json:"tileset_refs,omitempty"\`` to both `createMapRequest` and `updateMapRequest` structs; wired `req.TilesetRefs` through to `CreateMapInput` and `UpdateMapInput`.
- `internal/gamemap/handler_test.go` — Added two TDD tests (`TestHandler_CreateMap_TilesetRefs`, `TestHandler_UpdateMap_TilesetRefs`) that send `tileset_refs` in the request body and assert the field reaches the store layer.

### TDD Cycle
1. **Red:** Wrote failing tests asserting `capturedRefs.Valid == true` after POST/PUT with `tileset_refs`. Both failed with "tileset_refs should be populated".
2. **Green:** Added the field to both request structs and wired it into the service input construction. Both tests pass.
3. **Verify:** `make test` — all tests pass. `make cover-check` — all coverage thresholds met.

## Verification
```
$ make test        → PASS (all packages)
$ make cover-check → OK: coverage thresholds met
```
