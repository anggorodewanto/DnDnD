# Worker Report: J-H09

**Finding:** HandleMoveConfirm doesn't publish an encounter snapshot after updating position.

## Changes Made

### `internal/discord/move_handler.go`
1. Added `MoveEncounterPublisher` interface with `PublishEncounterSnapshot(ctx, encounterID)` method.
2. Added `publisher MoveEncounterPublisher` field to `MoveHandler` struct.
3. Added `SetPublisher(p MoveEncounterPublisher)` setter method.
4. Added publish call after the confirmMove gate/no-gate block succeeds:
   ```go
   if h.publisher != nil {
       _ = h.publisher.PublishEncounterSnapshot(ctx, turn.EncounterID)
   }
   ```

### `internal/discord/move_handler_test.go`
- Added `mockMovePublisher` (records calls with mutex for concurrency safety).
- Added `TestMoveHandler_HandleMoveConfirm_PublishesSnapshot` — asserts the publisher is called exactly once with the correct encounter ID after a successful move confirm.

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
- TDD workflow followed: test written first (failed on missing `SetPublisher`), then implementation added to make it green.
