---
id: A-14-portal-url-baseurl
group: A
phase: 14
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# `/create-character` portal URL should respect BASE_URL

## Finding
The `/create-character` portal URL is hard-coded to `https://portal.dndnd.app/create?token=…`; the production host name is fixed regardless of `BASE_URL`.

## Code paths cited
- `internal/discord/registration_handler.go:271` — hard-coded `https://portal.dndnd.app/create?token=…`.

## Spec / phase-doc anchors
- `docs/phases.md` phase 14

## Acceptance criteria (test-checkable)
- [ ] Portal URL emitted by `/create-character` is derived from `BASE_URL` (or equivalent config) rather than hard-coded
- [ ] Test in `internal/discord/registration_handler_test.go` (or equivalent) covers a non-default `BASE_URL` and fails before the fix, passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None obvious.

## Notes
Flagged as low severity / cosmetic in the doc.
