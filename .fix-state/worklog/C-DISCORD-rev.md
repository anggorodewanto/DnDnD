# C-DISCORD bundle — reviewer worklog

Reviewer: Claude Opus 4.7 (1M context). READ-ONLY.

## Per-task verdicts

### C-30-occupant-size — APPROVED
`buildOccupants(all, mover, sizeFn)` in `move_handler.go` resolves size via
the wired `sizeSpeedLookup` (closure from `occupantSizeFn(ctx)`); nil-safe
fallback to `SizeMedium`. All 5 call sites updated (combat /move,
exploration /move, prone stand & move, prone crawl via `buildOAPath`,
`buildGridForTurn`). Tests at `remediation_test.go:25/53/64` exercise
Tiny/Large pass-through and nil-fallback.

### C-32-range-rejection-format — APPROVED
`formatAttackError` + `formatOffhandAttackError` route the
`"out of range: Xft away (max Yft)"` sentinel through
`combat.FormatRangeRejection`. Non-range errors keep legacy
`"Attack failed: <err>"` wording. 5 tests cover main + off-hand + legacy +
parser branches.

### C-33-followup-discord-walls — APPROVED
`AttackMapProvider` interface + `SetMapProvider` setter + `loadWalls`
helper mirror `cast_handler.loadWalls`. `AttackCommand.Walls` /
`OffhandAttackCommand.Walls` populated when wired; nil-safe degrades to
"no wall cover". Production wiring of `cmd/dndnd/discord_handlers.go`
filed as `C-DISCORD-followup-cmd-wire-setters.md` (verified present).

### C-40-frightened-move — APPROVED
`rejectFrightenedTowardSource` invoked at validation time (before grid
build) in combat /move. Calls `combat.ValidateFrightenedMovement` with
fear-source positions map. Tests cover block/allow/no-condition cases.

### C-43-stabilize — APPROVED with caveat
`/action stabilize <target>` dispatch case + `dispatchStabilize` with DC
10 gate, 5ft Chebyshev reach, dying-state gate (`combat.IsDying`),
`StabilizeTarget` helper, persisted via `ActionStabilizeStore`. 5 tests.
Caveat: flat d20 (no WIS modifier) — documented in worklog as future plumbing.

### C-43-block-commands — APPROVED
Shared `incapacitatedRejection` + `dyingRejection` in `commands.go`; wired
into `/move`, `/attack`, `/action` (both freeform + dispatch), `/bonus`
(via `resolveContext`). `/deathsave` correctly untouched. 4 tests cover
dying + unconscious paths across handlers.

## Verification

- `make build`: clean
- `make test`: all packages green (no FAIL output)
- `make cover-check`: OK; discord package 86.96%
- Batch-1/2 dispatch intact: stand, drop-prone, escape, channel-divinity,
  lay-on-hands, hide, dash, dodge cases preserved; cover wiring in attack
  pipeline preserved (both AttackCommand + OffhandAttackCommand carry Walls).
- Test-fixture migration: bulk `HpCurrent: 10` addition prevents
  pre-existing tests from being silently marked "dying" by the new guard.

## Findings

- The frightened-validation helper parses positions via concatenation
  `mover.PositionCol + fmt.Sprintf("%d", mover.PositionRow)` — a minor
  style issue but functional. Same pattern in `stabilizeReachFt`.
- `C-DISCORD-followup-cmd-wire-setters.md` properly filed (status: open).

## Next steps

1. Land follow-up `cmd/dndnd/discord_handlers.go` wiring for
   `SetMapProvider` and `SetStabilizeStore`.
2. Plumb actor WIS modifier into `ActionCombatService` so the DC 10
   Medicine check is not flat.
3. Close C-30/32/33-followup/40/43-stabilize/43-block in TASKS.md.
