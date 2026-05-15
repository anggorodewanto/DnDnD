finding_id: H-C01
status: done
files_changed:
  - internal/character/spellslots.go
  - internal/character/spellslots_test.go
test_command_that_validates: go test ./internal/character/ -run TestCalculateSpellSlots_SingleClassHalfCaster -v
acceptance_criterion_met: yes
notes: |
  The bug was in CalculateSpellSlots which used floor division (classLevel/2) for single-class half-casters, routing them through the multiclass table at the wrong caster level. The fix adds a special case for single-class half-casters that uses ceiling division ((classLevel+1)/2) instead. Tests cover Paladin 3 ({1:3}), Paladin 5 ({1:4,2:2}), and Paladin 9 ({1:4,2:3,3:2}). All existing tests continue to pass. Package coverage is 99.3%.
follow_ups:
  - Consider whether single-class third-casters (Eldritch Knight, Arcane Trickster) have the same floor-vs-ceil issue
  - Verify Ranger class also benefits from this fix (same "half" progression)
