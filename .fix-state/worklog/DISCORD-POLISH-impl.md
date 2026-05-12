# DISCORD-POLISH implementation worklog

Bundle: 7 follow-up tasks across action / move / status / combat-log / commands handlers + cmd/dndnd wiring. TDD throughout. All `make test`, `make build`, `make cover-check` green.

## Per-task status

### D-54-followup-stand-max-speed — DONE
- Added `ActionSpeedLookup` interface on `ActionHandler` (LookupWalkSpeed).
- `dispatchStand` now consults the wired lookup and falls back to 30ft on nil-lookup / error.
- Reused `moveSizeSpeedAdapter` in `cmd/dndnd/discord_handlers.go` (added `LookupWalkSpeed` method).
- Wired via `handlers.action.SetSpeedLookup(...)` in `buildDiscordHandlers`.
- Tests: `TestActionHandler_Dispatch_Stand_*` (fallback, Halfling 25ft, error path) + wiring test `TestBuildDiscordHandlers_ActionHandlerHasSpeedLookup`.

### D-56-followup-drag-tile-sync — DONE
- Added `syncDragTargetsAlongPath` + `tileOneStepBack` helpers in `move_handler.go`.
- After a successful `HandleMoveConfirm`, the handler invokes the wired drag lookup; for each grappled target, persists position to the dragger's prior tile (one step back along path) so the 5ft adjacency invariant holds.
- Best-effort: any lookup / persistence failure aborts silently.
- Test: `TestMoveHandler_Drag_MultiTilePath_SyncsTargetEachStep` walks a 3-tile A1→D1 path and asserts the grappled target lands within Chebyshev 1 of D1.
- Note: full per-step path animation deferred; single trailing-tile sync satisfies the spec invariant and reuses existing `CheckDragTargets`.

### C-43-stabilize-followup-wis-modifier — DONE
- Added `ActionMedicineLookup` interface (LookupMedicineModifier).
- `dispatchStabilize` now rolls `d20 + medicineMod` (was flat d20) and includes the modifier in the success / failure log lines.
- `moveSizeSpeedAdapter.LookupMedicineModifier` resolves WIS + Medicine proficiency + expertise for PCs via `character.SkillModifier`, falls back to creature.Skills.medicine / WIS-mod for NPCs.
- Tests: `TestActionHandler_Stabilize_HighWisProficient_AutoPasses` (d20=9 + 6 = 15 ≥ DC10), `..._LowWisAmateur_FailsAtDC10` (d20=9 + -1 = 8 < DC10), `..._LookupError_FallsBackToFlatRoll`. Plus wiring test.

### E-67-followup-discord-zone-trigger-prompts — PARTIAL (helper landed, end-to-end wiring deferred)
- Added `FormatZoneTriggerResults` + `PostZoneTriggerResultsToCombatLog` helpers in `combat_log.go` with `combat.ZoneTriggerResult` rendering and #combat-log post path.
- Tests: empty results, damage trigger, save trigger, multi-zone, nil-CSP no-op, happy-path post.
- **Deferred**: the call sites in the turn-advance flow (`done_handler.go`) and the `/move` entry-trigger path (`cmd/dndnd/discord_handlers.go` move handler) are outside this implementer's edit zone (`done_handler.go` not in the open-file list; cmd/dndnd restricted to zone-lookup wiring only). Helper is ready for a follow-up batch to invoke.

### E-71-followup-discord-ready-spell-flags — DONE
- Added `spell` (string) and `slot` (integer) options to the `/action` slash-command schema.
- `performReadyAction` now reads both via `optionString` / `optionInt` and threads them into `combat.ReadyActionCommand.SpellName` / `SpellSlotLevel`.
- Tests: `TestActionHandler_Ready_WiresSpellAndSlot`, `..._NoSpellLeavesFieldsEmpty`, and the schema assertion in `TestCommandDefinitions`.

### E-71-followup-status-readied-actions — ALREADY SATISFIED
- `StatusHandler.populateReactions` already splits `IsReadiedAction` declarations into `info.ReadiedActions`; `status.FormatStatus` renders them as `**Readied Actions:** "..."`. Verified by existing `TestStatusHandler_InCombat_ReadiedActions` and `TestFormatStatus_ReadiedActions`. No code change required.

### COMBAT-MISC-followup-cmd-zone-lookup — DONE
- Added `HasZoneLookup()` introspection on `CheckHandler` and `ActionHandler`.
- `buildDiscordHandlers` now calls `handlers.check.SetZoneLookup(deps.combatService)` and `handlers.action.SetZoneLookup(deps.combatService)` (combat.Service already exposes `ListZonesForEncounter`).
- Wiring tests: `TestBuildDiscordHandlers_{Check,Action}HandlerHasZoneLookup`.

## Files touched

Open zone:
- `internal/discord/action_handler.go` — speed/medicine/zone lookups, stand + stabilize rewrites, performReadyAction spell threading.
- `internal/discord/action_handler_test.go` — spell-aware interaction builder + Ready spell/slot tests.
- `internal/discord/action_handler_dispatch_test.go` — Stand speed-lookup tests + `errStub` helper.
- `internal/discord/remediation_test.go` — Stabilize WIS/medicine modifier tests.
- `internal/discord/move_handler.go` — `syncDragTargetsAlongPath` + `tileOneStepBack`.
- `internal/discord/move_handler_drag_test.go` — multi-tile drag-sync test + discordgo import.
- `internal/discord/check_handler.go` — `HasZoneLookup()` introspection.
- `internal/discord/combat_log.go` — `FormatZoneTriggerResults` + `PostZoneTriggerResultsToCombatLog`.
- `internal/discord/combat_log_test.go` — NEW; full coverage for the zone-trigger formatter.
- `internal/discord/commands.go` — `/action` spell + slot options.
- `internal/discord/commands_test.go` — schema assertion rows for spell/slot.

cmd/dndnd:
- `cmd/dndnd/discord_handlers.go` — SetZoneLookup, SetSpeedLookup, SetMedicineLookup wiring; LookupWalkSpeed + LookupMedicineModifier methods on moveSizeSpeedAdapter; parseProficienciesFromJSON / creatureMedicineMod local helpers.
- `cmd/dndnd/discord_handlers_wiring_test.go` — Has{Zone,Speed,Medicine}Lookup assertions for check + action handlers.

## Verification

- `make test`: green across all packages (no FAIL grep hits).
- `make build`: clean (bin/dndnd + bin/playtest-player both produced).
- `make cover-check`: thresholds met; `internal/discord` at 86.50% (above 85% per-package gate).

## Deferrals

- E-67 trigger surfacing end-to-end (consumer call sites in done_handler.go / cmd/dndnd /move handler) is the substantial remaining work. The combat-log formatter + post helper are ready; a small follow-up that invokes `PostZoneTriggerResultsToCombatLog(...)` from `sendTurnNotifications` and from `MoveHandler.HandleMoveConfirm` will close the parent task. Not done in this batch because both call sites live outside the open-edit list.
