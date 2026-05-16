# Worker Report: J-H06

**Worker:** worker-J-H06
**Finding:** /whisper accepts empty message and spams a dm-queue item
**Status:** ✅ Fixed

## Changes

### `internal/discord/whisper_handler.go`
- Added `"strings"` import.
- Added empty-message validation after `optionString(interaction, "message")`:
  ```go
  if strings.TrimSpace(message) == "" {
      respondEphemeral(h.session, interaction, "Please provide a message.")
      return
  }
  ```

### `internal/discord/whisper_handler_test.go`
- Added `TestWhisperHandler_EmptyMessage` with subtests for empty string and whitespace-only input.

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
