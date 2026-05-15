finding_id: C-C03
status: done
files_changed:
  - internal/combat/attack.go
  - internal/combat/attack_test.go
test_command_that_validates: go test ./internal/combat/ -run "TestServiceOffhandAttack_NoAttackMadeThisTurn|TestServiceOffhandAttack_RangedMainHand|TestServiceOffhandAttack_RangedOffHand" -v
acceptance_criterion_met: yes
notes: Added two validation checks to OffhandAttack: (1) rejects if no attack has been made this turn by comparing AttacksRemaining against resolveAttacksPerAction max, and (2) rejects if either the main-hand or off-hand weapon is ranged via IsRangedWeapon. Three new tests were written (red-first TDD), and two existing happy-path tests were updated to set AttacksRemaining: 0 to reflect that an attack was already made. All tests pass including make test and make cover-check.
follow_ups:
  - Consider whether thrown melee weapons (e.g., dagger with "thrown" property) should be allowed for off-hand TWF when used in melee range — current implementation allows them since WeaponType is "simple_melee".
