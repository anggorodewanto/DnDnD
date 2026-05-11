---
id: F-86-item-picker-narrative-price
group: F
phase: 86
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Item picker: no narrative description / price override on picker endpoint

## Finding
The picker endpoint itself is bare: no "narrative description field" or "price override" capability. Those fields exist downstream (loot.AddItemRequest.Description, shops.CreateShopItemParams.PriceGp) but the shared picker component does not surface them per spec.

## Code paths cited
- `internal/itempicker/handler.go` — picker endpoints
- `internal/itempicker/routes.go` — picker routes

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 86) — picker as a "shared component"

## Acceptance criteria (test-checkable)
- [ ] Picker accepts and returns optional narrative description and price override fields
- [ ] Downstream consumers receive the fields without manual re-entry
- [ ] Test in `internal/itempicker/handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-86-item-picker-homebrew-flag (same package)
- F-86-item-picker-custom-entry (same package)

## Notes
Practical impact is limited because dashboard composes the form client-side, but the shared component contract is incomplete per spec.
