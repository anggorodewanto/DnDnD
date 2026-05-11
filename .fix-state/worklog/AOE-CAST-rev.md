# AOE-CAST bundle — independent review

Reviewer: opus-4.7 (1m), 2026-05-11. Read-only.

## Verification

- `make build`: clean.
- `make test`: green (all packages ok).
- `make cover-check`: overall 92.57%; combat 92.99%; discord 86.96%; OK.
- Targeted `Cover|Zone|Rage|FogOfWar` tests still pass (batch-1/2 intact).

## Per-task verdicts

### E-59-aoe-pending-saves — APPROVED

- `Service.CastAoE` writes one `pending_saves` row per affected combatant with `source=aoe:<spell-id>` (aoe.go:450-466). Source helpers (`AoEPendingSaveSourcePrefix`, `AoEPendingSaveSource`, `IsAoEPendingSaveSource`, `SpellIDFromAoEPendingSaveSource`) are present and round-trip cleanly.
- Fan-in: `ResolveAoEPendingSaves` returns `(nil, nil)` while any row is `status="pending"`, otherwise parses `spell.Damage` and drives `ResolveAoESaves` — see covered tests `TestResolveAoEPendingSaves_AppliesDamageOnceAllResolved` and `_NoopWhenPendingRemain`.
- `RecordAoEPendingSaveRoll` matches oldest pending AoE row by `(combatant, ability)`, computes success canonically `(total>=Dc & !autoFail)`. Auto-fail + no-match no-op tested.
- `dispatchAoE` posts per-player `/save` reminders in combat-log channel for non-NPC affecteds (`postAoESavePrompts`). Test asserts NPC prompts are filtered out.
- `/save` resolver hook (`maybeResolveAoESave`) calls `RecordAoEPendingSaveRoll` → `ResolveAoEPendingSavesForSpell`. Adapter `AoESaveServiceAdapter` round-trips.

### E-63-material-component-prompt — APPROVED

- `dispatchSingleTarget` short-circuits on `result.MaterialComponent.NeedsGoldConfirmation` BEFORE logging success (cast_handler.go:258-262), avoiding a phantom log line. Slot is preserved because the service returned early.
- Buy & Cast retry re-invokes `combat.Service.Cast` with `GoldFallback=true`; Cancel/timeout post a no-cast log; tests cover all four branches plus no-prompt-store fallback.
- Buttons routed via the shared `rxprompt:` prefix through existing `ReactionPromptStore.HandleComponent`; new `CastHandler.HandleComponent` is a thin shim.

### E-66b-cast-extended-flag — APPROVED

- `extended` boolean option added to `/cast` in commands.go:184-189. Handler-side reader (`metamagicFlags`) already included `extended`. Param-hints test row and a forward-to-service test (`TestCastHandler_ForwardsExtendedMetamagic`) both present.

## Red-before-green

Removing the new pending-row persistence loop, the `dispatchAoE` save-prompt block, the material-prompt branch, or the `extended` option entry each break a dedicated new test. TDD pattern is consistent.

## Findings / next steps

1. NOT WIRED IN PRODUCTION: neither `CastHandler.SetMaterialPromptStore` nor `SaveHandler.SetAoESaveResolver` is called from `cmd/dndnd/discord_handlers.go`. Without these the gold-fallback prompt silently falls back to a plain ephemeral and `/save` never resolves AoE pending rows in prod. Worklog acknowledges this implicitly; follow-up task required.
2. NPC/DM-dashboard resolution is intentionally deferred; resolver is shaped to accept the future PATCH hook.
3. `ResolveAoEPendingSaves` treats forfeited rows as failed saves; document this on the dashboard contract before wiring.
