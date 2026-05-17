# Worker Report: H-H05 — Builder service token redeem race & ownership check

## Status: FIXED

## Summary

Token validation and redemption now happen BEFORE character creation, and token ownership is enforced.

## Changes Made

### `internal/portal/builder_service.go`
1. Added `ErrTokenOwnership` sentinel error.
2. Added `ValidateToken(ctx, token) (*PortalToken, error)` to the `BuilderStore` interface.
3. Reordered `CreateCharacter`: validate token → check ownership → redeem token → create character/player_character records.
4. `RedeemToken` failure is now a hard error (no longer a warning-only path).

### `internal/portal/builder_store_adapter.go`
- Added `ValidateToken` method delegating to `tokenSvc.ValidateToken`.

### `internal/portal/builder_service_test.go`
- Added `validateToken` and `validateTokenErr` fields to `mockBuilderStore`.
- Added `ValidateToken` method to mock.
- **New test:** `TestBuilderService_CreateCharacter_RejectsMismatchedTokenUser` — verifies mismatched `DiscordUserID` returns `ErrTokenOwnership` and no character is created.
- **New test:** `TestBuilderService_CreateCharacter_RedeemFailsPreventsCreation` — verifies redeem failure prevents character creation.
- Updated `TestBuilderService_RedeemTokenError_WithLogger` to expect error (behavior change).

## Test Results

```
=== RUN   TestBuilderService_CreateCharacter_RejectsMismatchedTokenUser
--- PASS
=== RUN   TestBuilderService_CreateCharacter_RedeemFailsPreventsCreation
--- PASS
```

Full `./internal/portal/...` suite: **PASS** (all tests green).

One pre-existing failure in `internal/rest` (unrelated: `TestPartyRestHandler_LongRest_DawnRecharge`).
