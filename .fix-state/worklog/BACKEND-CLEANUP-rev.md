# BACKEND-CLEANUP bundle — reviewer worklog

Reviewer: Claude Opus 4.7 (1M context), read-only.
Working directory: /home/ab/projects/DnDnD.

## Per-task verdicts

### A-10-auth-routes-dead-code — APPROVED
- `git diff` confirms `internal/auth/routes.go` and `internal/auth/routes_test.go`
  are deleted (full file removals).
- `grep -R "auth.RegisterRoutes"` across `internal/`, `cmd/`, and the rest of the
  repo returns **no** code matches (only docs / worklog / task references). The
  remaining `RegisterRoutes` hits in `cmd/dndnd/` are for unrelated packages
  (portal, itempicker, shops, levelup, etc.), not `auth`.
- Shared test mocks (`mockTokenRefresher`, `mockUserInfoFetcher`,
  `newMockSessionRepo`) live in `internal/auth/oauth2_test.go` and are still
  referenced by `middleware_test.go`/`oauth2_test.go`, so no orphan fixtures.
- `make build` green.

### A-14-portal-url-baseurl — APPROVED
- `defaultPortalBaseURL = "https://portal.dndnd.app"` + `portalBaseURL` field on
  `CreateCharacterHandler` + functional option
  `WithCreateCharacterPortalBaseURL` added in
  `internal/discord/registration_handler.go`. `strings.TrimRight(baseURL, "/")`
  prevents double slashes.
- `RegistrationDeps.PortalBaseURL` plumbed through
  `internal/discord/router.go:253-301`; only appended when non-empty (zero-
  config callers unchanged).
- `cmd/dndnd/main.go:1063-1067` reads `os.Getenv("BASE_URL")` into
  `registrationDepsConfig.portalBaseURL`; matches the existing `buildAuth`
  pattern.
- Two new tests in `registration_handler_test.go:375-431` —
  `_UsesConfiguredBaseURL` (asserts no `portal.dndnd.app` leak when override
  set) and `_DefaultsToProductionHost` (pins fallback). Test reads as a proper
  red/green pair against the deleted hard-coded URL.

### C-35-dm-adv-flags — DEFERRAL ACCEPTED
- Worklog cites `docs/phases.md:201` — verified: line 201 is the Phase 35
  bullet, which explicitly scopes "DM override from dashboard" as the missing
  piece.
- Code citations verified: `internal/combat/advantage.go:18-19,47-50` carries
  `DMAdvantage/DMDisadvantage`; `internal/combat/attack.go:316-1365` propagates
  them; `internal/combat/dm_dashboard_handler.go` exists and is out-of-scope
  per bundle edit zone.
- Gating event proposed (new POST endpoint mirroring existing override routes)
  is concrete and testable.

### H-121.4-playtest-transcripts — DEFERRAL ACCEPTED
- Task file (`Notes:` line 39) explicitly names
  deferred-with-justification as the expected outcome.
- New "Transcript capture status (H-121.4)" section appended at
  `docs/playtest-checklist.md:205-233` with deferral rationale, per-row gating
  event, and links back to the task file and `.fix-state/SUMMARY.md`. All 11
  scenarios retain `Status: pending`.

## Verification

- `make build` — green.
- `make test` — green (all 50+ packages pass).
- `make cover-check` — green after removing stale `coverage.out`; reports
  `Overall 92.32%`, every package above 85%, `internal/discord` at 86.51%.

## Verdict

All four tasks pass review: A-10 + A-14 implemented cleanly with red/green
tests, C-35 + H-121.4 deferrals are well-justified with spec anchors and
explicit gating events.
