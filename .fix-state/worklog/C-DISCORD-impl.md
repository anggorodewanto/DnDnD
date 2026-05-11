# C-DISCORD bundle — implementer worklog

Implementer: Claude Opus 4.7 (1M context).
Working directory: /home/ab/projects/DnDnD.

Scope: Group C discord-side wiring (6 task IDs).

## Per-task status

### C-30-occupant-size — DONE
- `internal/discord/move_handler.go`: `buildOccupants` now takes a
  `sizeFn func(refdata.Combatant) int` callback that resolves each
  occupant's size category. New helper `occupantSizeFn(ctx)` returns a
  closure over the wired `sizeSpeedLookup`; nil-safe.
- All five call sites of `buildOccupants` updated (combat /move,
  exploration /move, prone Stand & Move, prone Crawl, `buildGridForTurn`).
- Tests:
  - `TestBuildOccupants_UsesSizeLookup_PerCombatant` (Tiny + Large pass-through).
  - `TestBuildOccupants_NilSizeFn_FallsBackToMedium`.
  - `TestMoveHandler_BuildOccupants_RoutesThroughWiredSizeLookup`.

### C-32-range-rejection-format — DONE
- `internal/discord/attack_handler.go`: new `rangeRejectionMessage(err)`
  parser + `formatAttackError` / `formatOffhandAttackError` wrappers route
  the service-level `"out of range: Xft away (max Yft)"` sentinel through
  `combat.FormatRangeRejection`. Non-range errors keep the legacy
  `"Attack failed: <err>"` wording.
- The helper is now production-callable; the previous lone caller was its
  own unit test.
- Tests:
  - `TestAttackHandler_OutOfRange_UsesFormatRangeRejection`.
  - `TestAttackHandler_OffhandOutOfRange_UsesFormatRangeRejection`.
  - `TestAttackHandler_OtherErrors_KeepLegacyWording`.
  - `TestRangeRejectionMessage_ParsesAttackError` / `_NonRangeError`.

### C-33-followup-discord-walls — DONE
- `internal/discord/attack_handler.go`: new `AttackMapProvider` interface
  + `SetMapProvider` setter + `loadWalls` helper mirroring
  `cast_handler.loadWalls`. `AttackCommand.Walls` (and
  `OffhandAttackCommand.Walls`) populated when a provider is wired.
- Production wiring (`cmd/dndnd/discord_handlers.go`) is OUT OF SCOPE for
  this implementer per the bundle's file zone. See "Wiring follow-ups".
- Tests:
  - `TestAttackHandler_PopulatesWallsFromMap`.
  - `TestAttackHandler_NoMapProvider_WallsRemainNil`.
  - `TestAttackHandler_OffhandPopulatesWallsFromMap`.

### C-40-frightened-move — DONE
- `internal/discord/move_handler.go`: new `rejectFrightenedTowardSource`
  helper builds the `fearSources` map from all alive combatants and calls
  `combat.ValidateFrightenedMovement`. Invoked at validation time in the
  combat-mode /move path so the user gets a clean rejection rather than a
  consumed movement budget.
- Tests:
  - `TestMoveHandler_Frightened_BlocksApproachToSource`.
  - `TestMoveHandler_Frightened_AllowsMoveAwayFromSource`.
  - `TestMoveHandler_NotFrightened_NoRejection`.

### C-43-stabilize — DONE
- `internal/discord/action_handler.go`: new `ActionStabilizeStore`
  interface + `SetStabilizeStore` setter + `dispatchStabilize` function.
  Wired as a new dispatch branch (`case "stabilize"`).
- DC 10 Medicine check (flat d20 via wired roller). On success calls the
  existing pure `combat.StabilizeTarget` helper and persists the resulting
  `DeathSaves{Successes: 3}` via the store's `UpdateCombatantDeathSaves`.
- Validates target is within 5ft (Chebyshev grid distance) and currently
  dying via `combat.IsDying`. Out-of-reach / not-dying targets are
  rejected with explicit messages.
- Production wiring not in scope. See "Wiring follow-ups".
- Tests:
  - `TestActionHandler_Stabilize_SuccessPersistsThreeSuccesses`.
  - `TestActionHandler_Stabilize_FailureDoesNotPersist`.
  - `TestActionHandler_Stabilize_TargetNotDying_Rejected`.
  - `TestActionHandler_Stabilize_OutOfReach_Rejected`.
  - `TestActionHandler_Stabilize_NoStore_ReportsUnavailable`.

### C-43-block-commands — DONE
- `internal/discord/commands.go`: new shared `incapacitatedRejection(c)`
  guard + `dyingRejection(c)` helper. Returns a specific
  `"You are dying — only \`/deathsave\` is available until you stabilize."`
  message for combatants making death saves and a generic
  `"You are incapacitated and cannot take that action."` for stunned /
  paralyzed / unconscious / petrified combatants.
- Wired into `/move`, `/attack`, `/action` (combat-path freeform + dispatch
  subcommands), and `/bonus` (via `resolveContext`). `/deathsave` and
  off-turn / DM-side commands are intentionally untouched.
- Tests:
  - `TestMoveHandler_DyingCombatant_Blocked`.
  - `TestMoveHandler_UnconsciousCombatant_Blocked`.
  - `TestAttackHandler_DyingCombatant_Blocked`.
  - `TestActionHandler_DyingCombatant_BlocksDispatch`.

## Wiring follow-ups (out of this bundle's file zone)

Production wiring for the two new optional setters needs an edit in
`cmd/dndnd/discord_handlers.go`. The combat / discord package tests pass
without it because both setters are nil-safe; without the wiring the
production paths simply degrade:

- `AttackHandler.SetMapProvider`: without wiring, /attack runs as before
  (no wall-based cover). Should be wired to the existing
  `castLookupAdapter`-style map adapter so /attack and /cast share map
  reads.
- `ActionHandler.SetStabilizeStore`: without wiring, `/action stabilize`
  rejects with "Stabilize is not available right now (no persistence
  wired)." Should be wired to `deps.queries` (the same store
  `DeathSaveHandler` uses).

A follow-up task file should be added to `.fix-state/tasks/` to land
those two `cmd/dndnd/discord_handlers.go` edits.

## Test-fixture migration

The new dying / incapacitated guard is sensitive to the
`HpCurrent == 0 && IsAlive == true` combination (the canonical "dying"
state). Several pre-existing test fixtures relied on Go's zero-value
defaults and so were implicitly "dying" once the guard landed. A bulk
sed-style edit added `HpCurrent: 10` to every `IsAlive: true` Combatant
literal in `internal/discord/*_test.go` so existing tests continue to
exercise the happy path. Three multi-line literals that already declared
`HpCurrent` separately were corrected by hand to drop the duplicate.

## Gates

- `make build` — clean.
- `make cover-check` — coverage thresholds met (discord 86.96% overall;
  per-package thresholds unchanged).
- `go test ./...` — all packages green.

## Risks / notes

- The dying-state guard uses `combat.ParseDeathSaves` + `combat.IsDying`.
  If a future schema migration changes the death-save JSON shape, the
  guard silently degrades to "not dying" (returns the legitimate error
  but doesn't block) — this is the safest fail-open mode.
- The Stabilize dispatch uses a flat d20 (no Medicine modifier) because
  `ActionCombatService` doesn't expose the actor's ability scores. A
  future task should plumb the actor's WIS modifier through the service
  surface so the DC 10 check matches the spec exactly.
- The C-30 fix preserves the historical Medium fallback when no size
  lookup is wired, so legacy unit tests that depend on Medium-shape
  occupants keep working.
