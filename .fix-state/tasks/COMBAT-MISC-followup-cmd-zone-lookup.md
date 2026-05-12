---
id: COMBAT-MISC-followup-cmd-zone-lookup
group: E
phase: 69
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-12
---

# Follow-up: wire SetZoneLookup on check + action handlers in cmd/dndnd

## Finding
COMBAT-MISC batch added `CheckHandler.SetZoneLookup` and `ActionHandler.SetZoneLookup` setters so obscurement gates fire from zones. Without production wiring in `cmd/dndnd/discord_handlers.go`, the gates are silent (no regression — degrades to current behavior without zone awareness).

`*combat.Service` already satisfies both interfaces structurally via `ListZonesForEncounter`.

## Code paths cited
- `internal/discord/check_handler.go` — `CheckZoneLookup` interface, `SetZoneLookup` setter.
- `internal/discord/action_handler.go` — `ActionZoneLookup` interface, `SetZoneLookup` setter.
- `cmd/dndnd/discord_handlers.go` — production handler construction site.

## Acceptance criteria
- [ ] `cmd/dndnd/discord_handlers.go` calls `handlers.check.SetZoneLookup(combatSvc)` and `handlers.action.SetZoneLookup(combatSvc)` (or via deps.combatService).
- [ ] Test confirms obscurement reason surfaces in /check perception inside a heavily-obscured zone.
- [ ] Test confirms /action hide is blocked outside obscured zones and allowed inside.
- [ ] `make test && make cover-check && make build` clean.

## Related / overlap risks
- Coordinate with the next cmd/dndnd wiring batch (likely small additions only).

## Notes
Surfaced by COMBAT-MISC implementer; one-line setters mirroring existing pattern.
