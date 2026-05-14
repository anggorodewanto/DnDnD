# Batch 15: DM queue, help, status, whisper, exploration (Phases 106a–110a)

## Summary

All eleven phases (106a–110a) are checked off in `docs/phases.md` and have substantial implementation. The DM-notification framework (`internal/dmqueue/`), the dashboard resolver pages (`internal/dashboard/dmqueue_page.go`), the slash-command handlers (`/help`, `/status`, `/whisper`, `/action`, `/reaction`, `/check`, `/use`, `/rest`, `/retire`, `/undo`), and the exploration package (`internal/exploration/`) are all wired through `cmd/dndnd/discord_handlers.go` + `cmd/dndnd/main.go`.

However, a **systematic correctness bug** exists in the direct-from-handler Post call sites: six handlers do not pass `Event.CampaignID`, so `dmqueue.PgStore.Insert` will fail with `"parse campaign id"` AFTER the Discord message has already been sent. This produces orphan messages in `#dm-queue` that cannot be resolved or cancelled (no row in `dm_queue_items` → `Get(itemID)` returns false → 404). Two spec event types (narrative teleport, level-up review) have framework-only support but no runtime caller.

The exploration mode lacks a real combat-transition flip — `/dashboard/exploration/transition-to-combat` returns a positions JSON but never calls `UpdateEncounterMode` or `StartCombat`. This is consistent with the deferred-to-combat-start design but the dashboard endpoint name overstates what it does.

## Per-phase findings

### Phase 106a — Core infrastructure & initial events

- Status: matches (with one cross-cutting wiring bug, see below).
- Key files:
  - `/home/ab/projects/DnDnD/db/migrations/20260411120000_create_dm_queue_items.sql` — table with the right columns (campaign_id, guild_id, channel_id, message_id, kind, player_name, summary, resolve_path, status, outcome, extra JSONB, timestamps) and the right indexes (campaign+status, channel+message uniq).
  - `/home/ab/projects/DnDnD/internal/dmqueue/notifier.go` — `Notifier.Post`/`Cancel`/`Resolve` + `Status` (pending/resolved/cancelled).
  - `/home/ab/projects/DnDnD/internal/dmqueue/format.go` — emoji/label table covers all spec event kinds plus undo/retire extras.
  - `/home/ab/projects/DnDnD/internal/dmqueue/store.go`, `pgstore.go` — Store interface + memory + pg implementations.
  - `/home/ab/projects/DnDnD/internal/dashboard/dmqueue_page.go`, `routes.go` — Resolve → link + dashboard handler.
  - `/home/ab/projects/DnDnD/internal/discord/rest_handler.go`, `use_handler.go`, `action_handler.go` (combat path via `internal/combat/freeform_action.go`) — initial event posters.
- Findings:
  - Cancel/Resolve edits correctly use `FormatCancelled` / `FormatResolved` to strip the Resolve link and render strike-through / ✅ + outcome, matching the spec sample messages.
  - `Notifier.Post` performs `Sender.Send` BEFORE `Store.Insert`. When `PgStore.Insert` fails (e.g. on empty CampaignID, see cross-cutting), the message is in Discord but unresolvable from the dashboard. Should either (a) reserve the row first, or (b) treat insert failure as fatal and delete the just-posted message.
  - Spec table line "Action cancelled" appears as a state transition, not a separate KindAction-Cancelled — implementation correctly models it via Cancel edit on the original message.

### Phase 106b — Remaining events & whisper replies

- Status: partial (enemy turn ready and whisper covered; narrative teleport is framework-only).
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/initiative.go` (lines 803–825) — `postEnemyTurnReady` fires on NPC turns and correctly threads CampaignID via `GetCampaignByEncounterID`.
  - `/home/ab/projects/DnDnD/internal/dmqueue/notifier.go` (lines 219–247) — `PostNarrativeTeleport` and `PostWhisper` helpers exist with proper CampaignID arguments.
  - `/home/ab/projects/DnDnD/internal/discord/whisper_handler.go` — production whisper poster.
  - `/home/ab/projects/DnDnD/internal/discord/direct_messenger.go` (DM delivery), `internal/dashboard/dmqueue_page.go` `HandleWhisperReply` (DM resolves from dashboard).
  - `/home/ab/projects/DnDnD/internal/dmqueue/format.go` `FormatSkillCheckNarrationFollowup` covers the cross-channel follow-up shape.
- Findings:
  - **No runtime caller of `PostNarrativeTeleport`.** Grep across `internal/discord/`, `internal/combat/`, `cmd/` finds zero call sites — only tests. The /cast handler does not detect "Teleport beyond current map" and route to dm-queue. Spec event "Narrative teleport" therefore never fires in production despite framework-level support. Mark partial.
  - Whisper end-to-end works: `WhisperHandler.Handle` → `Notifier.PostWhisper` (stashes Discord user id in `WhisperTargetDiscordUserIDKey`) → dashboard form → `ResolveWhisper` → `DirectMessenger.SendDirectMessage`. Standalone-per-whisper model matches spec.
  - Spec mentions "level-up review" only in the level-up flow (lines 2456, 2488); the implementation routes that via the existing approval queue (not dm-queue), which is acceptable but not explicitly cross-referenced.

### Phase 106c — /reaction wiring

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/reaction_handler.go` — handler with declare / cancel / cancel-all; stashes itemID per-declaration for later cancel.
  - `/home/ab/projects/DnDnD/internal/discord/reaction_handler_test.go` — 23 tests covering declare, cancel substring-match, cancel-all combatant scoping.
  - `cmd/dndnd/main.go` line 1184: `discordHandlerSet.reaction.SetNotifier(dmQueueNotifier)`.
- Findings:
  - **CampaignID bug.** `reaction_handler.go:159` builds the Event with `Kind`, `PlayerName`, `Summary`, `GuildID`, and `ExtraMetadata` but omits `CampaignID`. `PgStore.Insert` will return `"parse campaign id"` after the Discord message has been sent. The reaction declaration row is persisted to `reaction_declarations` independently, so the in-game effect still works, but the `#dm-queue` Resolve link 404s.
  - Cancel linkage is in-memory-only; restart loses the map. The comment acknowledges this trade-off ("a restart losing the map is tolerable") — acceptable for phase scope.

### Phase 106d — /check skill-check narration gating

- Status: matches algorithmically (with the same CampaignID bug).
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/check_handler.go` lines 298–308 (gating decision), 466–518 (`shouldGate`, `postSkillCheckNarration`).
  - `/home/ab/projects/DnDnD/internal/discord/check_handler_phase106d_test.go` — 8 tests.
  - `/home/ab/projects/DnDnD/internal/dmqueue/format.go` `FormatSkillCheckNarrationFollowup`, narration metadata keys.
  - `/home/ab/projects/DnDnD/internal/dashboard/dmqueue_page.go` `HandleSkillCheckNarration`.
- Findings:
  - Gating rule: gate everything unless (natural 20 AND total ≥ DC with explicit DC), or (natural 1 with explicit DC). This is a reasonable interpretation of "trivial / auto-resolve checks bypass the queue" but the spec text is vaguer than the implementation. Worth documenting in the campaign settings layer ("narration policy") which Phase 106d's scope mentions.
  - AutoFail (e.g. character is incapacitated) bypasses gating and responds immediately — matches spec intent.
  - Same `CampaignID` omission as 106c at line 499; `PgStore.Insert` fails after Discord post.
  - itemID `""` fallback (no #dm-queue configured) gracefully falls through to immediate result.

### Phase 106e — /use runtime wiring

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/use_handler.go` lines 305–310 (`postConsumableToDMQueue`).
  - `cmd/dndnd/discord_handlers.go` line 196 builds `UseHandler`.
  - `cmd/dndnd/main.go` line 1188 calls `discordHandlerSet.use.SetNotifier(dmQueueNotifier)`.
- Findings:
  - UseHandler is now constructed at runtime (was a unit-test-only construction historically). `attachPhase105Handlers` registers it.
  - Same `CampaignID` omission at `use_handler.go:308`; consumable post fails to persist.

### Phase 106f — Dashboard auth middleware

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/dashboard/dm_middleware.go` (`RequireDM` + `DMVerifier`).
  - `/home/ab/projects/DnDnD/internal/dashboard/dm_middleware_test.go`.
  - `cmd/dndnd/main.go` lines 744–767 build `dmAuthMw = authMw ∘ dmRequire` and pass it to `RegisterDMQueueRoutes`.
- Findings:
  - The middleware is now composed correctly (sessions then DM check). `hasAuthUser` retained inside `ServeItem` / `HandleResolve` etc. as a defensive belt-and-suspenders check — fine.
  - `passthroughMiddleware` fallback for local dev (no `DISCORD_CLIENT_ID`) is preserved; `DMVerifier` is nil in that mode → gate is bypassed. This matches the documented intent.
  - 403 JSON body shape (`{"error":"forbidden: DM only"}`) is consistent with the rest of the dashboard.

### Phase 107 — /help system

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/help_handler.go` (lookup-by-topic).
  - `/home/ab/projects/DnDnD/internal/discord/help_content.go` (469 lines covering general help + per-command + per-class topics: attack, action, ki, rogue, cleric, paladin, metamagic, cast, move, check, save, rest, equip, inventory, use, give, loot, attune, prepare, retire, register, import, create-character, character, recap, distance, whisper, status, done, deathsave, command).
  - `/home/ab/projects/DnDnD/internal/discord/help_handler_test.go` (7 tests).
- Findings:
  - All ephemeral per spec.
  - Spec named class-specific topics rogue/cleric/paladin/ki/metamagic/attack/action — all present.
  - Embeds are not used (plain text with markdown), but the spec only says "embeds" descriptively. Acceptable.
  - "Context-specific tips (remaining attacks, available slots)" from spec line 440 are not implemented — `/help attack` returns a static string, not a runtime-personalised one. Minor gap.

### Phase 108 — /status

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/status_handler.go` (4 lookup interfaces — campaign, character, combatant, concentration, reaction).
  - `/home/ab/projects/DnDnD/internal/status/format.go` (`Info` struct, `FormatStatus`).
  - `/home/ab/projects/DnDnD/internal/discord/status_handler_test.go` (20 tests).
- Findings:
  - Sections covered: Conditions, Concentration, Temp HP, Exhaustion, Rage (with rounds), Wild Shape, Bardic Inspiration, Ki, Sorcery Points, Reaction Declarations, Readied Actions.
  - Conditions remaining duration in rounds is surfaced (`ConditionEntry.RemainingRounds`).
  - Two spec items partially covered:
    - "Channel Divinity uses" mentioned in `/help cleric` / `/help paladin` but NOT shown in `/status` (no `ChannelDivinityUses` field in `Info`).
    - "Smite slots" referenced in `/help paladin` but not in `/status` — implicit in spell slots though.
  - Out-of-combat path returns header + class-feature uses (Ki, Sorcery) only — matches the "active state for the character" intent.

### Phase 109 — /whisper

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/whisper_handler.go`.
  - `/home/ab/projects/DnDnD/internal/discord/whisper_handler_test.go` (6 tests).
  - `/home/ab/projects/DnDnD/internal/dmqueue/notifier.go` `PostWhisper` (note: correctly passes CampaignID), `ResolveWhisper`.
  - `/home/ab/projects/DnDnD/internal/dashboard/dmqueue_page.go` `HandleWhisperReply`.
- Findings:
  - Whisper poster correctly threads CampaignID (line 74: `campaign.ID.String()`). This handler is **NOT** affected by the CampaignID bug — uses the `PostWhisper` helper.
  - Ephemeral player ack + DM-queue post + DM dashboard reply form + Discord DM delivery is end-to-end wired.
  - "Standalone (no threaded view)" matches: each whisper is a fresh queue item.

### Phase 110 — Exploration mode

- Status: matches (with a transition-flip gap).
- Key files:
  - `/home/ab/projects/DnDnD/internal/exploration/service.go` — `StartExploration` (creates encounter with `mode='exploration'`, auto-seats PCs from spawn zones), `CapturePositions`, `ApplyPositionOverrides`, `EndExploration`.
  - `/home/ab/projects/DnDnD/internal/exploration/spawn.go` — spawn-zone parsing + assignment.
  - `/home/ab/projects/DnDnD/internal/exploration/dashboard.go` — HTML page, `/dashboard/exploration/start`, `/dashboard/exploration/transition-to-combat`.
  - `/home/ab/projects/DnDnD/internal/discord/move_handler.go` lines 272–276, 477–571 — exploration branch inside `MoveHandler.Handle`; resolves the invoker's PC, skips turn / resource economy, retains pathfinding + wall validation.
  - `/home/ab/projects/DnDnD/db/migrations/.../*encounters*.sql` — `mode` column with `'combat'` / `'exploration'` enum.
- Findings:
  - **Transition-to-combat is incomplete.** `HandleTransitionToCombat` (`internal/exploration/dashboard.go:149`) merges base positions with DM overrides and returns the result as JSON, but never calls `UpdateEncounterMode` or any StartCombat path. Spec says "DM starts an encounter on the current map" — this needs a follow-up wiring task in combat startup to consume those positions and flip the encounter's mode. Likely intentional (the endpoint is described as "captures positions"), but the endpoint name is misleading.
  - Theater-of-mind path is documented but has no code surface (correctly — the spec says "no dedicated mechanics, existing commands cover it").
  - `/check`, `/action`, `/move` all branch on `encounter.Mode == "exploration"`. `/move` skips speed limit but enforces walls — matches spec.
  - PC auto-placement from spawn zones in `StartExploration` matches the user-resolved Q3 design decision (2026-04-15).

### Phase 110a — /action freeform wiring

- Status: matches.
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/action_handler.go` lines 274–286 (mode branch), 313–379 (combat path), 455–541 (exploration path).
  - `/home/ab/projects/DnDnD/internal/combat/freeform_action.go` (combat service-side: action deduction + dm-queue post + `pending_actions` row).
  - `/home/ab/projects/DnDnD/internal/discord/action_handler_test.go` (38 tests).
  - `cmd/dndnd/main.go` line 1196 `SetNotifier`.
- Findings:
  - Combat path uses `combat.FreeformAction` which **does** pass CampaignID (`freeform_action.go:101–106`). Combat freeform actions persist correctly.
  - Exploration path posts directly via `action_handler.go:530` — this code DOES pass `CampaignID: encounter.CampaignID.String()`. Exploration freeform actions persist correctly.
  - `/action cancel` works in both modes via `CancelFreeformAction` / `CancelExplorationFreeformAction` and the shared `respondCancelResult` helper.
  - Cancel error sentinels (`ErrNoPendingAction`, `ErrActionAlreadyResolved`) match the spec's exact error wording ("❌ No pending action to cancel.", "❌ That action has already been resolved — use `/undo` to request a correction instead.").
  - Standard-action shortcuts (Dash, Disengage, Dodge, Help, Hide, Stand, Drop Prone, Escape, Channel Divinity, Lay on Hands, Action Surge, Stabilize) are dispatched away from the freeform path correctly — matches spec lines 1135–1186.

## Cross-cutting concerns

### Missing CampaignID in handler-direct Post calls (CRITICAL)

Six call sites build a `dmqueue.Event` without `CampaignID`:

| File:line | Kind |
|---|---|
| `internal/discord/reaction_handler.go:159` | KindReactionDeclaration |
| `internal/discord/check_handler.go:499` | KindSkillCheckNarration |
| `internal/discord/rest_handler.go:70` | KindRestRequest |
| `internal/discord/retire_handler.go:116` | KindRetireRequest |
| `internal/discord/undo_handler.go:75` | KindUndoRequest |
| `internal/discord/use_handler.go:308` | KindConsumable |

Effect chain:

1. `Notifier.Post` calls `Sender.Send` (line 165) — message lands in `#dm-queue` with Resolve link.
2. `Store.Insert` (line 170) — `PgStore.Insert` (`internal/dmqueue/pgstore.go:50–53`) calls `uuid.Parse("")` and returns `"parse campaign id: ..."`.
3. `Notifier.Post` returns `("", err)`.
4. Caller ignores or logs the error (the typical `_, _ = h.notifier.Post(...)` shape in undo / rest / use / retire; reaction stores `""` itemID).
5. Player clicks Resolve → dashboard 404s because `Notifier.Get(itemID)` returns false.
6. The DM-Queue list endpoint never shows the item because it filters by campaign.

Each call site already has access to the campaign (via `h.campaignProvider.GetCampaignByGuildID`) — the fix is a one-line addition. The combat-service-side `FreeformAction` and `postEnemyTurnReady` paths and the dedicated `PostWhisper` / `PostNarrativeTeleport` helpers all correctly thread `CampaignID`, so the fix pattern already exists.

The unit tests pass because they use the in-memory `MemoryStore` which doesn't validate CampaignID.

### Notifier.Post message-then-persist ordering

Even after the CampaignID fix, the Post sequence (Send → Insert) means any `Insert` failure leaves an orphan message in Discord. Reserve the ID + insert first, then send, OR delete the message on insert failure. The spec's "Resolved items are edited" model presumes the bot owns the message; an orphan can't be edited via the dashboard.

### Narrative teleport not posted at runtime

`KindNarrativeTeleport` is fully framework-supported (formatter, helper, dashboard rendering, test) but no /cast code path posts it. Either the /cast handler needs a "teleport beyond current map" detection (consulting spell scope) or a separate `/teleport` command. Phase 106b "Done when" mentions "narrative teleport" without specifying where the trigger lives.

### Exploration→Combat handoff incomplete

`/dashboard/exploration/transition-to-combat` computes the merged positions but doesn't flip `encounters.mode` or initiate combat. Spec requires "DM starts an encounter on the current map, which adds initiative." A separate combat-start dashboard handler likely needs to consume the positions JSON, but no caller exists. Document the boundary or add the flip.

### /help context-specific tips

Spec line 440 calls for "context-specific tips (e.g., remaining attacks, available spell slots)" via `/help`. Implementation returns purely static strings. Minor gap; cleanly bolted on later via runtime data injection in `HelpHandler.Handle`.

### Channel Divinity / Smite slots in /status

`/status` does not show Channel Divinity uses or Smite slot tracking, both of which the class-specific /help text invites the player to check via /status (`/help cleric`, `/help paladin`). Either trim the /help wording or add the fields.

## Critical items

1. **CampaignID omission in six handler-direct Post call sites** (`reaction`, `check`, `rest`, `retire`, `undo`, `use`). Production-blocking when paired with `PgStore`: messages land in `#dm-queue` but are unresolvable and never appear in the dashboard list. Highest-priority fix.
2. **Notifier.Post Send-then-Insert ordering** orphans messages on any Store error (campaign id, DB outage, JSON encode). Switch to reserve-ID-first or compensate on failure.
3. **Narrative teleport runtime caller missing.** Spec event listed for Phase 106b is dead code in production.
4. **Exploration→Combat transition endpoint is misleading.** `/dashboard/exploration/transition-to-combat` should either flip the mode + start combat in one POST or be renamed `/capture-positions` and paired with a separate StartCombat endpoint that consumes the JSON.
5. **No level-up review event in dm-queue.** The phase-106b checklist mentions "level-up review" informally; the implementation routes that flow through the approval queue. Worth a one-line note in `docs/phases.md` clarifying the design split.
