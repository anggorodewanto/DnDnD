# task crit-01c — Workflow handlers + wire existing-but-unwired Set methods

## Finding (from chunk4 + chunk7 cross-cutting "Slash-command stub gap")

> "Of the 28 game commands in `router.go:198`, only 15 have real handlers. The 13 stubs include the *primary mechanical actions of every spell/feature in this chunk*: `attack`, `cast`, `bonus`, `shove`, `interact`, `deathsave`, ..., `prepare`, `retire`, `undo`, `character`, `help` ..."

> chunk7 high-15: "/help is a stub in production. NewHelpHandler / SetHelpHandler defined but never called outside tests."

After batches C-1 + C-2 closed: combat handlers (/attack /bonus /shove /interact /deathsave) and spell handlers (/cast /prepare /action ready) are now wired.

This task closes the rest:

**A. Wire SetXxxHandler in `cmd/dndnd/discord_handlers.go` for handlers whose constructors already exist** (these all have `Set*Handler` methods on `CommandRouter` and live handler files in `internal/discord/`):
- `/help` → `SetHelpHandler` + `HelpHandler`
- `/equip` → `SetEquipHandler` + `EquipHandler`
- `/inventory` → `SetInventoryHandler` + `InventoryHandler`
- `/give` → `SetGiveHandler` + `GiveHandler`
- `/attune` → `SetAttuneHandler` + `AttuneHandler`
- `/unattune` → `SetUnattuneHandler` + `UnattuneHandler`
- `/character` → `SetCharacterHandler` + `CharacterHandler`
- ASI/Feat component callbacks → `SetASIHandler` + `ASIHandler`

**B. Build NEW handlers** for two stubbed commands without existing implementations:
- `/undo <reason>` — player asks the DM to undo their last action. Implementation: post a dm-queue event of kind "undo-request" with the player's character + reason + last-action-summary. No actual rollback (the DM uses the dashboard undo to perform the rollback). Just routes the request.
- `/retire <reason>` — player retires their character. Implementation: post a dm-queue request and on DM approval call `registration.Service.Retire` (which already exists at `internal/registration/service.go:174`). For this task, just route the request to dm-queue with kind "retire-request" — DM-side approval flow + Service.Retire wiring can be a separate task if not trivially obvious.

Spec: Phase 13, 14, 16, 84, 88a/b, 89d, 96/97a, 100b in `docs/phases.md`; "Player Onboarding" + "Inventory Management" + "Magic Items" + "Help System" in `docs/dnd-async-discord-spec.md`.

This task closes finding crit-01 entirely (combined with batches C-1 + C-2) and absorbs high-15 (/help wiring).

## Plan

1. Read existing handler constructors for /help, /equip, /inventory, /give, /attune, /unattune, /character, ASI to confirm signatures + dep contracts.
2. Wire all 8 in `cmd/dndnd/discord_handlers.go` via a new `attachInventoryAndCharacterHandlers` helper (mirroring `attachCombatActionHandlers`). Plumb the new optional deps (`levelUpService`, `dmQueueFunc`, `notifier`, `portalBaseURL`) through `discordHandlerDeps`.
3. Add `Set*Handler` calls for each new handler to `attachPhase105Handlers`, all guarded by nil checks.
4. Build two adapters in `discord_handlers.go`: `giveTargetResolverAdapter` (UUID-or-name match against `ListCharactersByCampaign`) and `asiServiceAdapter` (bridges `levelup.Service` ↔ `discord.ASIService`, translating between `ASIChoice` and `ASIChoiceData`).
5. Add a constant for `KindUndoRequest` and `KindRetireRequest` to `internal/dmqueue/format.go`, plus a label entry so the `FormatEvent` rendering works.
6. TDD-build `internal/discord/undo_handler.go` + `_test.go`. Handler resolves the player's combatant via the encounter resolver, summarises their last `action_log` row, and posts a `dmqueue.KindUndoRequest` event. Defensive about nil deps.
7. TDD-build `internal/discord/retire_handler.go` + `_test.go`. Handler looks up the player's character via the existing `InventoryCharacterLookup`, posts a `dmqueue.KindRetireRequest` event, and returns ephemeral confirmation.
8. Add `SetUndoHandler` and `SetRetireHandler` to `internal/discord/router.go` (mirroring the existing `Set*Handler` setters).
9. Wire both new handlers in `buildDiscordHandlers` + `attachPhase105Handlers`.
10. Plumb the new `discordHandlerDeps` fields through `cmd/dndnd/main.go` (uses already-built `levelUpSvc`, `dmQueueChannel`, `dmQueueNotifier`, `os.Getenv("BASE_URL")`).
11. Add integration tests in `cmd/dndnd/discord_handlers_test.go` asserting (a) /help is constructed without DB deps, (b) the inventory family is constructed when queries are present, (c) every wired command routes to a real handler (not the "not yet implemented" stub) when delivered to the router.
12. Run `go test ./internal/discord/... ./cmd/dndnd/...` then `make cover-check` and confirm thresholds met.
13. Append a follow-up note to `.fix-state/log.md` flagging that the dashboard's queue-resolve UI does not yet recognise `KindUndoRequest` / `KindRetireRequest` (Phase 97b dashboard undo + the existing `created_via="retire"` approval flow are the canonical performers, but they are not currently invoked from a queue resolve).

## Files touched

- `cmd/dndnd/discord_handlers.go` — added `levelUpService`, `dmQueueFunc`, `notifier`, `portalBaseURL` to `discordHandlerDeps`; added `help`, `inventory`, `equip`, `give`, `attune`, `unattune`, `character`, `asi`, `undo`, `retire` fields to `discordHandlers`; added `attachInventoryAndCharacterHandlers` helper; appended 11 new `Set*Handler` calls to `attachPhase105Handlers`; added `giveTargetResolverAdapter` + `asiServiceAdapter` adapters at the bottom of the file.
- `cmd/dndnd/discord_handlers_test.go` — added `TestBuildDiscordHandlers_HelpAlwaysConstructed`, `TestBuildDiscordHandlers_InventoryFamilyConstructedWithQueries`, `TestAttachPhase105Handlers_RegistersInventoryFamilyHandlers` (subtests for each of /help, /inventory, /equip, /give, /attune, /unattune, /character, /undo, /retire).
- `cmd/dndnd/main.go` — passed `levelUpSvc`, `dmQueueChannel`, `dmQueueNotifier`, `os.Getenv("BASE_URL")` into `buildDiscordHandlers`.
- `internal/discord/router.go` — added `SetUndoHandler` and `SetRetireHandler` setters.
- `internal/discord/undo_handler.go` — new file, `UndoHandler` posting a `dmqueue.KindUndoRequest` event with the player's last action summary.
- `internal/discord/undo_handler_test.go` — new file, 5 tests + `TestMostRecentActionFor` (4 subtests) covering the handler + helper.
- `internal/discord/retire_handler.go` — new file, `RetireHandler` posting a `dmqueue.KindRetireRequest` event.
- `internal/discord/retire_handler_test.go` — new file, 7 tests covering the handler.
- `internal/dmqueue/format.go` — added `KindUndoRequest` and `KindRetireRequest` constants, plus their label entries.
- `.fix-state/log.md` — appended worker note flagging the dashboard-side queue-resolve gap.

## Tests added

- `internal/discord/undo_handler_test.go`:
  - `TestUndoHandler_PostsToDMQueue` — happy path, verifies summary contains the most recent action description + reason.
  - `TestUndoHandler_NoEncounter_StillPosts` — degraded path when active encounter lookup fails.
  - `TestUndoHandler_NoNotifier_RespondsAnyway` — handler still confirms to player without dm-queue.
  - `TestUndoHandler_MissingReason_StillPosts` — empty reason becomes "(no reason given)".
  - `TestMostRecentActionFor` — 4 subtests covering description / action_type fallback / unknown / no match.
- `internal/discord/retire_handler_test.go`:
  - `TestRetireHandler_PostsToDMQueue` — happy path.
  - `TestRetireHandler_NoCampaign_StillResponds`.
  - `TestRetireHandler_NoCharacter_StillResponds`.
  - `TestRetireHandler_NoReason_DefaultsToPlaceholder`.
  - `TestRetireHandler_NoNotifier_StillRespondsConfirmation`.
  - `TestRetireHandler_NilCampaignProv_RespondsGracefully`.
  - `TestRetireHandler_NilCharacterLookup_RespondsGracefully`.
- `cmd/dndnd/discord_handlers_test.go`:
  - `TestBuildDiscordHandlers_HelpAlwaysConstructed`.
  - `TestBuildDiscordHandlers_InventoryFamilyConstructedWithQueries`.
  - `TestAttachPhase105Handlers_RegistersInventoryFamilyHandlers` (9 subtests, one per command).

## Implementation notes

- `/help` is the only handler with no deps — `NewHelpHandler(session)` is called unconditionally inside the helper. The other 8 handlers all require `*refdata.Queries`.
- The `*refdata.Queries` value already structurally satisfies `EquipCharacterStore`, `AttuneCharacterStore`, `GiveCharacterStore`, `CharacterLookup`, and `CampaignProvider` (verified via existing `regDeps.CampaignProv: queries` wiring in main.go). No new sqlc queries needed.
- `GiveTargetResolver` and `ASIService` did not have production adapters; both built fresh in this task. `giveTargetResolverAdapter` first attempts UUID parse, then case-insensitive name match across `ListCharactersByCampaign`. `asiServiceAdapter` translates `discord.ASIChoiceData` → `levelup.ASIChoice` and rebuilds the `ASICharacterData` snapshot from `GetCharacterForLevelUp` + `character.FormatClassSummary`.
- `GiveCombatProvider` is currently unused inside `GiveHandler.Handle` (no adjacency check today), so the production wiring passes `nil`. Once Phase 16 wires the in-combat adjacency rule, the resolver can be added without breaking the existing handler signature.
- `/undo` does not call any rollback API — it only posts a dm-queue event. The dashboard's Phase 97b undo handler (`internal/dashboard/dm_dashboard_undo.go`) is the canonical performer, but it is invoked from its own UI panel rather than from a queue-resolve action. Wiring the queue-resolve → dashboard-undo bridge is captured as a follow-up note in `.fix-state/log.md`.
- `/retire` does not call `registration.Service.Retire` — it only posts a dm-queue event. The dashboard already supports a `created_via="retire"` approval flow, but it consumes a separate "retire submission" card rather than the new dm-queue retire-request item. Same follow-up note.
- All new handlers use early-return style for nil-safety (matches `/home/ab/.claude/CLAUDE.md` rule).
- No simplify-pass collapses applied — the new code does not have 5+ identical lines repeating; each handler constructor and each `Set*Handler` registration uses a unique type, so the apparent "duplication" is structural and correct.
- Coverage: new handler files are at 100% / near-100% per-function; package-level coverage stayed at 89.7% (above the 85% threshold). `cmd/dndnd/discord_handlers.go` is on the existing exclusion list so the new wiring code does not lower the overall figure.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

- Part A: All 8 wired in `attachInventoryAndCharacterHandlers` + `attachPhase105Handlers` with real production deps (queries, session, levelUpSvc, dmQueueChannel, dmQueueNotifier, BASE_URL); /help is dependency-free and unconditional; ASI gated on levelUpService.
- Part B: `/undo` and `/retire` each have `Set*Handler` setters in `internal/discord/router.go`, dedicated handler files, post `KindUndoRequest`/`KindRetireRequest` to dm-queue, return ephemeral confirmation. Neither calls a rollback/retire API directly — both correctly only route the request (Phase 97b dashboard undo and `created_via="retire"` flow remain canonical performers; bridge gap noted in `.fix-state/log.md`).
- /undo last-action lookup correctness: combatant resolved via `combatantByDiscordAdapter.GetCombatantIDByDiscordUser` (encounter→campaign→player_character→combatant.character_id match) → `mostRecentActionFor` filters `ActionLog.ActorID == playerCombatantID`, so DM/NPC/other-player rows are skipped. Verified against `refdata.ActionLog{ActorID uuid.UUID}`.
- Tests: `TestAttachPhase105Handlers_RegistersInventoryFamilyHandlers` runs 9 subtests (one per command), routes a real interaction through the production router, and asserts responses contain neither "not yet implemented" nor "Unknown command" — proves stub replacement. `TestUndoHandler_PostsToDMQueue` asserts the most recent action description appears in the summary.
- dmqueue kinds: `KindUndoRequest`/`KindRetireRequest` follow existing `Kind` + PascalCase naming with snake_case wire values; both have `kindLabels` entries (⏪ "Undo Request", 🪦 "Retire Request").
- Adapters: `giveTargetResolverAdapter` (UUID-then-name, returns `sqlNoRowsLike()`) and `asiServiceAdapter` (translates `ASIChoiceData`↔`levelup.ASIChoice`, builds `ASICharacterData` snapshot) are minimal and purpose-built — no premature abstraction.
- Scope: dashboard queue-resolve→undo/retire bridge correctly deferred (worker note in `log.md:36`).
