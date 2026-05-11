---
id: A-08-retire-approved-transition
group: A
phase: 8
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Allow approved → retired status transition

## Finding
Status transitions are only defined from `pending`, so retiring an already-`approved` character (the realistic case per spec) returns `invalid status transition`. The gap repeats in the dashboard approval store, where `validApprovalTransitions["approved"]` is missing.

## Code paths cited
- `internal/registration/service.go:40-47` — transitions defined only from `pending`; `Service.Retire` will return `invalid status transition` for the realistic case.
- `internal/dashboard/approval_store.go:22-29` — `validApprovalTransitions["approved"]` is missing.

## Spec / phase-doc anchors
- `docs/dnd-async-discord-spec.md:33-43` — requires retiring an approved character via `/retire` → DM approval → `status='retired'`.
- `docs/phases.md` phase 8

## Acceptance criteria (test-checkable)
- [ ] `Service.Retire` accepts a character currently in `approved` status and transitions it to `retired`
- [ ] `validApprovalTransitions["approved"]` includes `retired`
- [ ] Test in `internal/registration/service_test.go` (or equivalent) fails before the fix and passes after
- [ ] Test in `internal/dashboard/approval_store_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Files likely also touched by: `A-08-retire-created-via-schema`, `A-16-retire-approval-unreachable`

## Notes
Phase 16 retire branch (`approval_handler.go:248`) is only reachable once the `created_via` schema and transition gaps below are both fixed.
