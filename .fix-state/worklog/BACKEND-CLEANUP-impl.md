# BACKEND-CLEANUP bundle — implementer worklog

Implementer: Claude Opus 4.7 (1M context).
Working directory: /home/ab/projects/DnDnD.

Scope: 4 backend-cleanup tasks — A-10 + A-14 implemented, C-35 + H-121.4
closed as `deferred-with-justification`.

## Per-task status

### A-10-auth-routes-dead-code — DONE
- `rg "auth\.RegisterRoutes"` confirmed only `internal/auth/routes_test.go`
  referenced the helper (the portal mounts `/portal/auth/*` inline via
  `internal/portal/routes.go`).
- Deleted `internal/auth/routes.go` and `internal/auth/routes_test.go`.
  The shared OAuth test mocks (`mockTokenRefresher`, `mockUserInfoFetcher`,
  `newMockSessionRepo`) live in `oauth2_test.go`, so removing the routes
  test does not orphan any fixtures.
- `go build ./...` clean, `go test ./internal/auth/...` clean.
- Acceptance: `grep -R "auth.RegisterRoutes" internal cmd` returns no
  matches. `make test && make cover-check && make build` green.

### A-14-portal-url-baseurl — DONE
- `internal/discord/registration_handler.go`: added
  `defaultPortalBaseURL = "https://portal.dndnd.app"` constant +
  `portalBaseURL` field on `CreateCharacterHandler` + functional-option
  `WithCreateCharacterPortalBaseURL(baseURL string) CreateCharacterOption`
  (mirrors the existing `ImportHandlerOption` pattern). The `/create-character`
  Handle now builds `fmt.Sprintf("%s/create?token=%s", base, token)` where
  `base` is the configured override (trailing `/` trimmed) or the default.
- `internal/discord/router.go`: added `PortalBaseURL string` field on
  `RegistrationDeps`; the `create-character` handler is now constructed
  with `WithCreateCharacterPortalBaseURL` whenever the field is non-empty,
  keeping zero-config callers unchanged.
- `cmd/dndnd/main.go`: threaded `portalBaseURL` through
  `registrationDepsConfig` → `buildRegistrationDeps` → `discord.RegistrationDeps`;
  the prod call site reads `os.Getenv("BASE_URL")` (matching the existing
  `buildAuth` pattern in the same file).
- Tests in `internal/discord/registration_handler_test.go`:
  - `TestCreateCharacterHandler_PortalURL_UsesConfiguredBaseURL` — asserts
    the emitted link uses the configured BASE_URL and does NOT contain
    `portal.dndnd.app`. Verified red against the old hard-coded URL,
    green after the fix.
  - `TestCreateCharacterHandler_PortalURL_DefaultsToProductionHost` —
    pins the fallback so existing zero-config callers (and the original
    smoke tests) keep working.
- Acceptance: `make test && make cover-check && make build` green.

### C-35-dm-adv-flags — DEFERRED-WITH-JUSTIFICATION
- **Justification:** Phase 35 (`docs/phases.md:201`) explicitly scopes the
  DM advantage/disadvantage override as **"DM override from dashboard."**
  The data-model layer already honors the flags end-to-end
  (`internal/combat/advantage.go:18-19,47-50`, propagated through
  `AttackInput.DMAdvantage` / `DMDisadvantage` in
  `internal/combat/attack.go:316-1365` and `internal/combat/monk.go`).
  The dashboard-side setter is absent — there is no
  `internal/dashboard/combat_dm_handler.go` and the existing DM dashboard
  handler (`internal/combat/dm_dashboard_handler.go`) is outside this
  bundle's edit zone ("`internal/combat/* — read-only inspection allowed`",
  bundle scope). Wiring a minimum setter would require either adding a
  new method to `combat.DMDashboardHandler` (forbidden), persisting the
  flag on `refdata` queries (cross-cuts the combat package), or
  introducing a parallel dashboard handler that mutates combatant state
  without sharing the existing per-turn advisory lock — none of which
  qualifies as "minimum dashboard handler" per the bundle instructions.
- **Spec anchor:** `docs/phases.md:201-204`,
  `.review-state/group-C-phases-29-43.md` Phase 35 findings.
- **Gating event to close:** introduce a dashboard-side
  POST `/api/combat/{encounterID}/override/combatant/{combatantID}/dm-adv`
  endpoint on `combat.DMDashboardHandler` (mirroring the existing
  `override/combatant/{combatantID}/hp` / `position` / `conditions`
  endpoints in the same router block) that sets a one-shot flag on the
  combatant which `buildAttackInput` consumes and clears. That work
  belongs to the next sweep of `internal/combat/` (out of zone for this
  bundle).
- **No code change in this commit.**

### H-121.4-playtest-transcripts — DEFERRED-WITH-JUSTIFICATION
- **Justification:** task file's stated expected outcome is exactly
  deferred-with-justification (Notes: "this task tracks
  deferred-with-justification as the expected outcome"). Real transcripts
  must be captured by `cmd/playtest-player` against a live Discord bot +
  database during an actual playtest session; the 11 scenarios exercise
  cross-system flows (`#combat-log` echoes, DM dashboard edits, real
  death-save dice) that the offline harness cannot fabricate without
  bypassing the integrations the recordings are meant to replay-test.
- **Spec anchor:** `docs/phases.md:632-829` (Group H, Phase 121.4);
  Phase 121.4 is explicitly marked `[ ]` in the phase doc, and the bundle
  task explicitly says deferred-with-justification is the expected
  outcome.
- **Gating event (documented now in `docs/playtest-checklist.md`):** when
  a scenario is walked end-to-end under `playtest-player --record …`,
  the JSONL lands at
  `internal/playtest/testdata/transcripts/<name>.jsonl`, and
  `make playtest-replay TRANSCRIPT=…` exits 0 against the recording, the
  scenario flips from `Status: pending` to `Status: captured`. The
  recorder, replay loader, and `make playtest-replay` target are already
  wired and green against `internal/playtest/testdata/sample.jsonl`.
- **Doc change:** appended a "Transcript capture status (H-121.4)"
  section to `docs/playtest-checklist.md` capturing the deferral, the
  why, the precise gating event per scenario, and the already-in-place
  tooling. All 11 scenarios already carry `Status: pending` so the
  listing requirement (criterion to flip each row) is satisfied.
- **No code change in this commit.**

## Verification

- `make test` — green (full module, including the two new
  `internal/discord` tests).
- `make cover-check` — green ("OK: coverage thresholds met"); `internal/discord`
  at 86.51%, above the 85% per-package threshold.
- `make build` — green (`bin/dndnd`, `bin/playtest-player`).
- `/simplify` — reviewed in-conversation. The A-14 implementation reuses
  the existing `ImportHandlerOption` functional-options pattern from the
  same file; no duplicate helpers introduced; `strings.TrimRight` is the
  only normalization; no hot-path additions.

## Files touched

- `internal/auth/routes.go` (deleted)
- `internal/auth/routes_test.go` (deleted)
- `internal/discord/registration_handler.go` (CreateCharacterHandler options)
- `internal/discord/registration_handler_test.go` (two new tests)
- `internal/discord/router.go` (RegistrationDeps.PortalBaseURL)
- `cmd/dndnd/main.go` (thread BASE_URL through `buildRegistrationDeps`)
- `docs/playtest-checklist.md` (H-121.4 deferral note)
