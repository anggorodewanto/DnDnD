# Worker Report: A-H04

**Worker:** worker-A-H04
**Date:** 2026-05-16
**Status:** ✅ Complete

## Finding

After `FetchUserInfo`, `user.ID` was never validated. An empty ID could be stored in sessions and `player_characters.discord_user_id`.

## Fix Applied

Added early-return validation in `HandleCallback` (`internal/auth/oauth2.go`):

```go
if user.ID == "" {
    http.Error(w, "invalid user", http.StatusForbidden)
    return
}
```

## TDD Evidence

1. **Red:** Added `TestHandleCallback_EmptyUserID` — mocks `FetchUserInfo` returning `DiscordUser{ID: ""}`, asserts 403. Confirmed failure (got 307).
2. **Green:** Added the `user.ID == ""` check. Test passes.
3. **Verify:** `make test` passes. `make cover-check` passes (all thresholds met).

## Files Changed

- `internal/auth/oauth2.go` — added 4 lines (empty-ID guard)
- `internal/auth/oauth2_test.go` — added `TestHandleCallback_EmptyUserID`
