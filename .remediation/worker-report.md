finding_id: E-C02
status: done
files_changed:
  - internal/combat/aoe.go
  - internal/combat/aoe_test.go
test_command_that_validates: go test ./internal/combat/ -run TestResolveAoEPendingSaves_UpcastScalesDamageDice -count=1 -v
acceptance_criterion_met: yes
notes: |
  The bug was that ResolveAoEPendingSaves read dmgInfo.Dice verbatim without calling ScaleSpellDice.
  The fix encodes effectiveSlotLevel and charLevel into the pending_saves source tag (format: "aoe:<spell-id>:s<slot>c<charLevel>[:e<N>]")
  at CastAoE time, then extracts them in ResolveAoEPendingSaves to call ScaleSpellDice before passing dice to the damage pipeline.
  Legacy source tags (without :s<N>c<N>) gracefully fall back to base dice via the slotLevel=0 path in ScaleSpellDice.
  The cantrip scaling half of the acceptance criterion (Thunderclap at char level 5 → 2d6) is also fixed by this same mechanism since charLevel is now encoded.
follow_ups:
  - Cantrip AoE scaling test (e.g. Thunderclap at level 5 → 2d6) could be added for completeness
  - Existing integration tests that use the old source format ("aoe:<spell-id>") still pass via backward-compat path
