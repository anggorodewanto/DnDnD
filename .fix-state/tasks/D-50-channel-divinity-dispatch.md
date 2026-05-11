---
id: D-50-channel-divinity-dispatch
group: D
phase: 50
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Channel Divinity has zero Discord wiring

## Finding
No handler calls any of the Channel Divinity services (Turn Undead, Destroy Undead, Preserve Life, Sacred Weapon, Vow of Enmity). `/action channel-divinity` is documented in `help_content.go:142, 201, 219` but `action_handler.go` only routes `cancel` / `ready` / freeform; everything else falls through to the freeform-action path → DM queue. Players cannot invoke any Channel Divinity option from Discord.

## Code paths cited
- `internal/combat/channel_divinity.go` — 676 lines of service code (orphaned at dispatch layer)
- `internal/discord/action_handler.go` — does not switch on `channel-divinity` subcommand
- `internal/discord/help_content.go:142, 201, 219` — advertises commands that are unreachable

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 50)
- `docs/dnd-async-discord-spec.md` Channel Divinity section

## Acceptance criteria (test-checkable)
- [ ] `/action channel-divinity turn-undead` routes to `TurnUndead` service and applies the Turned condition on failed WIS saves
- [ ] `/action channel-divinity destroy-undead` enforces CR threshold by cleric level
- [ ] `/action channel-divinity preserve-life` validates half-max + 30ft range and distributes HP
- [ ] `/action channel-divinity sacred-weapon` (Devotion) wires correctly
- [ ] `/action channel-divinity vow-of-enmity` (Vengeance) wires correctly
- [ ] Test in `internal/discord/action_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Shares `action_handler.go` dispatch refactor with D-53 (Action Surge) and D-54 (Standard Actions) and D-57 (Hide).

## Notes
DM-queue routing (`ChannelDivinityDMQueue`) already implemented; the dispatch layer just needs to call into the service.
