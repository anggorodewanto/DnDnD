---
id: C-DISCORD-followup-cmd-wire-setters
group: C
phase: 33,43
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: wire C-DISCORD setters in cmd/dndnd

## Finding
C-DISCORD bundle added two new optional setters on Discord handlers that need production wiring in `cmd/dndnd/discord_handlers.go`:

1. `AttackHandler.SetMapProvider(...)` — required for `/attack` to compute cover via `Walls`. Without it, attacks run without wall-cover (cover gate from batch 1 is unreachable from Discord).
2. `ActionHandler.SetStabilizeStore(...)` — required for `/action stabilize` to persist the death-save reset. Without it the command reports "not available".

## Code paths cited
- `internal/discord/attack_handler.go` — `SetMapProvider` setter and `loadWalls` helper.
- `internal/discord/action_handler.go` — `SetStabilizeStore` setter.
- `cmd/dndnd/discord_handlers.go` — production handler construction site that needs the two setter calls (similar to `SetRoller`, `SetChannelIDProvider` wiring done in batch 1).

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md Phase 33 (cover) + Phase 43 (stabilize)

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/discord_handlers.go` calls both `SetMapProvider` and `SetStabilizeStore` with concrete implementations
- [ ] e2e test in `cmd/dndnd/` confirms `/attack` against a covered target uses the wall data
- [ ] e2e test confirms `/action stabilize` persists death-save success ticks
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- `cmd/dndnd/discord_handlers.go` — high-traffic file; coordinate with other cmd/ tasks.

## Notes
Surfaced by C-DISCORD implementer; out of zone for that bundle. Small wiring task.
