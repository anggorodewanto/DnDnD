---
id: H-104c-public-levelup-deferred
group: H
phase: 104c
severity: MINOR
status: deferred-with-justification
owner: lifecycle
reviewer:
last_update: 2026-05-11
---

# Public-channel level-up announcement is a no-op (deferred)

## Finding
`notifierAdapter.SendPublicLevelUp` is intentionally a no-op, while the private DM path works. A public-channel level-up announcement is acknowledged deferred work in the existing comment; the audit flags this so it isn't lost. Per the audit, the gap "falls outside Phase 104c scope" but the deferred behavior should still be tracked.

## Code paths cited
- `internal/levelup/notifier_adapter.go` — `SendPublicLevelUp` no-op implementation
- `internal/levelup/store_adapter.go` — companion adapters
- `internal/levelup/handler.go` — handler that would emit a public announcement
- `cmd/dndnd/main.go:735-749` — wiring site

## Spec / phase-doc anchors
- `docs/phases.md:632-829` (Group H, Phase 104c)
- Phase 104c scope: "Mount `levelup.Handler` with DB Store Adapter"

## Acceptance criteria (test-checkable)
- [ ] `SendPublicLevelUp` posts a public-channel announcement (story/combat channel as appropriate) when a character levels up
- [ ] Behavior is covered by a test in `internal/levelup/<package>_test.go` that asserts the public-channel post fires
- [ ] Test in `internal/levelup/<package>_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None in Group H. The notifier adapter is exclusively a level-up surface.

## Notes
Audit explicitly says this "falls outside Phase 104c scope" — confirm whether this should be re-scoped under a new phase or treated as a follow-up before implementing. Severity MINOR because private DM path already informs the player.
