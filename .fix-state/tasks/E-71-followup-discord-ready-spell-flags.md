---
id: E-71-followup-discord-ready-spell-flags
group: E
phase: 71
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
parent: E-71-readied-action-expiry
---

# /action ready needs spell-name / slot-level surface in Discord

## Finding
`ReadyActionCommand` carries `SpellName` and `SpellSlotLevel`, and the service-side `ReadyAction` correctly:
- expends the spell slot at ready-time;
- sets concentration on the readied spell;
- surfaces the spell name + slot level in `FormatReadiedActionsStatus`;
- clears concentration when `ExpireReadiedActions` cancels a readied-spell.

But `internal/discord/action_handler.go:performReadyAction` never reads these fields from the slash-command interaction — it only forwards `Description`. As a result the readied-spell-with-slot path is unreachable via Discord today (only via dashboard / direct service calls).

This task tracks the Discord-side surface that:
- parses optional `spell:` and `slot:` slash-command options from `/action ready`;
- threads them into `ReadyActionCommand.SpellName` / `SpellSlotLevel`;
- updates the slash-command definition file to register those options.

## Code paths cited
- internal/discord/action_handler.go:268 — `performReadyAction`
- internal/discord/slash_commands.go — `/action` option registration
- internal/combat/readied_action.go — `ReadyActionCommand` fields

## Related
- E-71-readied-action-expiry (parent — service side complete)

## Notes
Out of scope of the batch-2 implementer (discord package was carved out).
