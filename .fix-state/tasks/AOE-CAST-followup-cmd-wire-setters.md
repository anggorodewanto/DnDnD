---
id: AOE-CAST-followup-cmd-wire-setters
group: E
phase: 59,63
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: wire AOE-CAST setters in cmd/dndnd

## Finding
AOE-CAST bundle added two new optional setters that need production wiring in `cmd/dndnd/discord_handlers.go`:

1. `CastHandler.SetMaterialPromptStore(...)` — required for E-63 Buy & Cast gold-fallback prompt. Without it, the prompt degrades to plain ephemeral and player can't confirm.
2. `SaveHandler.SetAoESaveResolver(...)` — required for E-59 `/save` to resolve AoE pending_saves rows. Without it, AoE saves are persisted but never resolved → damage never applies.

Both bundles' service-level closure is correct; only the production-handler wiring is missing.

## Code paths cited
- `internal/discord/cast_handler.go` — `SetMaterialPromptStore` setter.
- `internal/discord/save_handler.go` — `SetAoESaveResolver` setter + `AoESaveServiceAdapter`.
- `cmd/dndnd/discord_handlers.go` — handler construction site needing the two setter calls.

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md Phase 59 (AoE save flow) + Phase 63 (material component cost gate)

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/discord_handlers.go` calls both `SetMaterialPromptStore` and `SetAoESaveResolver` with concrete implementations
- [ ] e2e test confirms `/cast` of a high-cost material spell prompts before slot consumption
- [ ] e2e test confirms `/cast fireball` followed by all-players `/save dex` applies damage end-to-end
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- `cmd/dndnd/discord_handlers.go` is the same file `C-DISCORD-followup-cmd-wire-setters` will edit. Bundle the two wiring follow-ups into one cmd/dndnd implementer pass to minimize churn.

## Notes
Surfaced by AOE-CAST reviewer; out of zone for the bundle.
