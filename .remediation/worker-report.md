# Worker Report: G-H05

## Finding
**G-H05**: `CreateLootPool` reads NPC inventory/gold into the loot pool but never clears them from the NPC character, allowing duplicate loot on re-invocation.

## Fix Applied

**File:** `internal/loot/service.go` (CreateLootPool, after item/gold collection loop)

Added a second loop over defeated NPC combatants that calls:
- `UpdateCharacterGold(ctx, {ID: npcCharID, Gold: 0})`
- `UpdateCharacterInventory(ctx, {ID: npcCharID, Inventory: {Valid: false}})`

This zeroes gold and clears inventory for each defeated NPC whose items were moved into the pool.

## TDD Workflow

1. **Red:** Added `TestCreateLootPool_ClearsNPCGoldAndInventory` — asserts NPC gold == 0 and inventory is invalid after pool creation. Confirmed failure.
2. **Green:** Added clearing calls in `service.go`. Test passes.
3. **Mock fixes:** Updated two existing mock-based tests (`TestCreateLootPool_CreateLootPoolItemFailure`, `TestCreateLootPool_SkipsInvalidInventory`) to provide stubs for the new store calls.

## Verification

- `make test` — all packages pass.
- `make cover-check` — all coverage thresholds met.

## Files Modified
- `internal/loot/service.go` — added NPC clearing logic
- `internal/loot/service_test.go` — added integration test
- `internal/loot/service_mock_test.go` — added mock stubs to 2 existing tests
