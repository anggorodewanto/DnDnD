# Worker Report: A-H09

**Finding:** Sessions middleware continues serving request when SlideTTL fails  
**Status:** ✅ Fixed  

## Changes

### `internal/auth/oauth2_test.go`
- Added `slideTTLErr` field to `mockSessionRepo`
- `SlideTTL` now returns `m.slideTTLErr` when set

### `internal/auth/middleware_test.go`
- Added `TestSessionMiddleware_SlideTTLError_Returns500`: injects a SlideTTL error and asserts 500 response with handler not called

### `internal/auth/middleware.go` (lines 63–68)
- When `SlideTTL` returns an error, the middleware now aborts with `http.Error(w, "internal server error", 500)` and returns (fail-closed)
- Cookie re-issue moved out of `else` block to execute unconditionally after successful slide

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
