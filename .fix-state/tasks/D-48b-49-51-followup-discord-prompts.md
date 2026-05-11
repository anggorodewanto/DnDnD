---
id: D-48b-49-51-followup-discord-prompts
group: D
phase: 48b
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
parent: D-48b-stunning-strike-prompt, D-49-bardic-inspiration-prompt, D-51-divine-smite-prompt
---

# Discord-side post-hit prompt consumers

## Finding

Combat-side wiring for the three class-feature post-hit prompts now lands the
eligibility hints on `combat.AttackResult`:

- `PromptStunningStrikeEligible` + `PromptStunningStrikeKiAvailable` (D-48b)
- `PromptDivineSmiteEligible` + `PromptDivineSmiteSlots` (D-51)
- `PromptBardicInspirationEligible` + `PromptBardicInspirationDie` (D-49)

The Discord-side `ClassFeaturePromptPoster` (already wired in
`internal/discord/class_feature_prompt.go`) is NOT yet invoked from
`internal/discord/attack_handler.go`. The handler must read these fields
after `combat.AttackResult` returns, post the corresponding prompt, and
wire the OnChoice callback to `Service.StunningStrike` / `DivineSmite` /
`UseBardicInspiration`.

## Code paths cited
- `internal/combat/attack.go` — `AttackResult.PromptStunningStrikeEligible`, etc.
- `internal/combat/class_feature_prompt.go` — `Service.populatePostHitPrompts`
- `internal/discord/attack_handler.go` — missing post-hit dispatch
- `internal/discord/class_feature_prompt.go` — existing prompt poster

## Acceptance criteria (test-checkable)
- [ ] After a monk's `/attack` hits a creature, the player receives a Stunning Strike prompt with [Use Ki] / [Skip] buttons
- [ ] Accepting Stunning Strike calls `Service.StunningStrike`, deducts ki, and applies the stun condition on a failed save
- [ ] After a paladin's `/attack` hits in melee, the player receives a Divine Smite prompt with one button per available slot level
- [ ] After any attack roll by a holder with an active Bardic Inspiration die, the player receives a Bardic Inspiration prompt
- [ ] Accepting Bardic Inspiration calls `Service.UseBardicInspiration`, rolls the die, and clears the inspiration grant
- [ ] Forfeit (TTL expiry) consumes no resources

## Related
- D-48b-stunning-strike-prompt (service-side wiring — landed)
- D-49-bardic-inspiration-prompt (service-side wiring — landed)
- D-51-divine-smite-prompt (service-side wiring — landed)

## Notes
Discord channel for the prompt is the attacker's per-character DM (or the
combat-log channel for DM-controlled NPCs). `PromptDivineSmiteSlots` is
already sorted ascending so the button row reads naturally.
