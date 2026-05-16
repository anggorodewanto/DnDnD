# Worker Report: I-H07

## Finding

Narration and message-player handlers trusted `author_user_id` from the JSON request body, allowing a DM to impersonate another user.

## Fix Applied

Both handlers now ignore the body's `author_user_id` and instead use the authenticated user from the request context via `auth.DiscordUserIDFromContext(r.Context())`.

### Files Modified

- `internal/narration/handler.go` — `Post` handler now extracts user ID from context
- `internal/narration/handler_test.go` — Added `TestHandler_Post_UsesContextUserIDNotBody`; updated existing tests to set context user
- `internal/messageplayer/handler.go` — `Send` handler now extracts user ID from context
- `internal/messageplayer/handler_test.go` — Added `TestHandler_Send_UsesContextUserIDNotBody`; updated existing tests to set context user

## TDD Evidence

1. **Red:** Both new tests failed with `expected author_user_id='real-user', got "fake-user"`
2. **Green:** After overriding body field with context user, both new tests pass
3. **Verify:** `make test` passes (all packages), `make cover-check` passes (narration 94.68%, messageplayer 97.69%)

## Acceptance Criterion

✅ The stored `author_user_id` always matches the authenticated user, regardless of what the request body says. Tests `TestHandler_Post_UsesContextUserIDNotBody` and `TestHandler_Send_UsesContextUserIDNotBody` demonstrate this by sending `author_user_id="fake-user"` in the body while authenticated as `"real-user"`, and asserting the stored record uses `"real-user"`.
