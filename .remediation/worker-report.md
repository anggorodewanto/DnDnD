finding_id: B-C01
status: done
files_changed:
  - internal/dice/dice.go
  - internal/dice/dice_test.go
test_command_that_validates: go test ./internal/dice/ -run TestParseExpression_MultipleModifiers -v
acceptance_criterion_met: yes
notes: |
  The original modifier parsing stripped all `+` signs and called Atoi on the concatenated remainder, causing "5+5" to become "55". The fix replaces this with a `sumSignedTokens` helper that uses a regex to extract each `[+-]\d+` token and sums them. A `TrimLeft(residue, "+")` handles the case where dice groups separated by `+` leave behind consecutive `+` operators (e.g. `1d4+1d6+2` → `++2` after stripping). All existing tests continue to pass and coverage thresholds are met.
follow_ups: []

## Summary

Replaced the broken modifier parsing in `ParseExpression` (which concatenated digits after stripping `+` signs) with a proper token-by-token summation via a new `sumSignedTokens` helper. The helper uses a `[+-]\d+` regex to extract each signed integer from the post-dice residue and sums them. Added a table-driven test covering all four acceptance cases. All repo tests pass and coverage thresholds are met.
