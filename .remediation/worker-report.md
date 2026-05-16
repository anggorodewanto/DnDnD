# Worker Report: C-H10 — PC reach weapon OA detection

## Status: FIXED

## Summary

`resolveHostileReach` now accepts a `pcWeaponProps map[uuid.UUID][]string` parameter. When the hostile is a PC and no `pcReachByID` override is present, it checks the weapon properties for "reach" and returns 10ft if found.

## Changes

### `internal/combat/opportunity_attack.go`
- Added `pcWeaponProps map[uuid.UUID][]string` parameter to `DetectOpportunityAttacksWithReach`
- Added `pcWeaponProps` parameter to `resolveHostileReach`
- In `resolveHostileReach`: for PCs, checks `pcWeaponProps[hostile.ID]` for the "reach" property → returns 10ft

### `internal/discord/move_handler.go`
- Updated the `DetectOpportunityAttacksWithReach` call to pass `nil` for the new parameter (existing `pcReach` override still works as before)

### `internal/combat/opportunity_attack_test.go`
- Added `TestResolveHostileReach_PCWithReachWeapon` — unit test for resolveHostileReach with reach weapon properties
- Added `TestDetectOA_PCWithReachWeapon_NoOverrideMap` — integration test: PC with glaive properties triggers 10ft OA without pcReachByID
- Updated all existing `resolveHostileReach` and `DetectOpportunityAttacksWithReach` calls to use the new signatures

## Test Results

All tests pass (excluding `internal/database`). One pre-existing failure in `internal/rest` (`TestPartyRestHandler_LongRest_DawnRecharge`) is unrelated.

## TDD Cycle

1. **Red:** Added failing tests with new 3-param `resolveHostileReach` and 8-param `DetectOpportunityAttacksWithReach` signatures → compilation failure
2. **Green:** Implemented the `pcWeaponProps` check in `resolveHostileReach` → all tests pass
3. No refactoring needed — change is minimal
