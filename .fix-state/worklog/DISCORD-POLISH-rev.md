# DISCORD-POLISH review

Verified worklog vs diff, source, and `make build/test/cover-check`.

## Verification

- `make build` clean. `make test` clean across all packages. `make cover-check` thresholds met; `internal/discord` at 86.51%.

## Per-task verdicts

### D-54-followup-stand-max-speed — APPROVED
`ActionSpeedLookup` added (action_handler.go:107). `dispatchStand` → `resolveStandSpeed` threads into `StandCommand.MaxSpeed`; falls back to 30 on nil/error. Tests: fallback, Halfling 25ft, lookup-error. Wired via `moveSizeSpeedAdapter.LookupWalkSpeed` + `SetSpeedLookup` in `buildDiscordHandlers`. `HasSpeedLookup()` wiring test green.

### D-56-followup-drag-tile-sync — APPROVE-WITH-LIMIT
`syncDragTargetsAlongPath` + `tileOneStepBack` (move_handler.go:786-833) persist each grappled target on the dragger's trailing tile after `HandleMoveConfirm`. Test asserts target Chebyshev ≤1 of D1 after A1→D1. The task AC says "updated at each intermediate step"; impl only syncs at endpoint. Trade-off acknowledged in worklog. 5ft invariant holds at landing.

### C-43-stabilize-followup-wis-modifier — APPROVED
`ActionMedicineLookup` added. `dispatchStabilize` rolls `d20 + medicineMod` vs DC10; surfaces modifier inline. Tests: WIS-18 prof auto-pass (9+6=15), WIS-8 amateur fail (9-1=8), lookup-error → +0. `moveSizeSpeedAdapter.LookupMedicineModifier` uses `character.SkillModifier` for PCs, creature.Skills.medicine → WIS-mod for NPCs.

### E-67-followup-zone-trigger-prompts — APPROVE-WITH-PARTIAL
`FormatZoneTriggerResults` + `PostZoneTriggerResultsToCombatLog` land with 6 tests. Confirmed `done_handler.go` untouched and `cmd/dndnd /move` handler has no call site. PARTIAL claim accurate. **Action**: file `E-67-followup-zone-prompt-callsites` before closing parent E-67.

### E-71-followup-ready-spell-flags — APPROVED
`/action` schema adds `spell` + `slot` options. `performReadyAction` threads into `ReadyActionCommand.SpellName`/`SpellSlotLevel`. Two tests + schema assertion green.

### E-71-followup-status-readied-actions — APPROVED (ALREADY SATISFIED)
`status_handler_test.go:348` (`TestStatusHandler_InCombat_ReadiedActions`) confirms `**Readied Actions:**` block renders. No code change needed.

### COMBAT-MISC-followup-cmd-zone-lookup — APPROVED
`HasZoneLookup()` on both handlers. `buildDiscordHandlers` calls `SetZoneLookup(deps.combatService)` on check + action. Two wiring tests green. `*combat.Service` satisfies both interfaces structurally.

## Verdict: APPROVED

Bundle solid. Open `E-67-followup-zone-prompt-callsites` for the deferred call-site wiring before closing parent E-67.
