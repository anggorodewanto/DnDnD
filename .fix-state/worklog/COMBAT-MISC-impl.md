# COMBAT-MISC implementation worklog

Bundle: E-68-fov-minor, E-69-obscurement-misc, C-43-followup-timer-nat20-heal-reset.
Date: 2026-05-11.

## C-43-followup-timer-nat20-heal-reset — DONE

Nat-20 timer auto-resolve path now mirrors `MaybeResetDeathSavesOnHeal`:
after writing HP=1 the new helper `(*TurnTimer).resetDyingStateAfterNat20`
also clears death-save tallies to `{0,0}` and removes the dying-condition
bundle (unconscious + prone). A revived PC no longer carries stale failure
tallies or sleeping conditions.

Files:
- `internal/combat/timer_resolution.go` — Nat-20 branch invokes
  `resetDyingStateAfterNat20`; helper added at end of file.
- `internal/combat/timer_resolution_test.go` — new
  `TestAutoResolveTurn_DyingCombatant_Nat20_ResetsDeathSavesAndDyingConditions`
  asserts persisted death-saves are zero and unconscious/prone are removed.

Acceptance criteria:
- [x] Nat-20 death save resolves to 1 HP AND `death_save_failures` reset to 0.
- [x] Test in combat package exercises the Nat-20 -> recovery path.
- [x] `go test`, `make cover-check`, `make build` clean.

## E-69-obscurement-misc — DONE (3 of 3 consumers wired)

`/check` perception, `/action hide`, and combat-log lighting reason are now
wired to the obscurement helpers.

Files:
- `internal/combat/obscurement.go` — exported `ColToIndex` so discord
  handlers can convert combatant columns without dipping into the private
  helper in attack.go.
- `internal/discord/check_handler.go` — new `CheckZoneLookup` interface +
  `SetZoneLookup` setter; `applyObscurementToInput` reads the caster's
  combatant + active zones, computes `CombatantObscurement`, applies
  `ObscurementCheckEffect` (combined into RollMode via
  `dice.CombineRollModes`), and appends the reason to the response.
- `internal/discord/check_handler_test.go` — new
  `TestCheckHandler_Perception_InObscuredZone_AppliesDisadvantageAndReason`
  asserts a heavy-obscurement zone at the caster's tile yields a
  disadvantage roll and an "obscured" reason line.
- `internal/discord/action_handler.go` — new `ActionZoneLookup` interface +
  `SetZoneLookup` setter; `dispatchHide` gates Hide on
  `ObscurementAllowsHide` when zones exist and surfaces the lighting reason
  on success.
- `internal/discord/action_handler_dispatch_test.go` — new
  `TestActionHandler_Dispatch_Hide_BlockedWhenNotObscured` and
  `TestActionHandler_Dispatch_Hide_AllowedWhenObscured`.

Acceptance criteria:
- [x] `/check` perception inside obscurement applies disadvantage from
  `ObscurementCheckEffect`.
- [x] `/action hide` is blocked or allowed per `ObscurementAllowsHide`.
- [x] Combat log / response surfaces the obscurement reason.
- [x] Tests fail before / pass after.
- [x] `go test`, `make cover-check`, `make build` clean.

### Follow-up (out of this batch's edit zone — defer)

Production wiring in `cmd/dndnd/discord_handlers.go` (closed for this
batch) should call `handlers.check.SetZoneLookup(deps.combatService)` and
`handlers.action.SetZoneLookup(deps.combatService)`. `*combat.Service`
already satisfies both interfaces structurally via
`ListZonesForEncounter`. Without this wiring the gates / reasons are
silent (preserves test parity); with it, lighting-aware encounters get the
full RAW behavior.

## E-68-fov-minor — DEFERRED with justification

All four sub-items in the task file point at code paths OUTSIDE this
bundle's edit zone:

- `VisionSource.HasDevilsSight` lives in
  `internal/gamemap/renderer/fog_types.go` (renderer, closed).
- `ComputeVisibilityWithLights` magical-darkness demotion lives in
  `internal/gamemap/renderer/fog_types.go` (renderer, closed).
- DM-sees-all toggle lives in `internal/gamemap/renderer/fog.go`
  (renderer, closed).
- Symmetric shadowcasting rename / algorithm swap touches
  `internal/gamemap/renderer/fow.go` (renderer, closed) — explicitly
  flagged as a non-trivial algorithmic rewrite to defer regardless.

The combat-side `VisionCapabilities` already carries `HasDevilsSight` and
`obscurement.go` honors it (this batch added the public `ColToIndex`
helper there). The minor refinement needed in this batch's zone is
limited to that helper and `obscurement.go`-driven consumers (covered by
E-69 above).

Recommend re-bundling E-68 with the renderer batch.

## Verification

- `go test ./internal/combat/ ./internal/discord/` — pass.
- `make test` — pass.
- `make cover-check` — pass; thresholds held (combat package coverage
  unchanged, discord still above per-package threshold).
- `make build` — clean.

## Hard-rule compliance

- No git stash/reset/clean used.
- No edits outside the declared edit zone.
- No regression of batches 1, 2, 3a, 3b, 3c wiring (full discord +
  combat suites pass).
