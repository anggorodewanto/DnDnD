# Worker Report: A-H03

**Finding:** WebSocket origin verification defaults to `InsecureSkipVerify: true`
**Status:** REMEDIATED ✅
**Worker:** worker-A-H03
**Date:** 2026-05-16

## Changes Made

### 1. New failing test (Red)

**File:** `internal/dashboard/ws_test.go`

Added `TestWebSocketEndpoint_DefaultRejectsForeignOrigin` — verifies that a
freshly-constructed `Handler` (no `SetWebSocketOriginPolicy` call) rejects a
foreign-origin WebSocket upgrade with HTTP 403.

### 2. Default flipped (Green)

**File:** `internal/dashboard/handler.go` (line ~185)

Changed `wsInsecureSkipVerify: true` → `wsInsecureSkipVerify: false` in
`NewHandler`. Updated struct and constructor comments accordingly.

### 3. Local-dev wiring updated

**File:** `cmd/dndnd/main.go` (line ~1076)

The `else` branch (when `COOKIE_SECURE` is not `"true"`) now explicitly calls
`dashHandler.SetWebSocketOriginPolicy(nil, true)` so local-dev tooling (Vite
HMR, etc.) continues to work without origin restrictions.

## Verification

| Check | Result |
|-------|--------|
| New test fails before fix (Red) | ✅ Got 101, expected 403 |
| New test passes after fix (Green) | ✅ |
| `make test` | ✅ All packages pass |
| `make cover-check` | ✅ All thresholds met |
| No unrelated code touched | ✅ |
| No commit created | ✅ |
