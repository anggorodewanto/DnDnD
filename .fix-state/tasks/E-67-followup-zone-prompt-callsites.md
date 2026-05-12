---
id: E-67-followup-zone-prompt-callsites
group: E
phase: 67
severity: LOW
status: open
owner:
reviewer:
last_update: 2026-05-12
---

# Follow-up: invoke FormatZoneTriggerResults at done_handler and /move call sites

## Finding
DISCORD-POLISH bundle landed `FormatZoneTriggerResults` and `PostZoneTriggerResultsToCombatLog` helpers in `internal/discord/combat_log.go` with tests, but did not wire the call sites in `done_handler.go` or in the `/move` end-of-step flow because those files were outside the bundle's edit zone. Without the call-site wiring, zone trigger damage (Spirit Guardians etc.) doesn't surface in `#combat-log` even though the service-level `TurnInfo.ZoneTriggerResults` is populated.

## Code paths cited
- `internal/discord/combat_log.go` — `FormatZoneTriggerResults`, `PostZoneTriggerResultsToCombatLog` helpers (already landed).
- `internal/discord/done_handler.go` — turn-advance flow where `TurnInfo.ZoneTriggerResults` is available; should post the formatted results.
- `internal/discord/move_handler.go` — end-of-step flow where zone enter triggers fire; same posting hook.

## Acceptance criteria
- [ ] Done-handler turn-advance posts zone trigger results to `#combat-log` when `TurnInfo.ZoneTriggerResults` is non-empty.
- [ ] Move-handler end-of-step posts zone trigger results when the move triggered any zone effect.
- [ ] Test exercises Spirit Guardians: hostile moves into zone → damage logged in combat-log channel.
- [ ] `make test && make cover-check && make build` clean.

## Related / overlap risks
- Touches `done_handler.go` and `move_handler.go` — coordinate with any other move/done-handler tasks.

## Notes
Surfaced by DISCORD-POLISH reviewer; one-line call sites at known hook points.
