---
id: D-54-followup-stand-max-speed
group: D
phase: 54
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: /action stand uses hardcoded MaxSpeed=30

## Finding
D-54 main implementation wired `/action stand` (StandUp). `dispatchStand` in `internal/discord/action_handler.go` hardcodes `MaxSpeed: 30`, which is correct only for the default 30ft race. Halfling (25), Tabaxi (35), Dwarf (25), etc. require their actual speed for the half-movement-cost-to-stand rule.

## Code paths cited
- `internal/discord/action_handler.go` `dispatchStand` — hardcoded `MaxSpeed: 30`.
- `internal/discord/move_handler.go` `MoveSizeSpeedLookup` — existing speed lookup mechanism that `action_handler.go` could reuse.

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md (Phase 54 StandUp / movement rules)

## Acceptance criteria (test-checkable)
- [ ] `ActionHandler` accepts an injected speed-lookup interface (same shape as `MoveSizeSpeedLookup`)
- [ ] `dispatchStand` resolves the combatant's actual max speed from race / conditions and passes that to the StandUp service
- [ ] Test in `internal/discord/action_handler_dispatch_test.go` covers a Halfling (25ft) and confirms the half-movement-to-stand cost reflects 12 (or 13) feet, not 15
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- `internal/discord/action_handler.go` central — coordinate with other action-handler tasks.

## Notes
Surfaced by DISPATCH-D implementer + reviewer; both flagged as a known limitation of the main impl.
