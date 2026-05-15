finding_id: F-C02
status: done
files_changed:
  - internal/combat/turnresources.go
  - internal/combat/turnresources_test.go
test_command_that_validates: go test ./internal/combat/ -run "TestResolveTurnResources_HeavyArmorPenalty" -v
acceptance_criterion_met: yes
notes: Added heavy armor speed penalty application in ResolveTurnResources. After computing base speed and FES turn-start bonuses, the function now looks up the equipped armor via the store and calls CheckHeavyArmorPenalty to subtract 10ft when the character's STR is below the armor's strength_req. Two tests added: one for insufficient STR (expects 20ft) and one for sufficient STR (expects 30ft). Both `make test` and `make cover-check` pass.
follow_ups:
  - None identified; the fix is minimal and self-contained.
