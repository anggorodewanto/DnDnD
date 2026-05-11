---
id: F-88c-detect-magic-environment
group: F
phase: 88c
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# /cast detect-magic only lists caster's inventory, missing environmental aura

## Finding
`/cast detect-magic` only lists items in the caster's own inventory. Spec says "reveals magical aura on nearby items" — there is no scan of environment items / dropped loot / NPC inventory / nearby PCs. Functional for personal inventory only; misses the environmental aura aspect.

## Code paths cited
- `internal/discord/cast_handler.go:382` — `dispatchDetectMagic`
- `internal/inventory/identification.go` — `inventory.DetectMagicItems(items)` only takes caster items

## Spec / phase-doc anchors
- `docs/phases.md` lines 401-525 (Phase 88c) — "reveals magical aura on nearby items"

## Acceptance criteria (test-checkable)
- [ ] `/cast detect-magic` scans environment items / dropped loot / nearby NPC inventory / nearby PC inventory
- [ ] "Nearby" semantics defined (radius / tile distance) consistent with other range-based scans
- [ ] Caster's own inventory still included
- [ ] Test in `internal/inventory/identification_test.go` or `internal/discord/cast_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None identified

## Notes
May require introducing a scan helper that aggregates items from multiple sources within range.
