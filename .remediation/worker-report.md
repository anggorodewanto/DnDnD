# Worker Report: B-H06

## Finding

**B-H06:** DM-view fog-of-war ignores `MapData.DMSeesAll` when caller pre-computed fog.

## Status: Already Fixed — Test Added

The propagation code already exists in `renderer.go:53-54`:

```go
if md.FogOfWar != nil && md.DMSeesAll {
    md.FogOfWar.DMSeesAll = true
}
```

This was introduced in a prior commit (likely `dbb1464` batch 4). However, no test covered the **pre-computed fog** scenario specifically (existing test `TestRenderMap_DMSeesAll_RendersEnemyOnUnexplored` only tests auto-computed fog via VisionSources).

## What Was Done

1. **Red:** Wrote `TestRenderMap_PreComputedFog_DMSeesAll_ShowsAllCombatants` in `fog_extra_test.go`. Temporarily disabled the propagation to confirm the test fails (enemy on Unexplored tile is filtered out when `md.DMSeesAll=true` but `fow.DMSeesAll=false`).
2. **Green:** Restored the propagation. Test passes.
3. **Verify:** `make test` ✅ | `make cover-check` ✅

## Test Added

**File:** `internal/gamemap/renderer/fog_extra_test.go`

**Test:** `TestRenderMap_PreComputedFog_DMSeesAll_ShowsAllCombatants`

Scenario: Caller passes pre-computed `FogOfWar` with all tiles Unexplored and `DMSeesAll=false`, but sets `md.DMSeesAll=true`. Asserts that:
- `md.FogOfWar.DMSeesAll` is propagated to `true`
- Enemy combatant on Unexplored tile is NOT filtered out

## Files Modified

- `internal/gamemap/renderer/fog_extra_test.go` — added 1 test function (29 lines)
- `internal/gamemap/renderer/renderer.go` — **no changes** (fix already present)
