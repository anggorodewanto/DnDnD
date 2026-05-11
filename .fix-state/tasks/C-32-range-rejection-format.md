---
id: C-32-range-rejection-format
group: C
phase: 32
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# FormatRangeRejection helper defined+tested but unused

## Finding
`FormatRangeRejection` in `internal/combat/distance.go` is defined and unit-tested but never called in production. The actual out-of-range error returned from `attack.go:426` already contains both distance and max range, so the spec is functionally satisfied via a different code path. Cosmetic-only inconsistency.

## Code paths cited
- `internal/combat/distance.go` — `FormatRangeRejection` defined
- `internal/combat/distance_test.go` — only consumer
- `internal/combat/attack.go:426` — actual rejection path (returns raw "out of range: Xft away (max Yft)")

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 32 distance awareness)
- `.review-state/group-C-phases-29-43.md` Phase 32 findings

## Acceptance criteria (test-checkable)
- [ ] Either: `attack.go:426` (and any other range-rejection callers) routes through `FormatRangeRejection` for a consistent user-facing string; OR `FormatRangeRejection` is deleted with tests, with rationale captured in a comment
- [ ] No regression in existing range-rejection user-visible text
- [ ] Test in `internal/combat/attack_test.go` (or `distance_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None

## Notes
Pure cleanup. Prefer routing-through over deletion so the helper has at least one production caller.
