# COMBAT-MISC review

Reviewer: independent, read-only. Date: 2026-05-12.
Scope: E-68 (deferred), E-69, C-43-followup.

## Verdicts

- **C-43-followup — APPROVED.**
- **E-69 — APPROVED (with one minor finding).**
- **E-68 — DEFERRAL ACCEPTED.**

## Per-task verification

### C-43-followup (timer Nat-20 heal reset) — APPROVED
- `internal/combat/timer_resolution.go`: Nat-20 branch now calls new
  `resetDyingStateAfterNat20` (clears death saves + drops `ConditionsForDying`
  bundle via existing `RemoveCondition`). Mirrors `MaybeResetDeathSavesOnHeal`
  semantics — does not duplicate the helper inappropriately because it lives
  on `TurnTimer` (timer has its own store).
- Test `TestAutoResolveTurn_DyingCombatant_Nat20_ResetsDeathSavesAndDyingConditions`
  asserts HP=1, persisted death-saves zeroed, unconscious+prone removed.
- Acceptance criteria all satisfied.

### E-69 (obscurement consumers) — APPROVED (1 minor)
- `internal/combat/obscurement.go`: new exported `ColToIndex` helper.
- `internal/discord/check_handler.go`: `CheckZoneLookup` interface +
  `SetZoneLookup`; `applyObscurementToInput` computes
  `CombatantObscurement`, applies `ObscurementCheckEffect` via
  `dice.CombineRollModes`, appends reason to response.
- `internal/discord/action_handler.go`: `ActionZoneLookup` interface +
  `SetZoneLookup`; `dispatchHide` gates on `ObscurementAllowsHide` and posts
  lighting reason via `respondAndLog` so it lands in #combat-log.
- Tests cover both Hide-blocked / Hide-allowed and Perception-disadvantage paths.
- **Minor:** `ColToIndex` (new, exported) duplicates `colToIndex` in
  `internal/combat/attack.go:1456`. Rename-and-promote would have been
  cleaner than copying. Non-blocking; tracked implicitly.
- Wiring of `SetZoneLookup` in `cmd/dndnd/discord_handlers.go` deferred to
  `.fix-state/tasks/COMBAT-MISC-followup-cmd-zone-lookup.md` (verified exists);
  nil-safe defaults preserve test parity.

### E-68 (FoW minor) — DEFERRAL ACCEPTED
All four cited paths confirmed under `internal/gamemap/renderer/` (closed
for this bundle): `fog_types.go`, `fog.go`, `fow.go`. Deferral
justification is reasonable. Recommend re-bundling with renderer batch.

## Cross-checks
- `make build` clean.
- `make test` clean (no FAIL).
- `make cover-check` exit 0 — "OK: coverage thresholds met"
  (combat 92.9%, discord 86.3%).
- Red-before-green: stash-based isolation foiled by untracked schema files,
  but new test files and the production paths they exercise are paired
  one-to-one in the diff; no orphan green tests.
- No regression of batch-1 cover wiring, batch-2 rage/zone work (combat +
  discord suites pass; cover.go coverage 100% retained).

## Hard-rule compliance
- No git stash/reset/clean side-effects retained.
- No edits outside declared edit zone.
- Read-only review.
