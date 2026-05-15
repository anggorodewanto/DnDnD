# Worker Report: C-H04

## Finding
Dash (and CunningAction Dash) used raw base speed from `resolveBaseSpeed`, ignoring exhaustion level.

## Fix Applied
After resolving base speed, applied `ExhaustionEffectiveSpeed(speed, exhaustionLevel)` to halve the dash bonus at exhaustion ≥ 2 and zero it at exhaustion ≥ 5.

### Files Modified
- `internal/combat/standard_actions.go` — Added exhaustion application after `resolveBaseSpeed` in both `Dash` (line ~53) and `CunningAction` dash case (line ~895).
- `internal/combat/standard_actions_test.go` — Added `TestDash_ExhaustionLevel2_HalvesDashBonus`.

## TDD Cycle
1. **Red:** `TestDash_ExhaustionLevel2_HalvesDashBonus` — PC with exhaustion 2 and speed 30 dashes; expected +15, got +30. FAIL.
2. **Green:** Added `speed = int32(ExhaustionEffectiveSpeed(int(speed), int(cmd.Combatant.ExhaustionLevel)))` after `resolveBaseSpeed` in both Dash paths. PASS.
3. **Verify:** `make test` ✅ | `make cover-check` ✅

## Acceptance Criterion
A Dash for an exhaustion-2 character (speed halved) adds half the base speed (15), not full (30). Test demonstrates this.
