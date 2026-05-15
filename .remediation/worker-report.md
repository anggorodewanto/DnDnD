# Worker Report: A-H08

## Finding
Multiple fuzzy matches wrapped in a single `**...**` instead of each being individually bolded. Also used literal `<name>` placeholder instead of the actual suggestion.

## Fix Applied

**File:** `internal/discord/registration_handler.go` (line 97–100)

**Before:**
```go
suggestions := strings.Join(result.Suggestions, ", ")
respondEphemeral(h.session, interaction,
    fmt.Sprintf("❌ No character named \"%s\" found. Did you mean: **%s**? Use /register <name> to confirm.", characterName, suggestions))
```

**After:**
```go
bolded := make([]string, len(result.Suggestions))
for i, s := range result.Suggestions {
    bolded[i] = "**" + s + "**"
}
respondEphemeral(h.session, interaction,
    fmt.Sprintf("❌ No character named \"%s\" found. Did you mean: %s? Use /register %s to confirm.", characterName, strings.Join(bolded, ", "), result.Suggestions[0]))
```

## Tests Added

**File:** `internal/discord/registration_handler_test.go`

1. `TestRegisterHandler_FuzzyMatch_MultipleSuggestions_BoldsEachName` — asserts 3 matches render as `**Thorn**, **Thorin**, **Thora**` and first name is used in the `/register` hint.
2. `TestRegisterHandler_FuzzyMatch_SingleSuggestion_BoldsName` — asserts single match is bolded and actual name replaces `<name>`.

## Verification

- `make test` — PASS (all tests green)
- `make cover-check` — PASS (all thresholds met)
