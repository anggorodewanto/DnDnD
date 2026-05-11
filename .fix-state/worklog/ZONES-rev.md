# ZONES review — Phase 67 + Phase 71 (6 tasks)

Reviewer: opus 4.7 (1M) — fresh context
Date: 2026-05-11
Branch: main (worktree dirty; changes still unstaged)

## Per-task verdict

| Task | Verdict | Notes |
| --- | --- | --- |
| E-67-silence-zone-type | PASS | `zone_definitions.go:108` flips `ZoneType` to `"silence"`; matches the filter in `concentration.go` (`combatantInSilenceZone` / `CheckSilenceBreaksConcentration`). Regression test `TestZoneDefinition_SilenceUsesSilenceType` guards. |
| E-67-zone-render-on-map | PASS | `discord_adapters.go:425-447` wires `ListEncounterZonesByEncounterID` into `mapRegeneratorAdapter.RegenerateMap` and populates `MapData.ZoneOverlays` via `zonesToRendererOverlays`. Two main_wiring tests assert build-from-rows + adapter render. `ZoneAffectedTilesFromShape`/`ZoneOriginIndex` exported as the minimal seam. |
| E-67-zone-anchor-follow | PASS | `service.go:558-564` invokes `UpdateZoneAnchor` immediately after the position write inside `UpdateCombatantPositionWithTriggers`. `TestUpdateCombatantPosition_InvokesZoneAnchorForFollowZones` asserts the origin moves to the new tile for Spirit Guardians. |
| E-67-zone-triggers | PASS | Enter: `service.go:580-586`. Start-of-turn: `initiative.go:705-710`. Round reset: `advanceRound` (`initiative.go:725-755`) wraps both `AdvanceTurn` round-tick branches and calls `ResetZoneTriggersForRound`. `TurnInfo.ZoneTriggerResults` surfaces results. Two integration tests cover. |
| E-67-zone-cleanup | PASS | `computeZoneExpiry` + `SpellDurationRounds` (spellcasting.go:751-836) set `ExpiresAtRound` on the Cast path; aoe.go:480-512 mirrors on AoE path. `CleanupExpiredZones` invoked from `advanceRound`; `EndCombat` routes through `CleanupEncounterZones` (single seam). Duration-table test + AdvanceTurn cleanup test pass. |
| E-71-readied-action-expiry | PASS (service side) | `createActiveTurn` invokes `expireReadiedActionsForTurn` (initiative.go:679-690); cancels declarations + clears concentration on readied spells. `TurnInfo.ExpiryNotices` surfaces. Two integration tests cover. Discord wiring (`/action ready` flags, `/status` formatter) correctly deferred to follow-up tasks listed in worklog. |

## Verification

- `make build` — PASS
- `make test` — PASS
- `make cover-check` — PASS (combat 93.1%; thresholds met)
- Batch-1 intact: `ConditionsForDying` still at `deathsave.go:240`; `applyDamageHP` signature unchanged at `concentration.go:320` (same 7-arg form); diff to concentration.go is unrelated comment/wiring tweaks only.

## Red-before-green sanity

- Anchor: remove the new `UpdateZoneAnchor` call in `UpdateCombatantPositionWithTriggers` → `updateEncounterZoneOriginFn` never invoked → assertion `Len(updated, 1)` fails.
- Triggers (enter): remove `CheckZoneTriggers(..., "enter")` → results empty → `Len(results, 1)` fails.
- Round reset: drop `advanceRound` wrapper → `resetAllTriggeredThisRoundFn` never called → `assert.True(t, resetCalled)` fails.
- Readied expiry: drop `expireReadiedActionsForTurn` → no `cancelReactionDeclaration` calls, empty `ExpiryNotices` → both readied-action tests fail.
- Render: drop `ListEncounterZonesByEncounterID` interface method → `mapRegeneratorAdapter` won't compile; the fake's `zones` map goes unused.

## Recommended next steps

1. Stage and commit ZONES bundle.
2. File-tracked follow-ups exist for the deferred Discord work: `E-67-followup-discord-zone-trigger-prompts.md`, `E-71-followup-discord-ready-spell-flags.md`, `E-71-followup-status-readied-actions.md`. Pick up in the next Discord-layer batch.
3. The worklog notes a future schema migration to promote `AmmoSpentTracker` to a column — unrelated to ZONES; track separately.
