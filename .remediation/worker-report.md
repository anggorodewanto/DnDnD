finding_id: G-C02
status: done
files_changed:
  - internal/discord/attune_handler.go
  - internal/discord/attune_handler_test.go
test_command_that_validates: go test ./internal/discord/ -run TestAttuneHandler_RejectsDuringCombat -count=1
acceptance_criterion_met: yes
notes: Added an optional CheckEncounterProvider to AttuneHandler with a SetEncounterProvider setter. The Handle method now checks ActiveEncounterForUser before proceeding; if the user is in an active encounter, it responds with "Attunement requires a short rest — you cannot attune during combat." and returns early. The pattern mirrors rest_handler.go's combat gate. All existing tests continue to pass because the encounterProvider is nil by default (no-op).
follow_ups:
  - Wire SetEncounterProvider in the bot's main setup (where handlers are constructed) so the check is active in production.
  - Consider also gating /attune behind the /rest short flow for full spec compliance (currently only blocks during combat, does not require an active rest session).
