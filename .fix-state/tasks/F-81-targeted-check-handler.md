---
id: F-81-targeted-check-handler
group: F
phase: 81
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Targeted check: missing adjacency validation and action cost in combat

## Finding
Targeted check (e.g. `/check medicine target:AR` on a downed ally) does not deduct an action in combat and does not validate tile adjacency. Spec explicitly calls out both. Only contested-check pathway exists; non-contested targeted-check has no enforcement.

## Code paths cited
- `internal/check/check.go` — check service
- `internal/discord/check_handler.go` — `/check` handler including `handleContestedCheck`

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 81) — "adjacency validation, action cost in combat"

## Acceptance criteria (test-checkable)
- [ ] Non-contested targeted `/check` (e.g. medicine on adjacent downed ally) deducts an action when in combat
- [ ] Targeted check rejected when target is not adjacent (tile distance check)
- [ ] Out-of-combat targeted check unaffected (no action deduction)
- [ ] Test in `internal/check/check_test.go` or `internal/discord/check_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-81-group-check-handler (same package)
- F-81-dm-prompted-checks (same package)

## Notes
Affects only the targeted (non-contested) pathway. Contested checks already routed via `handleContestedCheck`.
