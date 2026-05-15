finding_id: D-C04
status: done
files_changed:
  - internal/discord/save_handler.go
  - internal/discord/save_handler_test.go
test_command_that_validates: go test ./internal/discord/ -run TestSaveHandler_RagingCombatantGetsAdvantageOnSTRSave -v
acceptance_criterion_met: yes
notes: The EffectContext constructed in save_handler.go never set IsRaging, so the rage save-advantage effect (WhenRaging condition) was always filtered out. Fixed by calling lookupInvokerCombatant before building the EffectContext and copying comb.IsRaging into it. The fix degrades silently (IsRaging stays false) when no encounter/combatant is found, matching the SR-006 convention used throughout the handler. Test confirms a raging barbarian making a STR save now gets advantage via the FES.
follow_ups:
  - Consider also populating IsConcentrating in the EffectContext from the combatant state (same pattern, same location)
