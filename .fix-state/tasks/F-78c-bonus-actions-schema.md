---
id: F-78c-bonus-actions-schema
group: F
phase: 78c
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Add structured bonus_actions field to creature data model

## Finding
Phase 78c parses bonus actions at runtime by scanning each ability's description for "bonus action" (case-insensitive). The spec requires a structured `bonus_actions` field on the creature data model; currently `creatures` table only has an `abilities` JSONB column. Parsing happens at request time instead of being persisted.

## Code paths cited
- `internal/combat/turn_builder.go:464` — `ParseBonusActions` runtime scan
- `internal/combat/turn_builder_test.go:645+` — current tests
- `db/migrations/20260310120005_create_creatures_magic_items.sql:25` — only `abilities` JSONB, no `bonus_actions` column

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 78c) — "Add structured `bonus_actions` field to creature data model."

## Acceptance criteria (test-checkable)
- [ ] `creatures` table has a structured `bonus_actions` JSONB column (migration added)
- [ ] Turn builder reads bonus actions from the structured column rather than (or in addition to) parsing descriptions
- [ ] Existing Goblin Nimble Escape behavior preserved
- [ ] Test in `internal/combat/turn_builder_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None identified

## Notes
Spec letter requires a structured field on the data model; current implementation is functional but not structured.
