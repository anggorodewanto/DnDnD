# Worker Report: B-H02

**Finding:** RenderMap mutates caller-supplied MapData.TileSize
**Status:** FIXED ✅

## What was done

### Red (failing test)
Added `TestRenderMap_DoesNotMutateTileSize` in `renderer_test.go`:
- Creates `MapData` with `TileSize=64` and `Width=150` (triggers large-map path)
- Calls `RenderMap`
- Asserts `md.TileSize` is still 64 after the call
- Confirmed test **failed** before fix: `got 32, want 64`

### Green (fix)
In `renderer.go`, replaced the direct mutation:
```go
// Before (mutates caller):
if md.Width > 100 || md.Height > 100 {
    md.TileSize = 32
}
```
With a local variable + defer restore pattern:
```go
// After (no permanent mutation):
tileSize := md.TileSize
if md.Width > 100 || md.Height > 100 {
    tileSize = 32
}
origTileSize := md.TileSize
md.TileSize = tileSize
defer func() { md.TileSize = origTileSize }()
```
This ensures draw helpers still see the reduced tile size during rendering, but the caller's struct is restored on exit.

### Verification
- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
- Existing `TestRenderMap_LargeMapTileSize` still passes (image dimensions correct with 32px tiles)

## Files changed
- `internal/gamemap/renderer/renderer.go` (lines 13–19)
- `internal/gamemap/renderer/renderer_test.go` (added test at end)
