# E-67-followup-zone-prompt-callsites — implementer worklog

## Scope
Wire `PostZoneTriggerResultsToCombatLog` at the turn-advance call site so
start-of-turn zone effects (Spirit Guardians, Wall of Fire, Moonbeam, etc.)
land in `#combat-log` for the DM to resolve.

## Files touched
- `internal/discord/done_handler.go` — added the one-shot call in
  `sendTurnNotifications` right after the auto-skip fan-out, using the
  already-resolved `ctx`, `h.session`, `h.campaignSettingsProvider`,
  `encounterID`, `nextCombatant`, and `turnInfo.ZoneTriggerResults`.
- `internal/discord/done_handler_new_test.go` — two new TDD tests:
  - `TestDoneHandler_PostsZoneTriggerResultsToCombatLog` (RED → GREEN):
    `combat.TurnInfo{ZoneTriggerResults: [{SourceSpell:"Spirit Guardians",
    Effect:"damage", Trigger:"start_of_turn"}]}` → asserts that a
    `chan-cl` channel send contains both the spell name and the next-up
    combatant's display name.
  - `TestDoneHandler_NoZoneTriggerPostWhenEmpty` (regression guard):
    asserts no "Zone effects" header is ever posted when the slice is
    empty so the helper's empty-result short-circuit is enforced from
    the call site.

## Move-handler call site (deferred)
`internal/discord/move_handler.go` calls
`MoveService.UpdateCombatantPosition` (not `…WithTriggers`) and the
interface returns only `(refdata.Combatant, error)` — no
`ZoneTriggerResult` slice is surfaced through the move path today.
Plumbing trigger results into the Discord move flow would require
either changing the `MoveService.UpdateCombatantPosition` signature or
adding a sibling method on the interface plus an adapter in
`cmd/dndnd/main.go`. Both expansions live outside this task's edit zone
(`internal/combat/*`, `cmd/dndnd/*` are read-only here), so per the
task's hard rule "only wire what's available without expanding scope"
the move-handler hook is left for a fresh follow-up.

Recommended follow-up scope: extend `MoveService` (or add a
`MoveZoneTriggerResolver` hook) so the existing `Service.
UpdateCombatantPositionWithTriggers` results can flow through to
`HandleMoveConfirm` / `HandleMoveConfirmWithMode` and the exploration
move, then call `PostZoneTriggerResultsToCombatLog` once at each of
those three call sites.

## Verification
- `go test ./internal/discord/` — passes (RED before edit, GREEN after).
- `make test` — full suite passes.
- `make cover-check` — overall + per-package thresholds hold.
- `make build` — clean.

## Status
Done-handler call site wired and covered. Move-handler call site
intentionally deferred behind a wider plumbing change; see notes above.
