---
id: C-33-followup-discord-walls
group: C
phase: 33
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: Discord attack handler must populate Walls for cover calc

## Finding
C-33 service-level wiring is complete: `Service.Attack` / `Service.OffhandAttack` / `Service.attackImprovised` now compute attacker→target cover when `AttackCommand.Walls` is non-empty. The Discord attack handler currently passes `nil` Walls, so cover never applies through the slash-command pipeline. Mirror `cast_handler.loadWalls` pattern.

## Code paths cited
- `internal/discord/attack_handler.go` — `AttackCommand` construction site, needs `Walls` populated.
- `internal/discord/cast_handler.go` — existing `loadWalls` helper to mirror.
- `internal/combat/attack.go` — service-level consumer of `AttackCommand.Walls` (already wired by C-33 main impl).

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md (Phase 33 cover section)

## Acceptance criteria (test-checkable)
- [ ] `/attack` slash command populates `AttackCommand.Walls` from current map state
- [ ] Slash-command-driven attack against a target with half cover yields the cover bonus
- [ ] Test in `internal/discord/attack_handler_test.go` exercises a covered target
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- `internal/discord/attack_handler.go` is the central hotspot — schedule separately from any other discord attack-handler tasks.

## Notes
Surfaced as a deferral by the C-33 implementer (`internal/combat/` file zone forbade discord/ edits).
