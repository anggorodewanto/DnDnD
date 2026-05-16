# Worker Report: J-H05 Remediation

## Finding
**J-H05**: `GetActiveEncounterIDByCharacterID` uses `LIMIT 1`, silently masking the corrupt state where a character is in multiple active encounters.

## Fix Applied

### Approach
Added a secondary count check in the `discordUserEncounterResolver` adapter layer. After retrieving the single encounter ID (via the existing LIMIT 1 query), the resolver calls `CountActiveEncountersByCharacterID`. If count > 1, it returns `ErrAmbiguousEncounter` instead of silently picking one.

### Files Changed

1. **`cmd/dndnd/discord_adapters.go`**
   - Added `ErrAmbiguousEncounter` sentinel error.
   - Extended `resolverQueries` interface with `CountActiveEncountersByCharacterID`.
   - Added ambiguity check in `ActiveEncounterForUser` after the existing query returns.

2. **`internal/refdata/encounters_extra.go`** (new file)
   - Manual (non-sqlc-generated) `CountActiveEncountersByCharacterID` method on `*Queries`.
   - Runs `SELECT COUNT(*)` without `LIMIT 1` to detect duplicates.

3. **`cmd/dndnd/discord_adapters_test.go`**
   - Extended `fakeResolverQueries` with `activeEncCountByChar` field and `CountActiveEncountersByCharacterID` method.
   - Added `TestDiscordUserEncounterResolver_AmbiguousEncounter`: sets up a character in 2 active encounters and asserts `ErrAmbiguousEncounter` is returned.

### TDD Workflow
1. **Red**: Test written referencing `ErrAmbiguousEncounter` — build failed (undefined).
2. **Green**: Sentinel, interface method, count query, and adapter check added — test passes.
3. **Verify**: Full test suite passes (`go test ./...` excluding `internal/database`). Build compiles cleanly.

## Status: ✅ Complete
