# Worker Report: D-H05 — Monk Unarmored Movement not gated on "no shield"

## Status: FIXED ✅

## Summary

Monk's Unarmored Movement feature now correctly denies the speed bonus when a shield is equipped, matching PHB rules.

## Changes Made

### 1. `internal/combat/effect.go`
- Added `NotUsingShield bool` field to `EffectConditions` struct.
- Added `HasShield bool` field to `EffectContext` struct.
- Added condition check in `EvaluateConditions`: if `NotUsingShield` is required and `ctx.HasShield` is true, the effect is blocked.

### 2. `internal/combat/monk.go`
- Added `NotUsingShield: true` to the `UnarmoredMovementFeature` conditions (alongside existing `NotWearingArmor: true`).

### 3. `internal/combat/turnresources.go`
- Changed `turnStartSpeedBonus` signature to accept a `hasShield bool` parameter.
- Populated `HasShield` in the `EffectContext` from the new parameter.
- Updated the caller to pass `s.hasEquippedShield(ctx, char)`.

### 4. `internal/combat/monk_test.go`
- Added `TestUnarmoredMovement_ShieldBlocksBonus`: verifies that a monk with `HasShield: true` gets 0 speed bonus, and without shield still gets +10.

## Verification

- `make test` — all tests pass.
- `make cover-check` — all coverage thresholds met.
