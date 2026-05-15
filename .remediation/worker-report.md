finding_id: C-C02
status: done
files_changed:
  - internal/combat/advantage.go
  - internal/combat/advantage_test.go
  - internal/combat/attack.go
test_command_that_validates: go test ./internal/combat/ -run "TestDetectAdvantage_RecklessCondition" -v
acceptance_criterion_met: yes
notes: Added `AbilityUsed` field to `AdvantageInput` struct and a `"reckless"` case in the attacker-conditions loop of `DetectAdvantage` that grants advantage only when the weapon is melee and the ability used is STR. Passed `AbilityUsed` through from `ResolveAttack`. Three tests added: melee STR gets advantage (positive), ranged does not (negative), melee DEX does not (negative). All existing tests pass, coverage thresholds met.
follow_ups:
  - Verify that Service.Attack populates `AbilityUsed` on `AttackInput` when building from `AttackCommand` (integration-level confirmation).
