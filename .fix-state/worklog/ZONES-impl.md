# ZONES impl — Phase 67 spell zones + Phase 71 readied-action expiry

Implementer: opus 4.7 (1M)
Date: 2026-05-11
Branch: main

## Scope

Phase 67 spell-zone cluster + Phase 71 readied-action expiry (6 tasks).

## Task status

### E-67-silence-zone-type — DONE
- `internal/combat/zone_definitions.go:97-114` — changed Silence's `ZoneType` from `"control"` to `"silence"` so `combatantInSilenceZone` and `CheckSilenceBreaksConcentration` (both filter on `"silence"`) can see the zone. Silence now actually silences combatants in its area and breaks V/S concentration on entry.
- `internal/combat/zone_definitions_test.go` — fixed the table-driven test that expected "control", added `TestZoneDefinition_SilenceUsesSilenceType` regression guard.

### E-67-zone-render-on-map — DONE
- `cmd/dndnd/discord_adapters.go:369-380` — added `ListEncounterZonesByEncounterID` to the narrow `mapRegeneratorQueries` interface.
- `cmd/dndnd/discord_adapters.go:425-447` — `RegenerateMap` now loads zones and populates `MapData.ZoneOverlays` so `DrawZoneOverlays` paints them.
- `cmd/dndnd/discord_adapters.go:503-561` — new `zonesToRendererOverlays` + `parseHexRGBA` helpers convert `refdata.EncounterZone` rows into `renderer.ZoneOverlay` records (semi-transparent fill + marker icon).
- `internal/combat/zone.go:303-340` — exported `ZoneAffectedTilesFromShape` / `ZoneOriginIndex` for cross-package use.
- `cmd/dndnd/main_wiring_test.go` — `TestZonesToRendererOverlays_BuildsOverlaysFromZones` + `TestMapRegeneratorAdapter_RendersWithZoneOverlays` validate the wiring.

### E-67-zone-anchor-follow — DONE
- `internal/combat/service.go:522-585` — `UpdateCombatantPosition` is now a thin wrapper around the new `UpdateCombatantPositionWithTriggers` which (a) calls `UpdateZoneAnchor` after the position update so combatant-anchored zones (Spirit Guardians, Aura of Protection) follow the caster on `/move`, and (b) returns `[]ZoneTriggerResult` for zone-entry effects.
- `internal/combat/zones_integration_test.go::TestUpdateCombatantPosition_InvokesZoneAnchorForFollowZones` — verifies the Spirit Guardians origin updates to the new tile.

### E-67-zone-triggers — DONE
- `internal/combat/service.go:545-590` — `UpdateCombatantPositionWithTriggers` invokes `CheckZoneTriggers(..., "enter")` after the position update and returns the results.
- `internal/combat/initiative.go:678-708` — `createActiveTurn` invokes `CheckZoneTriggers(..., "start_of_turn")` and surfaces results via `TurnInfo.ZoneTriggerResults`.
- `internal/combat/initiative.go:730-758` — new `advanceRound` helper centralises round-tick hooks; both round-advance branches in `AdvanceTurn` now call it, which fires `ResetZoneTriggersForRound` so the per-round dedupe map resets each round.
- `internal/combat/initiative.go:344-360` — extended `TurnInfo` with `ExpiryNotices` and `ZoneTriggerResults` (consumed by discord layer).
- `internal/combat/zones_integration_test.go::TestUpdateCombatantPosition_FiresEnterTriggers` and `TestAdvanceTurn_RoundAdvanceTriggersZoneCleanupAndReset` cover the wiring.

DEFERRED to follow-up: the actual damage application / DM-save prompt from a zone trigger lives in the Discord+dashboard layer (out of zone for this batch). New file: `.fix-state/tasks/E-67-followup-discord-zone-trigger-prompts.md`.

### E-67-zone-cleanup — DONE
- `internal/combat/spellcasting.go:751-836` — added `computeZoneExpiry(ctx, encounterID, spell)` + `SpellDurationRounds(duration)` to translate spell duration strings ("1 round", "10 rounds", "Concentration, up to 1 minute", etc.) into `currentRound + N`. `maybeCreateSpellZone` now sets `ExpiresAtRound` on every auto-created Cast-path zone.
- `internal/combat/aoe.go:480-512` — same `computeZoneExpiry` wiring on the AoE auto-create path (Fog Cloud, Darkness, Wall of Fire, etc.).
- `internal/combat/initiative.go::advanceRound` — calls `CleanupExpiredZones` at every round tick.
- `internal/combat/service.go:787-795` — `EndCombat` now routes encounter-end cleanup through the `CleanupEncounterZones` service method (single seam).
- `internal/combat/zones_integration_test.go::TestSpellDurationRounds` + `TestAdvanceTurn_RoundAdvanceTriggersZoneCleanupAndReset` cover the wiring.

### E-71-readied-action-expiry — DONE (service side)
- `internal/combat/initiative.go:680-720` — `createActiveTurn` now calls `expireReadiedActionsForTurn` which (a) cancels any active readied actions belonging to the combatant whose turn is starting, (b) clears the held concentration when the expired readied action was a spell, and (c) surfaces the notice strings via `TurnInfo.ExpiryNotices` so the discord turn-start notifier can compose `FormatTurnStartPromptWithExpiry`.
- `internal/combat/zones_integration_test.go::TestCreateActiveTurn_ExpiresReadiedActions` and `TestCreateActiveTurn_ReadiedSpellExpiryClearsConcentration` cover the wiring.

DEFERRED to follow-ups (discord/* out of zone):
- `.fix-state/tasks/E-71-followup-discord-ready-spell-flags.md` — `/action ready` needs `spell:` + `slot:` options so the readied-spell path is reachable.
- `.fix-state/tasks/E-71-followup-status-readied-actions.md` — `/status` needs to call `FormatReadiedActionsStatus`.

## Verification

- `go test ./internal/combat/ -count=1 -short` — PASS
- `go test ./cmd/dndnd/ -count=1 -short` — PASS
- `make test` — PASS (full suite green)
- `make cover-check` — PASS (combat 93.1%, cmd/dndnd 64.2%, all thresholds met)
- `make build` — PASS

## Notes on simplification (post-/simplify pass)

- Round-tick hooks (`CleanupExpiredZones` + `ResetZoneTriggersForRound`) factored into a single `advanceRound` helper so the natural-advance and skip-everyone branches in `AdvanceTurn` stay DRY.
- `UpdateCombatantPosition` kept its old signature (one return + error) and now delegates to `UpdateCombatantPositionWithTriggers` so existing callers don't need an update; the new signature is opt-in for callers who want to surface zone-trigger prompts.
- `ZoneAffectedTilesFromShape` + `ZoneOriginIndex` exported only because the discord adapter needs them; service-internal callers still use the unexported `zoneAffectedTiles`.

## Files touched

- internal/combat/zone_definitions.go (+ _test.go)
- internal/combat/zone.go
- internal/combat/concentration.go (no change — already correct after E-67-silence-zone-type fix)
- internal/combat/service.go
- internal/combat/initiative.go
- internal/combat/spellcasting.go
- internal/combat/aoe.go
- internal/combat/service_test.go (nil-safe GetCombatant)
- internal/combat/zones_integration_test.go (new)
- cmd/dndnd/discord_adapters.go
- cmd/dndnd/main_wiring_test.go (zone fake + tests)
- .fix-state/tasks/E-67-followup-discord-zone-trigger-prompts.md (new)
- .fix-state/tasks/E-71-followup-discord-ready-spell-flags.md (new)
- .fix-state/tasks/E-71-followup-status-readied-actions.md (new)
