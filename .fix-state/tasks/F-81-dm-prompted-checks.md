---
id: F-81-dm-prompted-checks
group: F
phase: 81
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# DM-prompted checks: no handler/endpoint to prompt a player to roll

## Finding
There is no explicit handler/endpoint that creates a check prompt for a player to roll. Saves use `pending_saves`; checks have no analogous prompt flow. DM cannot ask a specific player for a skill/ability check.

## Code paths cited
- `internal/check/check.go` — service exists, no prompt persistence
- `internal/discord/check_handler.go` — no DM-prompt entry point
- (no `pending_checks` table or analogous mechanism)

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 81) — DM-prompted check flow

## Acceptance criteria (test-checkable)
- [ ] DM can prompt a player for a check; player receives an interactive prompt
- [ ] Player response routes back through `check.SingleCheck` (or appropriate variant) and posts result
- [ ] Persisted state survives restart (analogous to `pending_saves`)
- [ ] Test in `internal/check/` or `internal/discord/` package_test.go fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-81-targeted-check-handler (same package)
- F-81-group-check-handler (same package)

## Notes
Analog to the existing `pending_saves` mechanism for saving throws.
