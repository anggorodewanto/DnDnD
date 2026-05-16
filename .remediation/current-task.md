finding_id: G-H09
severity: High
title: Encounter-active check on rest can be bypassed for party rest
location: internal/discord/rest_handler.go:159-164
spec_ref: spec §Rest Constraints line 2630 (Phase 83a)
problem: |
  Individual /rest calls ActiveEncounterForUser and rejects if the caller is a combatant. But the rest is still permitted for users not registered as combatants in an active encounter. A bystander could /rest long while their party is mid-fight.
suggested_fix: |
  Use PartyEncounterChecker.HasActiveEncounter in the individual handler too so any active encounter in the campaign blocks rests.
acceptance_criterion: |
  /rest is rejected when any active encounter exists in the campaign, not just when the caller is a combatant. A test demonstrates this.
