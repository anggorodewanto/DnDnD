# Worker Report: H-H01

## Finding
ASI handlers (`HandleASIChoice`, `HandleASISelect`, `HandleASIFeatSelect`, `HandleASIFeatSubChoiceSelect`) did not verify that the interacting Discord user owns the character. Any guild member could press ASI buttons/menus for another player's character.

## Fix Applied

Added `validateASIOwner(interaction, charData) error` helper to `ASIHandler` in `internal/discord/asi_handler.go`. It compares `discordUserID(interaction)` with `charData.DiscordUserID` and responds with an ephemeral "⛔ This is not your character." message on mismatch.

Called at the top of all four handlers, immediately after `charData` is loaded:
- `HandleASIChoice` (line ~380)
- `HandleASISelect` (line ~425)
- `HandleASIFeatSelect` (line ~680)
- `HandleASIFeatSubChoiceSelect` (line ~770)

## Tests Added

Four new tests in `internal/discord/asi_handler_test.go`:
- `TestASIHandler_HandleASIChoice_RejectsNonOwner`
- `TestASIHandler_HandleASISelect_RejectsNonOwner`
- `TestASIHandler_HandleASIFeatSelect_RejectsNonOwner`
- `TestASIHandler_HandleASIFeatSubChoiceSelect_RejectsNonOwner`

Each creates an interaction from `"other-user"` on a character owned by `"owner-user"` and asserts the response contains "not your character".

Three existing tests were updated to include `DiscordUserID` in their mock character data (they previously omitted it, which now correctly triggers the rejection).

## Verification
- `make test` — PASS
- `make cover-check` — PASS (all thresholds met)
