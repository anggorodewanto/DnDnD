---
id: F-86-item-picker-homebrew-flag
group: F
phase: 86
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Item picker search: no homebrew flag exposed or filterable

## Finding
The item-picker search endpoint does not surface or filter on the `homebrew` flag. Refdata has the column (`20260407120000_add_homebrew_to_refdata.sql`) but the picker neither filters nor surfaces it in search results.

## Code paths cited
- `internal/itempicker/handler.go` — search handler
- `internal/itempicker/routes.go` — route registration
- `db/migrations/20260407120000_add_homebrew_to_refdata.sql` — homebrew column source

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 86) — item picker shared component surface

## Acceptance criteria (test-checkable)
- [ ] `GET /api/campaigns/{id}/items/search` returns a `homebrew` boolean per result
- [ ] Optional query param to filter by homebrew (e.g. `?homebrew=true|false`) works
- [ ] Test in `internal/itempicker/handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-86-item-picker-custom-entry (same package)
- F-86-item-picker-narrative-price (same package)

## Notes
Refdata column exists; only picker surface needs update.
