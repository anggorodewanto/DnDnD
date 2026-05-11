---
id: F-86-item-picker-custom-entry
group: F
phase: 86
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Item picker: no custom entry endpoint for freeform name/desc/quantity/gold

## Finding
The item picker has no "custom entry" endpoint for freeform name/desc/quantity/gold. Spec calls out "custom entry (freeform name/desc/quantity/gold)".

## Code paths cited
- `internal/itempicker/handler.go` — only search + creature-inventories endpoints
- `internal/itempicker/routes.go` — no custom-entry route

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 86) — "custom entry (freeform name/desc/quantity/gold)"

## Acceptance criteria (test-checkable)
- [ ] Picker exposes an endpoint that accepts a freeform custom entry (name, description, quantity, gold)
- [ ] Endpoint returns a payload usable by downstream loot/shop/inventory consumers
- [ ] Test in `internal/itempicker/handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- F-86-item-picker-homebrew-flag (same package)
- F-86-item-picker-narrative-price (same package)

## Notes
Downstream consumers (loot.AddItemRequest, shops.CreateShopItemParams) already accept the required fields; the picker shared component is missing the surface.
