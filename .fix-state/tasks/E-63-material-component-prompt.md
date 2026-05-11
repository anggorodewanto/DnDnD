---
id: E-63-material-component-prompt
group: E
phase: 63
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Material component gold-fallback prompt not surfaced in Discord

## Finding
`CastResult.MaterialComponent.NeedsGoldConfirmation=true` and `FormatGoldFallbackPrompt` exist on the service side, but `internal/discord/cast_handler.go` never reads `result.MaterialComponent` nor offers a "Buy & Cast" / "Cancel" prompt. Players cannot trigger the gold fallback via Discord — the cast just succeeds silently with `MaterialComponent` populated.

## Code paths cited
- internal/combat/spellcasting.go — `ValidateMaterialComponent`, `MaterialComponentResult`, `FormatMaterialRejection`, `FormatGoldFallbackPrompt`, `RemoveInventoryItem`, `AddInventoryItem`
- internal/discord/cast_handler.go — handler never inspects `result.MaterialComponent`

## Spec / phase-doc anchors
- docs/phases.md — Phase 63 ("Spell Casting — Material Components"); gold-fallback "Buy & Cast" / "Cancel" prompt

## Acceptance criteria (test-checkable)
- [ ] When `result.MaterialComponent.NeedsGoldConfirmation` is true, the Discord handler posts the gold-fallback prompt with "Buy & Cast" / "Cancel" buttons
- [ ] "Buy & Cast" deducts gold, adds/consumes the component, and completes the cast
- [ ] "Cancel" aborts the cast without spending slot or gold
- [ ] Existing rejection (no gold + no item) and consume-on-cast paths remain reachable
- [ ] Test in `internal/discord/cast_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-59-aoe-pending-saves, E-66b-cast-extended-flag (same handler file)

## Notes
Service layer is complete; the gap is purely the Discord prompt + button-callback wiring.
