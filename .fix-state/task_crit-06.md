# task crit-06 — Portal token validator nil + TokenFunc placeholder

## Finding (verbatim from chunk7_dashboard_portal.md, Phase 91a)

> ✅ Token logic complete: `TokenService` with single-use semantics + 24 h TTL parameterized (`internal/portal/token_service.go:48-90`); cryptographic random tokens.
> ✅ Discord `/create-character` posts an ephemeral link (`internal/discord/registration_handler.go:262`).
> ✅ Portal landing/create/error templates (`internal/portal/handler.go:158-269`).
> ❌ **`TokenService` never instantiated from `main.go`.** `grep -rn portal.NewTokenService cmd/` returns nothing. `cmd/dndnd/main.go:730` uses a hardcoded `func(_ uuid.UUID, _ string) (string, error) { return "e2e-token", nil }`.
> ❌ **`portal.NewHandler(logger, nil)` is constructed with a nil validator** (`cmd/dndnd/main.go:594`). Visiting `/portal/create?token=…` will trigger a nil dereference at `internal/portal/handler.go:94` (`tok, err := h.validator.ValidateToken(...)`).

Spec: Phase 91a in `docs/phases.md`; "Player Portal (Web)" coverage map line 895.

Recommended approach (chunk7 follow-up #1): "Wire `portal.TokenService` in `cmd/dndnd/main.go` — replace the placeholder `TokenFunc` with `tokenSvc.CreateToken(ctx, campaignID, discordUserID, "create_character", 24*time.Hour)` and pass the same `tokenSvc` as the validator to `portal.NewHandler`."

## Plan

1. Add `newPortalTokenIssuer(ctx, svc) func(uuid.UUID, string) (string, error)` helper to `cmd/dndnd/discord_adapters.go` so the wiring is unit-testable in isolation (main.go is excluded from coverage).
2. Construct `portalTokenSvc := portal.NewTokenService(portal.NewTokenStore(db))` once in main.go around line 591.
3. Pass `portalTokenSvc` (which satisfies `portal.TokenValidator`) to `portal.NewHandler` instead of `nil`.
4. Replace the `TokenFunc: func(_ uuid.UUID, _ string) (string, error) { return "e2e-token", nil }` placeholder at line 731 with `TokenFunc: newPortalTokenIssuer(ctx, portalTokenSvc)`.
5. Constants: hoist purpose (`"create_character"`) and TTL (`24 * time.Hour`) into named consts so the spec invariant lives next to the helper.
6. Failing-first tests in a new `cmd/dndnd/portal_wiring_test.go`: prove the helper does NOT return `"e2e-token"`, that it persists a row with the right campaign/user/purpose/TTL, and that the same `TokenService` round-trips issue → `ValidateToken`.

## Files touched

- `cmd/dndnd/main.go` — replace nil validator + placeholder TokenFunc with shared `portalTokenSvc` wiring.
- `cmd/dndnd/discord_adapters.go` — add `newPortalTokenIssuer` helper plus purpose/TTL consts; add `portal` import (and `time`).
- `cmd/dndnd/portal_wiring_test.go` (new) — fake `portal.TokenRepository` + 3 tests for the issuer (real-token, round-trip, error-propagation).

## Tests added

- `TestNewPortalTokenIssuer_ProducesRealToken_NotPlaceholder` — issuer must not return the literal `"e2e-token"`; persists with campaignID, discordUserID, purpose `"create_character"`, ~24h TTL.
- `TestNewPortalTokenIssuer_RoundTripsThroughValidator` — token minted by issuer is accepted by the same `TokenService.ValidateToken`.
- `TestNewPortalTokenIssuer_PropagatesServiceError` — repo Create error surfaces to caller (registration handler relies on a non-nil err to show "Error generating portal link").

## Implementation notes

- Helper takes `ctx` from `run()`'s `signal.NotifyContext` so token persistence honours graceful shutdown without forcing the registration handler to thread one through. `portal.TokenService.CreateToken` already accepts a `context.Context` so wiring is direct.
- Same `*portal.TokenService` instance backs both `portal.NewHandler(validator)` and `RegistrationDeps.TokenFunc`, so issue and validate hit the same store row — required for single-use semantics.
- `portal.NewTokenStore(db)` was already implemented; only the construction call was missing from main.go.
- `cmd/dndnd/main.go` is on the cover-check exclusion list, but the underlying helper in `discord_adapters.go` is on it too — added tests still drive the issuer end-to-end via a fake repo to lock in the contract.
- `make cover-check`: overall 94.39%; per-package thresholds met.
- `go test ./cmd/dndnd/... ./internal/portal/... ./internal/dashboard/...` — green.
- Out of scope (logged elsewhere or stays as separate tasks): `WithAPI` / `WithCharacterSheet` route-options on `portal.RegisterRoutes` (chunk7 follow-up #2), Phase 11 campaign auto-creation, OAuth campaign-id derivation. No changes to those surfaces.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

- main.go:600 constructs ONE `portalTokenSvc := portal.NewTokenService(portal.NewTokenStore(db))` and reuses the same instance at line 601 (validator on `portal.NewHandler`) and line 740 (`TokenFunc: newPortalTokenIssuer(ctx, portalTokenSvc)`). Single-use semantics preserved — issue and redeem hit the same store row.
- `newPortalTokenIssuer` returns `func(uuid.UUID, string) (string, error)` matching the `tokenFunc` field at `internal/discord/registration_handler.go:221` and the `NewCreateCharacterHandler` arg shape.
- Purpose `"create_character"` and TTL `24*time.Hour` hoisted into named consts in discord_adapters.go and verified by the test at portal_wiring_test.go:92,95.
- 3 tests in portal_wiring_test.go cover: real-token-not-placeholder + persisted fields (campaign/user/purpose/TTL), round-trip via the same `ValidateToken`, and Create-error propagation.
- `internal/portal/token_service.go` semantics unchanged (24h TTL parameter, single-use via `MarkUsed`, purpose tag stored). `make cover-check` previously verified by orchestrator; spot-check of cmd/dndnd, internal/portal: all green (cmd/dndnd 83.6%, internal/portal 94.5%).
- Out-of-scope items (Phase 91b/c `WithAPI`/`WithCharacterSheet`, Phase 11 campaign auto-create, OAuth campaign-id derivation) correctly deferred.
