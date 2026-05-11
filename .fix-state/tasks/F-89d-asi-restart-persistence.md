---
id: F-89d-asi-restart-persistence
group: F
phase: 89d
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# ASI/Feat pending choice lost on restart (in-memory only)

## Finding
Pending ASI/Feat choice state is stored in-memory (`sync.Map`-style mutex). A process restart loses pending state. Spec does not flag this explicitly but it is a durability gap noted in the audit.

## Code paths cited
- `internal/discord/asi_handler.go` — in-memory pending choice storage

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 89d) — interactive ASI/Feat selection

## Acceptance criteria (test-checkable)
- [ ] Pending ASI/Feat choice persists across process restart (DB-backed or equivalent durable store)
- [ ] On restart, in-flight prompts can still be approved/denied without re-prompting from scratch
- [ ] Test in `internal/discord/asi_handler_test.go` or `internal/levelup/` package_test.go fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None identified

## Notes
Audit flags this as a minor durability gap not flagged in spec.
