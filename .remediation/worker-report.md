# Worker Report: E-H01

**Finding:** Help action grants advantage only on attacks, not on ability checks  
**Status:** ✅ Fixed  

## Changes

### 1. `internal/combat/standard_actions.go`
- Added `github.com/google/uuid` import.
- When `cmd.Target.ID == uuid.Nil` (no enemy target), the Help action now applies a `help_check_advantage` condition on the ally instead of `help_advantage` scoped to an enemy.
- Adjacency check is skipped when no target is specified.
- Combat log message adapts to the no-target case.

### 2. `internal/combat/standard_actions_test.go`
- Added `TestHelp_NoTarget_AppliesCheckAdvantageOnAlly` — verifies that Help with a zero-value Target applies `help_check_advantage` on the ally, consumes the action, and does NOT apply `help_advantage`.

## Test Results

- New test: PASS
- Full suite (excluding `internal/database`): ALL PASS (38 packages)
