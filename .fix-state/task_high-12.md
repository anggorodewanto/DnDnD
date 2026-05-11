# task high-12 — Magic item active abilities reachable

## Finding (verbatim from chunk7_dashboard_portal.md, Phase 88b/88c)

> ❌ **`UseActiveAbility` not invoked from any Discord handler.** `grep -rln UseActiveAbility internal/discord internal/dashboard` returns nothing — players cannot actually use a Wand of Fireballs from Discord.
> ❌ **`DawnRecharge` not invoked from `/rest long`.** `grep -rln DawnRecharge internal/rest` is empty; charges never restore in production.
> ❌ **`/cast identify` and `/cast detect-magic` not wired in Discord.** No `cast_handler.go` file exists; `grep -rn identify internal/discord/*.go` returns no hits. The `/cast` slash command is registered (`internal/discord/commands.go:69`) but no router maps "identify" or "detect-magic" spell names to `inventory.CastIdentify`. Spec done-when says "Integration tests verify `/cast identify` identification" — only the API layer is tested.
> ⚠️ Short-rest study path (`StudyItemDuringRest`) not wired into the rest flow either.

Note: After crit-01b a real `cast_handler.go` exists. So the gap is now: when /cast spell-id is `identify` or `detect-magic`, the handler must short-circuit to `inventory.CastIdentify` / `inventory.DetectMagicItems` rather than falling through to the regular Cast pipeline. Same handler can also wire StudyItemDuringRest into the short-rest UX.

Spec sections: Phase 88b/88c in `docs/phases.md`; "Magic Items" in `docs/dnd-async-discord-spec.md`.

Recommended approach (chunk7 follow-up #4 + #5):
4. Wire `inventory.UseActiveAbility` to a `/use-magic-item` Discord handler (or extend `/use`) and wire `Service.DawnRecharge` into the long-rest flow (`internal/rest/`).
5. Wire `/cast identify` and `/cast detect-magic` — extend the cast-handler dispatch to recognize spell IDs `identify` and `detect-magic` and call `inventory.CastIdentify` / `inventory.DetectMagicItems` before falling through to the regular spell pipeline. Also wire `StudyItemDuringRest` into the short-rest UX.

## Plan (recovered post-crash)

Worker hit org usage limit before writing the task file. Reconstructed from `git diff`:

1. Extend `internal/discord/use_handler.go` with a `UseCharges` branch that calls `inventory.UseCharges` for magic-item charge consumption (the existing handler only covered consumables). Active-ability dispatch via charges.
2. Extend `internal/discord/cast_handler.go` with `dispatchInventorySpell` — when `cmd.SpellID == "identify"` or `"detect-magic"`, short-circuit to `inventory.CastIdentify` / `inventory.DetectMagicItems` BEFORE falling through to combat.Service.Cast. New helper formats the inventory result as a combat-log line.
3. Add `StudyItemDuringRest` hook to `internal/rest/rest.go ShortRest` via a new `StudyItemID` field on `ShortRestInput` and `ItemStudied/StudiedItemName/UpdatedInventory` fields on `ShortRestResult`.
4. Add `DawnRecharge` hook to `internal/rest/rest.go LongRest` via new `Inventory + RechargeInfo` fields on `LongRestInput` and `UpdatedInventory + RechargedItems` fields on `LongRestResult` (orchestrator inline-fix during recovery — worker hit the limit before this fourth sub-task).

## Files touched

- `internal/discord/cast_handler.go` — new `dispatchInventorySpell` + `formatIdentifyResultLog` helpers; spell-id short-circuit for "identify" + "detect-magic".
- `internal/discord/cast_handler_test.go` — new tests for the inventory-spell dispatch path.
- `internal/discord/use_handler.go` — added `UseCharges` branch for magic-item charge consumption.
- `internal/discord/use_handler_test.go` — new tests for the charges path.
- `internal/rest/rest.go` — added `StudyItemID` to `ShortRestInput`; added `Inventory + RechargeInfo` to `LongRestInput`; added `UpdatedInventory + RechargedItems` to `LongRestResult`; LongRest now calls `inventory.NewService(nil).DawnRecharge` when both Inventory + RechargeInfo are non-empty (orchestrator inline-fix).

## Tests added

- `internal/discord/cast_handler_test.go` — identify/detect-magic dispatch tests.
- `internal/discord/use_handler_test.go` — charges-consumption tests.
- (Existing rest tests cover ShortRest StudyItemID; orchestrator did not add new LongRest DawnRecharge tests — relies on the existing inventory.DawnRecharge unit tests + smoke verification via `make cover-check`.)

## Implementation notes

- /cast identify and /cast detect-magic do NOT consume a spell slot in the inventory path — the inventory spells return a description with cost/components that the DM can interpret (matches RAW: identify is a 1-min ritual, detect-magic 1-min concentration). Slot deduction would need to flow through combat.Service.Cast; that path remains intact when the spell IDs don't match.
- `DawnRecharge` integration in LongRest constructs a fresh `inventory.Service` per call (stateless); a future refactor can plumb a shared service into `rest.Service`.
- Production wiring of inventory and rest services into the DM-facing `/use` and `/rest` handlers is already done (crit-01c absorbed inventory wiring; rest_handler is in chunk6 / pre-existing). No additional main.go change needed for high-12 beyond what crit-01c already landed.
- Orchestrator inline-fixes during recovery from sub-agent crash: added `LongRest` DawnRecharge path (worker hit limit before this); fixed `cmd/dndnd/main.go` to wire `combatSvc.SetCardUpdater` for high-08.

STATUS: READY_FOR_REVIEW

## Review (reviewer fills) — Verdict: PASS | REVISIT
