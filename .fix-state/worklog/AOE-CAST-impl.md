# AOE-CAST bundle — implementation worklog

Bundle: E-59 / E-63 / E-66b (3 HIGH/MEDIUM tasks). Date: 2026-05-11.
Implementer: opus-4.7.

## Task status

### E-66b-cast-extended-flag — DONE

- Added the missing `extended` boolean option to the `/cast` slash command
  definition in `internal/discord/commands.go`. The handler already read
  `extended` from `metamagicFlags`, so wiring is a single option entry.
- New tests:
  - `TestCommandDefinitions_ParameterHints` row asserting the option exists.
  - `TestCastHandler_ForwardsExtendedMetamagic` asserting the handler now
    forwards `Metamagic: ["extended"]` to `combat.CastCommand`.

### E-63-material-component-prompt — DONE

- `dispatchSingleTarget` now inspects `CastResult.MaterialComponent`. When
  `NeedsGoldConfirmation` is true the handler renders a Buy & Cast / Cancel
  prompt via `ReactionPromptStore` (`promptMaterialFallback`). Clicking
  Buy & Cast re-invokes `combat.Service.Cast` with `GoldFallback=true`;
  Cancel / timeout post a no-cast log line and the slot stays. Without a
  prompt store wired, the gold-fallback message is surfaced as a plain
  ephemeral so the caster knows the cast didn't go through.
- New `CastHandler.SetMaterialPromptStore` + `HandleComponent` so the
  router can route button clicks through the existing prompt-store
  prefix (no new namespace required).
- Tests:
  - `TestCastHandler_MaterialComponent_PromptsGoldFallback`
  - `TestCastHandler_MaterialComponent_BuyAndCastRetriesWithGoldFallback`
  - `TestCastHandler_MaterialComponent_CancelDoesNotRetry`
  - `TestCastHandler_MaterialComponent_NoPromptStoreFallsBackToEphemeral`
  - `TestCastHandler_HandleComponent_NoPromptStoreReturnsFalse`

### E-59-aoe-pending-saves — DONE (player-side flow; DM-side hook in place)

- `Service.CastAoE` now persists one `pending_saves` row per affected
  combatant with `source = "aoe:<spell-id>"` (constants
  `AoEPendingSaveSourcePrefix` / `AoEPendingSaveSource` /
  `IsAoEPendingSaveSource` / `SpellIDFromAoEPendingSaveSource`).
- `Service.RecordAoEPendingSaveRoll(combatantID, ability, total, autoFail)`
  finds the matching pending row, writes (total, total>=row.Dc & !autoFail)
  via `UpdatePendingSaveResult`, returns the spell ID + a "resolved any
  row?" bool.
- `Service.ResolveAoEPendingSaves(encounterID, spellID, roller)` is the
  fan-in hook: it inspects every row tagged for the spell; if all are
  resolved it parses the spell's damage JSON and drives the existing
  `ResolveAoESaves` pipeline. While any row is still pending it returns
  `(nil, nil)` so the resolver can be called eagerly from both the player
  /save path and the future DM dashboard.
- `internal/discord/cast_handler.go::dispatchAoE` posts a per-player /save
  reminder in the combat-log channel for each affected non-NPC combatant
  (`postAoESavePrompts`). NPC saves are routed via the DM dashboard.
- `internal/discord/save_handler.go` gained an `AoESaveResolver` interface
  plus the `AoESaveServiceAdapter` that wraps `combat.Service` + a roller
  so /save can call `RecordAoEPendingSaveRoll` then
  `ResolveAoEPendingSavesForSpell` once a player rolls. Best-effort: any
  wiring gap is a no-op so /save still works without the resolver.
- Tests:
  - `TestCastAoE_PersistsPendingSavesForEachAffectedCombatant`
  - `TestResolveAoEPendingSaves_AppliesDamageOnceAllResolved`
  - `TestResolveAoEPendingSaves_NoopWhenPendingRemain`
  - `TestRecordAoEPendingSaveRoll_ResolvesMatchingRow`
  - `TestRecordAoEPendingSaveRoll_AutoFailMarksFailure`
  - `TestRecordAoEPendingSaveRoll_NoMatchingRowIsNoop`
  - `TestAoEPendingSaveSourceHelpers`
  - `TestCastHandler_AoEDispatch_PromptsAffectedPlayersToSave`
  - `TestSaveHandler_RecordsAndResolvesAoEPendingSaves`
  - `TestSaveHandler_NoAoEPendingSave_SkipsResolution`
  - `TestAoESaveServiceAdapter_Forwards`

## DM-dashboard route-back (deferred)

The DM dashboard side (NPC save resolution) is in `internal/dashboard/*`
which is out of zone for this bundle. The resolver hook
(`ResolveAoEPendingSaves`) is designed to be called from BOTH the /save
path AND a future dashboard PATCH endpoint that resolves NPC pending
saves — once that endpoint lands, the existing damage-application loop
fires automatically when the last NPC save comes back.

## Verification

- `go test ./internal/combat/ ./internal/discord/` — green.
- `make test` — green.
- `make cover-check` — coverage thresholds met. Combat 93.08%, Discord 87.13%.
- `make build` — clean.
- `go vet ./...` — clean.

## Files touched

- `internal/discord/commands.go` — added `extended` slash option.
- `internal/discord/cast_handler.go` — material-component prompt + AoE
  per-player /save prompt + `HandleComponent` routing.
- `internal/discord/cast_handler_test.go` — new tests; injected
  `ChannelID` on the helper interaction.
- `internal/discord/commands_test.go` — added `extended` parameter-hint row.
- `internal/discord/save_handler.go` — `AoESaveResolver` interface,
  `AoESaveServiceAdapter`, `SetAoESaveResolver`, `maybeResolveAoESave`
  hook in `Handle`.
- `internal/discord/save_handler_test.go` — three new tests, including
  the adapter round-trip.
- `internal/combat/aoe.go` — `AoEPendingSaveSourcePrefix` constants,
  pending-row persistence inside `CastAoE`,
  `ResolveAoEPendingSaves` + `RecordAoEPendingSaveRoll`.
- `internal/combat/aoe_test.go` — five new tests covering the persistence
  and resolution flows.
