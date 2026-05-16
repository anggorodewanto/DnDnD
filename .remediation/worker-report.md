# Worker Report: G-H08 — Party long rest dawn recharge

## Finding

`applyPartyLongRest` in `internal/rest/party_handler.go` built `LongRestInput` without `Inventory` or `RechargeInfo`, so the dawn-recharge logic in `LongRest()` never fired for party rests.

## Fix Applied

1. Added `Inventory []character.InventoryItem` and `RechargeInfo map[string]inventory.RechargeInfo` fields to `PartyCharacterInfo`.
2. Added `UpdatedInventory []character.InventoryItem` field to `CharacterRestUpdate`.
3. Wired both fields into `LongRestInput` construction and propagated `result.UpdatedInventory` into the `CharacterRestUpdate`.

## Files Modified

- `internal/rest/party_handler.go` — added fields + wiring
- `internal/rest/party_handler_test.go` — added `TestPartyRestHandler_LongRest_DawnRecharge`

## Test Result

- New test: `TestPartyRestHandler_LongRest_DawnRecharge` — PASS
- Full suite (excluding `internal/database`): ALL PASS (38 packages)

## TDD Cycle

- **Red:** Test failed to compile (fields didn't exist on structs).
- **Green:** Added fields and wiring; test passes.
- **Refactor:** No refactoring needed — minimal change.
