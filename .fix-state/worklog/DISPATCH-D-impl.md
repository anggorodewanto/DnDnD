# DISPATCH-D worklog (clean run, 2026-05-11)

Single-agent session, no concurrent edits, no stash/restore activity.

## Per-task status

| Task ID | Status | Notes |
|---|---|---|
| D-47-wild-shape-dispatch | DONE | `/bonus wild-shape <beast>` → `ActivateWildShape`; `/bonus revert-wild-shape` → `RevertWildShapeService`. |
| D-48b-flurry-of-blows | DONE | `/bonus flurry <target>` (alias `flurry-of-blows`) → `FlurryOfBlows`; Attack-action precondition enforced by service. |
| D-50-channel-divinity-dispatch | DONE | `/action channel-divinity <option>` routes to `TurnUndead` / `PreserveLife` / `SacredWeapon` / `VowOfEnmity` and falls back to `ChannelDivinityDMQueue` for unknown options. |
| D-52-lay-on-hands-action-vs-bonus | DONE | `/action lay-on-hands` is now wired (D-52 ACs satisfied). `/bonus lay-on-hands` retained as a deprecated alias so existing macros keep working — service validates `ResourceAction` either way. |
| D-53-action-surge | DONE | `/action surge` → `ActionSurge`. |
| D-54-standard-actions-wiring | DONE | `/action dash`, `disengage`, `dodge`, `help`, `stand`, `drop-prone`, `escape` all wired. `Stand`'s `MaxSpeed` is hardcoded to 30 (per default in the service); a dedicated PC-speed lookup adapter is out of zone (see clarification below). |
| D-54-cunning-action | DONE | `/bonus cunning-action <dash|disengage|hide>` → `CunningAction`. Rogue level-2 gate enforced by service. |
| D-56-drag-release | PARTIAL | `/bonus drag` (informational prompt), `/bonus release-drag`, AND `/move` drag-cost integration (x2 via `combat.DragMovementCost` when the mover is currently grappling someone). Grappled-target tile sync along the path is **NOT** implemented — would require encoding the dragged-target IDs into the `move_confirm:` button custom-ID and is a larger refactor than the dispatch fix scope. Documented as a follow-up. |
| D-57-hide-action | DONE | `/action hide` → `Hide`. Hostiles list filtered to opposite faction via `filterHostiles`. |
| D-57-cunning-action-hide | DONE (covered by D-54-cunning-action) | `/bonus cunning-action hide` routes through the same `CunningAction` service in hide mode. |

## Interface / adapter changes

- `ActionCombatService` (in `internal/discord/action_handler.go`) extended with 15 new methods.
- `BonusCombatService` (in `internal/discord/bonus_handler.go`) extended with 6 new methods.
- `MoveHandler` gained `MoveDragLookup` interface + `SetDragLookup` setter.
- `cmd/dndnd/discord_handlers.go` `actionCombatServiceAdapter` extended with forwarding methods for each new ActionCombatService entry.
- `BonusCombatService` adapter unchanged — `*combat.Service` structurally satisfies the new method set already (the BonusHandler wires `deps.combatService` directly).
- `MoveHandler.SetDragLookup(deps.combatService)` wired in `buildDiscordHandlers`.

## New tests

- `internal/discord/action_handler_dispatch_test.go` — 19 tests covering surge / dash / disengage / dodge / help (with single-arg defaulting) / hide (with/without roller) / stand / drop-prone / escape (grappled and not-grappled paths) / channel-divinity (turn-undead, sacred-weapon, vow-of-enmity, preserve-life, DM-queue fallback, missing-option) / lay-on-hands (happy + bad-args) / unknown-subcommand-falls-through-to-freeform / combat-log mirror / wrong-owner rejection.
- `internal/discord/bonus_handler_dispatch_test.go` — 13 tests covering wild-shape (happy + missing-beast) / revert / flurry (happy + alias + missing-target) / cunning-action (dash + disengage + hide + bad-mode) / drag (no-targets + with-targets) / release-drag (happy + nothing-to-release).
- `internal/discord/move_handler_drag_test.go` — 2 tests: drag prompt + doubled cost, vs no-targets baseline.

Mock plumbing added to `fakeActionCombatService` and `mockBonusCombatService` to implement the new interface methods.

## Build / test / coverage

- `go build ./...` → clean.
- `go test ./internal/discord/...` → all pass.
- `make test` → all pass.
- `make cover-check` → `OK: coverage thresholds met` (discord pkg at 86.99%, all per-package thresholds satisfied).
- `make build` → `bin/dndnd` and `bin/playtest-player` built.

## Clarifications / deferred work

- **`/action stand` PC-speed lookup**: the dispatcher passes `MaxSpeed: 30` to `Stand`. Halflings and Tabaxi will use the wrong cost when stand cost is half-of-walk-speed. The existing `MoveSizeSpeedLookup` lives on `MoveHandler` and reusing it would require a small adapter on `ActionHandler`. Filed as a follow-up — not a regression vs HEAD because `/action stand` previously routed to freeform / DM-queue.
- **`/move` grappled-target tile sync**: when a grappler moves, the spec says the grappled target's tile should also move along the path. Implementing this needs the dragged-target combatant IDs and per-tile updates threaded through the existing `move_confirm:` button parser. The current implementation correctly doubles the movement cost and surfaces the drag prompt; the target stays at their tile until released. Filed as a follow-up.
