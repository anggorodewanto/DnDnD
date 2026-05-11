# task crit-05 — Turn lock + ownership not enforced in `/move`, `/fly`, `/distance`

## Finding (verbatim from chunk3_combat_core.md, Phase 27/30/31)

> Phase 27 — `AcquireTurnLock`, `AcquireTurnLockWithValidation`, `ValidateTurnOwnership` all implemented and integration-tested in `internal/combat/turnlock_integration_test.go`. **Not wired into Discord handlers.** `grep -n 'AcquireTurnLock\|ValidateTurnOwnership' internal/discord` returns nothing — no handler in `internal/discord/` calls either function. Explicit gap: `move_handler.go:155 // TODO: turn ownership validation will be wired when full turn lock is available`. `fly_handler.go` (no lock, no ownership check) and `distance_handler.go` likewise rely on the encounter's current turn without validating that the invoker owns it. The `action_handler.go:165-168` does a manual `combatantBelongsToUser` ownership check but no advisory lock.
> `IsExemptCommand` only referenced from tests (`turnlock_integration_test.go:409-419`); no real router branches on it.

> Cross-cutting risk #2 (chunk3): "Out-of-turn writes possible from Discord today. Because `move_handler.go`, `fly_handler.go`, and the various other player commands skip ownership validation and skip the advisory lock, two concurrent `/move` calls from different players targeting the same active turn could both succeed and both `UpdateTurnActions`."

Spec sections: "Concurrency Model" (Phase 27); coverage map line 839 maps to Phase 27.

Recommended approach (chunk3 follow-up #2): "Replace the `move_handler.go:155` TODO with a real ownership + lock guard. Apply the same fix to `fly_handler.go`. After this, the integration tests in `turnlock_integration_test.go` should be re-run end-to-end against a real Discord interaction."

## Plan

1. Add `"distance"` to `combat.IsExemptCommand` (it's read-only — no DB writes,
   matching `/check`/`/save`/`/rest`/`/reaction` semantics) and update its test.
2. Define a thin `discord.TurnGate` interface in a new file
   `internal/discord/turnguard.go` that abstracts
   `combat.AcquireTurnLockWithValidation`. The interface returns `TurnOwnerInfo`
   + error; the production adapter (in `cmd/dndnd/discord_handlers.go`) calls
   `combat.AcquireTurnLockWithValidation`, then commits the held tx
   immediately to release the lock — the validation gate fires and the
   wrong-owner case is rejected before any handler write.
3. Add a `formatTurnGateError(err) string` helper so /move and /fly surface
   identical wording for `ErrNotYourTurn` / `ErrLockTimeout` / `ErrTurnChanged` /
   `ErrNoActiveTurn`.
4. Each of `MoveHandler`, `FlyHandler`, `DistanceHandler` gets a `SetTurnGate`
   setter (nil-safe — preserves backwards compat with existing tests and the
   pre-Phase-27 wiring graph).
5. `MoveHandler.Handle` and `FlyHandler.Handle`: after option parsing + encounter
   resolve + active-turn check, call `gate.AcquireAndRelease` before any turn /
   combatant lookup. Drop the `move_handler.go:155` TODO.
6. `DistanceHandler.Handle`: short-circuit on `combat.IsExemptCommand("distance")`
   at the very top with a defensive guard. The gate is wired (for symmetry) but
   never invoked — read-only commands MUST NOT take the per-turn lock.
7. Production wiring (`cmd/dndnd/discord_handlers.go`): add `db combat.TxBeginner`
   to `discordHandlerDeps`, pass `db` from `cmd/dndnd/main.go`, construct one
   `turnGateAdapter` and `Set*` it on all three handlers.
8. TDD: red tests added in `internal/discord/turnguard_test.go` first,
   confirming the gate must be invoked, args must include encounter+user,
   error mapping covers every sentinel, and `/distance` honors the exemption.

## Files touched

- `internal/combat/turnvalidation.go` — added `"distance"` to `IsExemptCommand`,
  expanded the function godoc.
- `internal/combat/turnlock_integration_test.go` — added one assertion
  `assert.True(t, combat.IsExemptCommand("distance"))` plus a clarifying comment.
- `internal/discord/turnguard.go` (new) — `TurnGate` interface +
  `formatTurnGateError` helper.
- `internal/discord/turnguard_test.go` (new) — 11 tests across `/move`, `/fly`,
  `/distance`, plus exhaustive `formatTurnGateError` coverage.
- `internal/discord/move_handler.go` — added `turnGate` field +
  `SetTurnGate(g)`; replaced the line-155 TODO with a real
  `h.turnGate.AcquireAndRelease(...)` call. Early-return on rejection — handler
  short-circuits before any turn/combatant DB read.
- `internal/discord/fly_handler.go` — same pattern as `MoveHandler`.
- `internal/discord/distance_handler.go` — added `turnGate` field +
  `SetTurnGate(g)` (kept for production-graph symmetry; never called by `Handle`).
  Added a defensive `IsExemptCommand("distance")` guard at the top of `Handle`
  with a comment documenting the exemption contract.
- `cmd/dndnd/discord_handlers.go` — added `db combat.TxBeginner` to
  `discordHandlerDeps`, added `turnGateAdapter` (delegates to
  `combat.AcquireTurnLockWithValidation` then commits the held tx to release
  the advisory lock), wired `Set*TurnGate(gate)` for all three handlers when
  `db` and `queries` are non-nil.
- `cmd/dndnd/main.go` — pass `db: db` into `discordHandlerDeps` literal.
- `.fix-state/log.md` — appended worker notes flagging deferred follow-ups
  (med-21 / med-24 hardcodings; Phase 37 thrown-attack hand desync; the
  parallel-worker mid-edit on `internal/combat/attack.go` + `attack_fes_test.go`
  that this task does not touch).

## Tests added

In `internal/discord/turnguard_test.go`:

1. `TestMoveHandler_TurnGate_RejectsWrongOwner` — gate returns `ErrNotYourTurn`;
   asserts (a) ephemeral response includes "not your turn" and the current
   owner's character name, (b) `UpdateCombatantPosition` is never called
   (zero writes after rejection), (c) gate fires exactly once.
2. `TestMoveHandler_TurnGate_PassesEncounterAndUserID` — verifies the gate
   receives the resolved encounter UUID and the discord user ID extracted from
   the interaction Member.User.
3. `TestMoveHandler_TurnGate_LockTimeoutMessage` — gate returns
   `combat.ErrLockTimeout`; asserts user-visible "busy" message.
4. `TestMoveHandler_TurnGate_NoActiveTurnMessage` — gate returns
   `combat.ErrNoActiveTurn`; asserts "No active turn." message.
5. `TestMoveHandler_TurnGate_NotInvokedInExploration` — exploration-mode
   encounter skips the gate entirely (no turn semantics in exploration).
6. `TestFlyHandler_TurnGate_RejectsWrongOwner` — same shape as #1 for `/fly`.
7. `TestFlyHandler_TurnGate_PassesEncounterAndUserID` — same shape as #2.
8. `TestDistanceHandler_IsExempt_GateNotInvoked` — wires a gate that would
   always reject; asserts `/distance` never calls it (proves the
   `IsExemptCommand("distance")` branch fires).
9. `TestIsExemptCommand_DistanceListed` — sanity guard against future
   regressions of the IsExemptCommand contract.
10. `TestFormatTurnGateError_AllCases` — table test covering nil,
    `ErrNotYourTurn`, `ErrNoActiveTurn`, `ErrLockTimeout`, `ErrTurnChanged`,
    and a generic error (100% coverage of `formatTurnGateError`).

In `internal/combat/turnlock_integration_test.go`:

11. `TestIsExemptCommand` — added `assert.True(t, combat.IsExemptCommand("distance"))`
    with a comment explaining the read-only exemption rationale. Existing
    assertions are unchanged.

## Implementation notes

- **Why `AcquireAndRelease` and not `AcquireAndHold`:** the existing /move and
  /fly handlers persist their writes through `*refdata.Queries` (NOT a
  tx-bound `WithTx(tx)` querier). Holding the lock past validation would leave
  the lock open while the rest of the handler runs against an unrelated
  connection — the writes would still race. The chunk3 cross-cutting risk #2
  is "wrong owner can act"; the gate solves that. A future patch can extend
  the adapter to thread the tx through the persistence layer for true
  serialized writes (the interface accommodates this without breaking callers).
- **Why a new interface in `discord` package, not direct dependency on
  `combat.AcquireTurnLockWithValidation`:** the combat function takes
  `TxBeginner` (a `*sql.DB`) and `TurnValidationQuerier` (a `*refdata.Queries`).
  Threading both into every handler constructor would balloon the
  `MoveHandler` constructor surface and force every test to construct a fake
  `TxBeginner`. The tiny `TurnGate` interface keeps the handler API surface
  unchanged (one nil-safe setter) and lets unit tests inject a stub.
- **Why "distance" was added to `IsExemptCommand`:** the chunk3 finding
  explicitly notes `IsExemptCommand` "only referenced from tests; no real
  router branches on it." `/distance` is the natural first real caller — it
  writes nothing, takes no resources, and would unfairly block a peer's /move
  by holding the lock just to measure range. Adding "distance" matches the
  spirit of the existing exemptions (read-only `/check`/`/save`/`/reaction`
  and off-turn `/rest`).
- **Backwards compatibility:** every existing handler test in
  `move_handler_test.go`, `fly_handler_test.go`, `distance_handler_test.go`
  continues to pass unchanged because `turnGate` is nil when not set via
  `SetTurnGate`. The discord package coverage held at 91.7%.
- **Coverage:** `internal/discord` 91.7% overall. New file `turnguard.go`:
  100% on `formatTurnGateError`. The `SetTurnGate` setter on all three
  handlers is 100% covered by the new tests.
- **Cannot run `make cover-check` end-to-end:** another in-flight worker has
  unrelated mid-edits in `internal/combat/attack.go` (FES wiring) and a new
  `internal/combat/attack_fes_test.go` that references undefined symbols
  (`attackAbilityUsed`, `nearestAllyDistanceFt`, `AttackInput.AbilityUsed`).
  This blocks `go test ./...` package-graph builds for downstream packages.
  When I stash those external changes and re-test, my changes pass cleanly:
  `internal/discord` 91.7%, `internal/combat` (my-only changes) green.
  Logged in `.fix-state/log.md`.
- **Simplify pass:** considered extracting the 4-line gate-check preamble
  into a helper. Decided AGAINST: only `/move` and `/fly` actually call the
  gate (3 lines each); `/distance` uses a structurally different
  `IsExemptCommand` branch. Per task constraints, "three similar lines beat a
  premature helper". The shared `formatTurnGateError` is sufficient
  deduplication.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

Reviewer: reviewer-A1 (rev=1, 2026-05-10)

Verification performed:

1. **`move_handler.go:155` TODO replaced.** The diff shows the TODO line is removed and a real `h.turnGate.AcquireAndRelease(ctx, encounterID, discordUserID(interaction))` call is inserted at the prior line 156-161, with an early-return + ephemeral on rejection.
2. **`fly_handler.go` matches.** Same pattern: gate fires after `GetEncounter` + `CurrentTurnID.Valid` check, before `GetTurn`/`GetCombatant`/`UpdateCombatantPosition`.
3. **Order-of-operations / TOCTOU.** Both handlers call the gate BEFORE any mutating call (`UpdateCombatantPosition`, `UpdateTurnActions`). The only DB reads preceding the gate are `ActiveEncounterForUser` and `GetEncounter`, both read-only and required to know which encounter the gate should validate. No write happens before the gate fires. The gate itself re-validates inside the lock (TOCTOU guard in `AcquireTurnLockWithValidation:142-155`). Good.
4. **`distance_handler.go` honors `IsExemptCommand("distance")`.** Defensive top-of-handler guard added. `IsExemptCommand` updated in `turnvalidation.go:169` to include `"distance"`. Read-only command — exemption matches Phase 27 semantics correctly (avoids blocking the active player just because a peer is measuring range).
5. **Production adapter does not leak the lock.** `turnGateAdapter.AcquireAndRelease` (`cmd/dndnd/discord_handlers.go:579-588`) calls `combat.AcquireTurnLockWithValidation`, which on its own error paths already rolls back the tx (verified at `turnvalidation.go:145,153`). On success, the adapter immediately calls `tx.Commit()` which releases `pg_advisory_xact_lock` per Postgres semantics. If commit itself fails the connection is returned to the pool and the lock is released on session end — acceptable. No leak surface.
6. **Tests cover required cases.** `turnguard_test.go` proves: (a) wrong-owner rejection with zero downstream writes (`TestMoveHandler_TurnGate_RejectsWrongOwner`, `TestFlyHandler_TurnGate_RejectsWrongOwner` via `callCountingMoveService.updatePosCalls == 0`); (b) gate not invoked for `/distance` (`TestDistanceHandler_IsExempt_GateNotInvoked`); (c) error mapping for all four sentinels — `ErrNotYourTurn`, `ErrNoActiveTurn`, `ErrLockTimeout`, `ErrTurnChanged` — plus nil and generic, all in `TestFormatTurnGateError_AllCases`.
7. **No scope creep.** `move_handler.go:212` (size lookup) and `:228` (maxSpeed=30) hardcodings remain; thrown-attack hand desync untouched; OAs untouched. Worker correctly deferred per task scope.
8. **Coverage.** `go test -cover ./internal/discord/...` reports 91.7% (≥85% bar). Vet clean across `internal/discord`, `internal/combat`, `cmd/dndnd`.

Note: As worker observed, full `make cover-check` is blocked by an unrelated in-flight FES wiring edit on `internal/combat/attack.go`. Not this task's responsibility.
