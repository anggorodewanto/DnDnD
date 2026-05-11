---
id: E-67-followup-discord-zone-trigger-prompts
group: E
phase: 67
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
parent: E-67-zone-triggers
---

# Zone trigger results need DM-driven damage / save prompts

## Finding
After `E-67-zone-triggers` wiring landed, `Service.UpdateCombatantPositionWithTriggers` and `TurnInfo.ZoneTriggerResults` now expose `[]ZoneTriggerResult` records for combatants who entered a damaging zone or started their turn in one. The combat-side helpers do NOT yet apply damage or enqueue saves — each `ZoneTrigger` definition only carries `Effect="damage"` / `"save"` with no Details payload, so the actual damage roll / DC lookup must happen in the consumer.

This task tracks the Discord / dashboard wiring that:
- consumes `TurnInfo.ZoneTriggerResults` after `AdvanceTurn` to post the start-of-turn save / damage prompts to the DM queue;
- consumes the trigger slice returned by `UpdateCombatantPositionWithTriggers` after `/move` to post the entry-trigger prompts;
- resolves the source spell's `Damage` / `SaveAbility` / `SaveEffect` to construct the prompt body.

## Code paths cited
- internal/combat/service.go — `UpdateCombatantPositionWithTriggers` returns trigger results
- internal/combat/initiative.go — `TurnInfo.ZoneTriggerResults` populated in `createActiveTurn`
- internal/combat/zone.go — `ZoneTriggerResult` shape
- cmd/dndnd/discord_handlers.go — `/move` handler (call site for entry-trigger prompts)
- internal/discord — turn-start handler (call site for start-of-turn prompts)

## Acceptance criteria (test-checkable)
- [ ] After `/move` into Spirit Guardians area, the affected hostile receives a Wisdom save prompt; on failure they take 3d8 radiant damage
- [ ] At start-of-turn inside Wall of Fire, the combatant takes the fire-damage roll
- [ ] Per-round dedupe still works (one trigger per zone per combatant per round)

## Related
- E-67-zone-triggers (parent — wiring landed)
- E-66b-cast-extended-flag (damage application)

## Notes
The trigger-source spell can be looked up via SourceSpell from the result and `store.GetSpell(...)` to pull Damage/SaveAbility for the prompt.
