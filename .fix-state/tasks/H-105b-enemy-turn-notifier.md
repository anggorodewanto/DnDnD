---
id: H-105b-enemy-turn-notifier
group: H
phase: 105b
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# enemyTurnNotifier constructed but never injected into combat.Handler

## Finding
`enemyTurnNotifier` is constructed in `buildDiscordHandlers` and `SetEncounterLookup` is called on it, but it is never injected into the `combat.Handler` — there is no `combatHandler.SetEnemyTurnNotifier(...)` call anywhere in `cmd/dndnd/main.go`. As a result, `combat.Handler.ExecuteEnemyTurn` falls through the `if h.enemyTurnNotifier != nil` branch as a silent no-op in production. The Phase 105 "⚔️ <display_name> — Round N" enemy-turn label is dead at runtime, which is exactly the failure mode Phase 105b was created to fix.

## Code paths cited
- `cmd/dndnd/discord_handlers.go:191` — `enemyTurnNotifier` constructed
- `cmd/dndnd/discord_handlers.go:243-245` — `SetEncounterLookup` called on it
- `cmd/dndnd/main.go:856-973` — router wiring, missing `combatHandler.SetEnemyTurnNotifier(...)`
- `internal/combat/handler.go` — `ExecuteEnemyTurn` guarded by `if h.enemyTurnNotifier != nil`

## Spec / phase-doc anchors
- `docs/phases.md:632-829` (Group H, Phase 105b)
- Phase 105b scope: "Discord Handler Wiring in main.go" — the explicit reason Phase 105b exists

## Acceptance criteria (test-checkable)
- [ ] `combat.Handler` exposes a setter (e.g. `SetEnemyTurnNotifier`) and `cmd/dndnd/main.go` calls it with the `enemyTurnNotifier` from `buildDiscordHandlers`
- [ ] `combat.Handler.ExecuteEnemyTurn` invokes the notifier and posts the "⚔️ <display_name> — Round N" message
- [ ] Test in `internal/combat/handler_test.go` (or `cmd/dndnd/discord_handlers_test.go` / `dmqueue_wiring_test.go` analog) fails before the fix and passes after, asserting the notifier is non-nil after wiring and that `ExecuteEnemyTurn` calls it
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Touches `cmd/dndnd/main.go` shared with `H-104b-rest-magicitem-publisher`. No semantic overlap.

## Notes
Audit calls this the literal failure mode Phase 105b was created to fix, so the wiring oversight is severity CRITICAL. Verify the setter name matches the existing pattern (`SetNotifier`, `SetPublisher`, etc.).
