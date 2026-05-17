# Worker Report: D-H08

**Worker:** worker-D-H08
**Finding:** D-H08 — ChannelDivinityDMQueue deducts a use even when dmNotifier is nil
**Status:** ✅ Fixed

## Summary

Added a nil-check for `s.dmNotifier` at the top of `ChannelDivinityDMQueue`. When no DM notifier is wired, the method now returns an error immediately — before deducting a Channel Divinity use or consuming the action resource.

## Changes

### `internal/combat/channel_divinity.go`
- Added `if s.dmNotifier == nil { return DMQueueResult{}, fmt.Errorf(...) }` as the first check in `ChannelDivinityDMQueue`.

### `internal/combat/channel_divinity_test.go`
- Added `TestChannelDivinityDMQueue_NilNotifier_ReturnsError`: calls `ChannelDivinityDMQueue` with no notifier wired, asserts error containing "no DM notifier".
- Updated existing DMQueue error-path tests to wire `&fakeDMNotifier{}` so they exercise the intended error paths (downstream of the new nil check).

### `internal/combat/channel_divinity_integration_test.go`
- Added `integrationFakeDMNotifier` (no-op implementation of `DMNotifier`) for the external test package.
- Wired it in `TestIntegration_ChannelDivinityDMQueue`.

## TDD Cycle

1. **Red:** `TestChannelDivinityDMQueue_NilNotifier_ReturnsError` failed — function returned nil error.
2. **Green:** Added nil guard; test passes.
3. **Refactor:** Updated existing tests that now hit the guard before their intended error path.

## Test Results

```
ok  github.com/ab/dndnd/internal/combat  16.691s
```

All packages pass (`go test ./... -short`, excluding `internal/database`).
