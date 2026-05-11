---
id: F-81-group-check-handler
group: F
phase: 81
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Group check has no Discord/dashboard handler

## Finding
`check.GroupCheck` exists with tests but no Discord or dashboard handler invokes it. The DM cannot trigger a group check; there are no `GroupCheck` references outside the service.

## Code paths cited
- `internal/check/check.go` — `GroupCheck` service method exists
- `internal/discord/check_handler.go` — no `GroupCheck` invocation
- (no dashboard route for group check)

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 81) — group check coverage required

## Acceptance criteria (test-checkable)
- [ ] DM can trigger a group check via Discord command or dashboard endpoint
- [ ] Handler calls `check.GroupCheck` and returns aggregated pass/fail result
- [ ] Test in `internal/discord/check_handler_test.go` or relevant handler test file fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-81-targeted-check-handler (same package)
- F-81-dm-prompted-checks (same package)

## Notes
Service is complete; only the entry point/handler is missing.
