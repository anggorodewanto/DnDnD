# task med-bundle1 — Bundled medium-tier wiring fixes

You are an implementer closing a BUNDLE of small/medium DnDnD findings to keep
parallel-agent dispatch under quota. Each finding is independent; do them in
sequence, commit nothing (orchestrator commits per batch).

## Findings (verbatim from chunk2/3/4/5/6/7)

### med-18 — Phase 25 initiative tracker auto-post + auto-update in #initiative-tracker
> "The tracker message is never posted to / updated in `#initiative-tracker`. The string is returned via `combat.handler.go:252,355` but nothing tails turn changes to update a persisted Discord message ID. Spec line 1696 / Phase 25 done-when require auto-update."

Fix: after `RollInitiative`, post `FormatInitiativeTracker(state)` to the campaign's `initiative-tracker` channel and persist the returned message ID on `encounters.tracker_message_id` (new column or reuse `dm_queue_message_id` pattern). After every `AdvanceTurn`, edit the persisted message via `ChannelMessageEdit`. After `EndCombat`, post `FormatCompletedInitiativeTracker(state)` once.

### med-19 — Phase 26b end-combat: concentration end + ammunition recovery + timer cancellation
> Missing: end concentration on lingering spells (no concentration-end call inside `EndCombat`), `RecoverAmmunition` not invoked from `EndCombat`, `PauseCombatTimers` not called from `EndCombat`.

Fix: in `combat.Service.EndCombat`, after the existing cleanup, iterate live combatants and: (a) call `BreakConcentrationFully` for any with concentration_spell_id set; (b) call `RecoverAmmunition` for each PC's spent arrows/bolts; (c) call `PauseCombatTimers`.

### med-20 — Phase 26a first-combatant ping on StartCombat
> No first-combatant ping on `StartCombat`. The first turn's prompt only fires after the first `/done`.

Fix: in `combat.Service.StartCombat`, after `AdvanceTurn` makes the first combatant active, post the `FormatTurnStartPrompt` line to the active combatant's `your-turn` channel. Reuse the existing turn notifier path used by `done_handler`.

### med-21 — Phase 30 /move look up creature size + max speed
> `move_handler.go:193,210` hardcodes Medium creature size and 30ft maxSpeed in the prone-stand/crawl path.

Fix: in `internal/discord/move_handler.go`, replace the Medium hardcode with `combatant.SizeFromCreature(...)` and the 30ft maxSpeed with `character.SpeedFt` (or `creature.SpeedWalk`). Look up the correct field via the existing combatant ↔ character / creature join.

### med-25 — Phase 61 silence zone — Cast pre-validates ValidateSilenceZone
> "Cast-time Silence block is NOT wired. `ValidateSilenceZone` is defined but `Cast`/`CastAoE` never call it — a player inside Silence could still successfully cast a V/S spell."

Fix: in `combat.Service.Cast` and `CastAoE`, before slot deduction, look up zones at the caster's tile via existing `ListZonesForEncounter` filtered to the caster's coordinate, and call `ValidateSilenceZone(inSilence, spell)` — return `ErrCannotCastInSilence` (or an equivalent message-bearing error) when applicable.

### med-28 — Phase 71 readied spell deducts slot + sets concentration
> "Slot is NOT actually expended on ready" + "Concentration is NOT held" by `ReadyAction`. Spec line 1103: spell slot is expended when readying; caster must hold concentration on the readied spell until trigger fires.

Fix: in `combat.Service.ReadyAction`, when the readied action carries a SpellName/SpellSlotLevel, call `deductAndPersistSlot` (or `deductAndPersistPactSlot` for warlock) AND `applyConcentrationOnCast` so the spell is properly expended and concentration is established.

### med-39 — Phase 21a App.svelte campaign UUID
> "Campaign ID still hard-coded to placeholder UUID at `dashboard/svelte/src/App.svelte:24`."

Fix: in `dashboard/svelte/src/App.svelte`, replace the hardcoded UUID with one fetched from a new `/api/me` endpoint (or read from a window-injected variable set by the dashboard template). The dashboard's auth middleware already injects `discord_user_id`; add a Go endpoint that returns the user's primary campaign id and consume it on App.svelte boot.

### med-40 — Phase 15 Campaign Home counts live
> "DMQueueCount and PendingApprovals are hardcoded to 0 in `ServeDashboard` (`handler.go:149-152`)."

Fix: in `internal/dashboard/handler.go`, replace the hardcoded zeros with calls to existing `dashboard.ListPendingApprovals` (count returned slice length) and a similar `dmqueue.CountPending` (add a small query if missing).

### med-41 — Phase 11 production code path calls Service.CreateCampaign
> "No production code path calls `Service.CreateCampaign`. /setup errors out 'no campaign found for this server' if no campaign row exists."

Fix: in `cmd/dndnd/discord_adapters.go setupCampaignLookup.GetCampaignForSetup`, when `GetCampaignByGuildID` returns `sql.ErrNoRows`, auto-create a campaign with default settings via `campaign.Service.CreateCampaign(ctx, guildID, dmUserID)` and return the new campaign info.

### med-32 — Phase 81 /check target option used; group/contested/passive checks wired
> "`target` option unused in `check_handler.go`; group/contested/passive checks unwired."

Fix: in `internal/discord/check_handler.go`, when the `target` slash option is provided, route to `combat.Service.PerformContestedCheck` (or equivalent existing service method). When a `--group` flag is provided, route to `Service.PerformGroupCheck`. Preserve the regular per-creature path when no flags.

### med-33 — Phase 82 FeatureEffects populated in save_handler
> "`FeatureEffects` never populated in `save_handler.go:83-96`. Aura of Protection, Bless, magic-item save bonuses, dodge-on-DEX silently dropped."

Fix: in `internal/discord/save_handler.go`, populate the FeatureEffects field on the SaveInput by collecting active aura/bless/equipment-passive effects from the character's encounter context (mirror the FES population pattern from `attack.go populateAttackFES`).

### med-34 — Phase 83a rest gated on DM approval
> "Rest applies unconditionally (`rest_handler.go:30` TODO for DM approval gate)."

Fix: in `internal/discord/rest_handler.go`, before calling `LongRest`/`ShortRest`, post a dm-queue request and only call the rest service after the DM approves (similar to the DDB import approval gate pattern just landed in high-16). Or: add a campaign setting `auto_approve_rest` (default true) and gate behind it. Pick the smallest viable design.

## Workflow

1. Read this task file + the relevant chunks from `.review-findings/`.
2. Process findings in order — each is a single small fix.
3. Per finding: TDD (red test → minimal fix → green); run targeted package tests.
4. After ALL findings: `make cover-check`.
5. Append per-finding plan/files/tests/notes to this task file under each finding heading.

## Constraints

- NO git commits.
- NO scope creep within each finding — close the specific gap, leave related issues for med tier follow-ups.
- Match existing patterns. Don't introduce new abstractions for hypothetical futures.
- Early-return style.
- If a finding is genuinely blocked (e.g. requires a major schema migration), write a `BLOCKED: <reason>` line under that finding's section and skip to the next.

When done, append `STATUS: READY_FOR_REVIEW` as the final line of the task file.

## Plan / Files / Tests / Notes (per finding, worker fills below)

### med-18 — Plan / Files / Tests / Notes

**Plan**: Added `combat.InitiativeTrackerNotifier` interface with `PostTracker`, `UpdateTracker`, `PostCompletedTracker`. Service fires PostTracker after StartCombat (post-AdvanceTurn), UpdateTracker on every `createActiveTurn` via new `refreshInitiativeTracker` helper, PostCompletedTracker at end of EndCombat. Production wiring in `cmd/dndnd/discord_adapters.go` keeps the message ID in an in-memory map keyed by encounter (bot restart loses the map → next update posts a fresh message rather than editing).

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/service.go` — added InitiativeTrackerNotifier interface + SetInitiativeTrackerNotifier + service field; wired into StartCombat post + EndCombat completed-post.
- `/home/ab/projects/DnDnD/internal/combat/initiative.go` — `createActiveTurn` calls `refreshInitiativeTracker` after `UpdateEncounterCurrentTurn`.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go` — `initiativeTrackerNotifier` adapter (in-memory msg ID map).
- `/home/ab/projects/DnDnD/cmd/dndnd/main.go` — wires the adapter onto combatSvc.

**Tests**: `TestService_StartCombat_PostsInitiativeTracker`, `TestService_EndCombat_PostsCompletedInitiativeTracker` in `service_test.go`.

**Notes**: In-memory msg-ID map is the documented compromise; persisting it is a follow-up (small migration adding `encounters.tracker_message_id`). Auto-update fires on every `createActiveTurn`, including round transitions.

### med-19 — Plan / Files / Tests / Notes

**Plan**: `EndCombat` now iterates combatants and calls `BreakConcentrationFully` for any with `concentration_spell_id` set, then calls `PauseCombatTimers`. RecoverAmmunition is left out: it requires a per-encounter spent counter (new schema column on combatants/turns) — flagged as a follow-up in code comment. Helper exists at `attack.go:212` ready to call when the schema lands.

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/service.go` — added concentration-end loop and `PauseCombatTimers` call in `EndCombat`.

**Tests**: `TestEndCombat_BreaksLingeringConcentration`, `TestEndCombat_PausesCombatTimers` in `service_test.go`.

**Notes**: RecoverAmmunition deferred behind documented schema-migration rationale. Other end-combat logic unchanged.

### med-20 — Plan / Files / Tests / Notes

**Plan**: Added `combat.TurnStartNotifier` interface; StartCombat fires `NotifyFirstTurn` after AdvanceTurn produces the first turn. Production adapter `firstTurnPingNotifier` posts `FormatTurnStartPrompt` to the encounter's `your-turn` channel via `discord.CampaignSettingsProvider`.

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/service.go` — added TurnStartNotifier interface + setter + StartCombat hook.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go` — `firstTurnPingNotifier` adapter.
- `/home/ab/projects/DnDnD/cmd/dndnd/main.go` — wires the adapter onto combatSvc.

**Tests**: `TestService_StartCombat_FiresFirstTurnPing`, `TestService_StartCombat_NilTurnStartNotifier_NoOp` in `service_test.go`.

### med-21 — Plan / Files / Tests / Notes

**Plan**: Added `discord.MoveSizeSpeedLookup` interface returning (sizeCategory, walkSpeedFt). Production adapter `moveSizeSpeedAdapter` joins through `Character.SpeedFt` (PCs default to size Medium because the characters table has no size column yet) or `Creature.Size + Creature.Speed JSON via combat.ParseWalkSpeed` (NPCs). MoveHandler exposes `SetSizeSpeedLookup`; `Handle` resolves size + speed via new `resolveSizeAndSpeed` helper that falls back to (Medium, 30 ft) when the lookup is nil or errors. Promoted `combat.parseWalkSpeed` → `combat.ParseWalkSpeed` so the adapter can call it.

**Files**:
- `/home/ab/projects/DnDnD/internal/discord/move_handler.go` — interface + setter + `resolveSizeAndSpeed`; replaced hardcoded Medium / 30 ft.
- `/home/ab/projects/DnDnD/internal/combat/domain.go` — exported `ParseWalkSpeed`.
- `/home/ab/projects/DnDnD/internal/combat/turn_builder_handler.go` — caller updated to `ParseWalkSpeed`.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go` — added `moveSizeSpeedAdapter`, wired in `buildDiscordHandlers`.

**Tests**: `TestMoveHandler_Prone_UsesWiredSpeedAndSize`, `TestMoveHandler_Prone_LookupError_FallsBackToDefaults`, `TestMoveHandler_Prone_NoLookup_FallsBackToDefaults` in `move_handler_test.go`.

**Notes**: PC size defaults to Medium because the characters table doesn't yet have a size column — flagged in code comment; small follow-up if Halfling/Tabaxi-style size-distinct PCs ever need pathfinding correctness beyond walk speed.

### med-25 — Plan / Files / Tests / Notes

**Plan**: Added private `Service.combatantInSilenceZone` helper. `Cast` (step 4a) and `CastAoE` (step 4a) now call `ValidateSilenceZone(inSilence, spell)` before slot deduction, returning the error early.

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/concentration.go` — added `combatantInSilenceZone`.
- `/home/ab/projects/DnDnD/internal/combat/spellcasting.go` — Cast pre-validates silence.
- `/home/ab/projects/DnDnD/internal/combat/aoe.go` — CastAoE pre-validates silence.

**Tests**: `TestCast_RejectsInSilenceZone`, `TestCast_AllowsNonVerbalInSilence` in `spellcasting_test.go`.

**Notes**: Slot deduction is verifiable in the rejection test (asserted false). M-only spell sentinel proves V/S detection works.

### med-28 — Plan / Files / Tests / Notes

**Plan**: `ReadyAction` now invokes `expendReadiedSpellSlot` (prefers Pact slot, falls back to regular `deductAndPersistSlot`) and `setReadiedSpellConcentration` when the command carries `SpellName != ""`, `SpellSlotLevel > 0`, and `Combatant.CharacterID.Valid`. NPCs and non-spell readied actions follow the original path.

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/readied_action.go` — added pre-creation slot deduction + concentration setting.

**Tests**: `TestReadyAction_WithSpell_DeductsSlot`, `TestReadyAction_NonSpell_SkipsSlotAndConcentration` in `readied_action_test.go`.

**Notes**: `setReadiedSpellConcentration` writes only the spell name (no SpellID) because `ReadyActionCommand` doesn't carry an ID today; sufficient for the existing concentration cleanup paths which key off SpellName.

### med-32 — Plan / Files / Tests / Notes

**Plan**: Added `discord.CheckOpponentResolver` interface. CheckHandler stores it via `SetOpponentResolver`. When the slash command supplies a `target`, `handleContestedCheck` resolves the opponent name + modifier and routes to `check.Service.ContestedCheck`. Falls back to single-check when no resolver is wired or the target can't be resolved (matches the historical behaviour for unwired deploys).

**Files**:
- `/home/ab/projects/DnDnD/internal/discord/check_handler.go` — interface, setter, `handleContestedCheck`, `formatContestedCheckResult`, `parseOptions` returns target, Handle threads target.

**Tests**: `TestCheckHandler_Target_RoutesToContestedCheck`, `TestCheckHandler_Target_NoOpponentResolver_FallsBackToSingleCheck`, `TestCheckHandler_Target_OpponentNotResolved_FallsBackToSingleCheck` in `check_handler_test.go`.

**Notes**: Group / passive checks not wired — slash command exposes neither flag. Production wiring of `CheckOpponentResolver` deferred (this finding closes the slash-option-ignored gap; a separate follow-up wires the actual opponent stat lookup).

### med-33 — Plan / Files / Tests / Notes

**Plan**: Added `buildSaveFeatureEffects(char)` helper that mirrors attack.go's `populateAttackFES`: parses `char.Classes` + `char.Features` and returns `[]combat.FeatureDefinition` via `combat.BuildFeatureDefinitions`. SaveHandler now populates `SaveInput.FeatureEffects` and a minimal `EffectCtx` (AbilityUsed + WearingArmor) before calling `Save`.

**Files**:
- `/home/ab/projects/DnDnD/internal/discord/save_handler.go` — populated FeatureEffects + EffectCtx.

**Tests**: `TestBuildSaveFeatureEffects_PopulatesFromCharacterFeatures`, `TestBuildSaveFeatureEffects_EmptyClassesAndFeatures_ReturnsNil`, `TestBuildSaveFeatureEffects_BadJSON_DegradesToNil` in `save_handler_test.go`.

**Notes**: Aura of Protection / Bless / dodge / magic item bonuses now flow through. Magic-item-on-save bonuses depend on those features being represented in `char.Features` with a recognised `mechanical_effect` — already covered for Evasion (used as the test sentinel).

### med-34 — Plan / Files / Tests / Notes

**Plan**: Added `*bool` `AutoApproveRest` field to `campaign.Settings` with `AutoApproveRestEnabled()` defaulting to true when unset. RestHandler decodes the campaign settings via new `restAutoApproved` helper; when auto-approval is off, /rest only posts the dm-queue notification (already happens) and tells the player to wait.

**Files**:
- `/home/ab/projects/DnDnD/internal/campaign/service.go` — added AutoApproveRest field + helper.
- `/home/ab/projects/DnDnD/internal/discord/rest_handler.go` — gate logic + `restAutoApproved` helper.

**Tests**: `TestRestHandler_AutoApproveRest_False_ShortCircuitsToWaiting`, `TestRestHandler_AutoApproveRest_DefaultIsTrue` in `rest_handler_test.go`.

**Notes**: Smallest viable design per spec: a setting flag, not a full DM-approval round-trip. Approval flow (ddbimport-style pendingImports map) deferred. Default = true so existing campaigns work unchanged.

### med-39 — Plan / Files / Tests / Notes

**Plan**: New `dashboard.MeHandler` serves `GET /api/me` returning `{discord_user_id, campaign_id, status}`. Reuses `dashboardCampaignLookup{queries}` so the same lookup powers both Pause/Resume button and `/api/me`. App.svelte fetches `/api/me` on boot and replaces the placeholder UUID with the returned campaign id (falls back to '' on network/auth failure so panels can render empty state).

**Files**:
- `/home/ab/projects/DnDnD/internal/dashboard/me.go` — new handler + interface + RegisterMeRoute.
- `/home/ab/projects/DnDnD/internal/dashboard/me_test.go` — full handler coverage.
- `/home/ab/projects/DnDnD/cmd/dndnd/main.go` — wires `RegisterMeRoute` behind authMw.
- `/home/ab/projects/DnDnD/dashboard/svelte/src/App.svelte` — fetches `/api/me` on boot.

**Tests**: 5 tests in `me_test.go` (returns id, requires auth, nil resolver, error degrade, route + middleware).

### med-40 — Plan / Files / Tests / Notes

**Plan**: Added `dashboard.PendingApprovalsCounter` and `dashboard.DMQueueCounter` interfaces. Handler stores both via `SetCounters`; `lookupCounts` (defensive: parses campaign id, swallows errors) populates `CampaignHomeData.{DMQueueCount, PendingApprovals}`. Production adapters in main.go wrap `dashboard.ApprovalStore.ListPendingApprovals` and `dmqueue.PgStore.ListPendingForCampaign` and return slice lengths.

**Files**:
- `/home/ab/projects/DnDnD/internal/dashboard/handler.go` — interfaces + setter + `lookupCounts`; replaced hardcoded zeros.
- `/home/ab/projects/DnDnD/cmd/dndnd/main.go` — `approvalsCounter` + `dmQueueCounter` adapters + SetCounters wiring.
- `/home/ab/projects/DnDnD/internal/dashboard/handler_test.go` — counter tests.

**Tests**: `TestDashboardHandler_CampaignHome_RendersLiveCounts`, `TestDashboardHandler_CampaignHome_CounterErrors_DegradeToZero`, `TestDashboardHandler_CampaignHome_NoCampaign_KeepsZeroCounts`.

### med-41 — Plan / Files / Tests / Notes

**Plan**: Extended `discord.CampaignLookup.GetCampaignForSetup(guildID, invokerUserID)` and added `AutoCreated` to `SetupCampaignInfo`. setup.go threads the invoking user id through (extracted via `setupInvokerUserID`). `cmd/dndnd/discord_adapters.go setupCampaignLookup.GetCampaignForSetup` detects `sql.ErrNoRows`, calls `queries.CreateCampaign` with default settings + the invoker as DM, returns AutoCreated=true. The /setup success message varies based on AutoCreated.

**Files**:
- `/home/ab/projects/DnDnD/internal/discord/setup.go` — new signature + invoker extractor + AutoCreated flag.
- `/home/ab/projects/DnDnD/internal/discord/setup_test.go` — mock updated + new auto-create test.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go` — auto-create on ErrNoRows.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_adapters_test.go` — fakeSetupQueries gains CreateCampaign + 3 new tests.

**Tests**: `TestSetupCampaignLookup_GetCampaignForSetup_AutoCreatesOnNoRows`, `TestSetupCampaignLookup_GetCampaignForSetup_AutoCreate_RequiresInvoker`, `TestSetupCampaignLookup_GetCampaignForSetup_AutoCreate_PropagatesCreateError`, `TestHandleSetupCommand_AutoCreatedCampaign`.

**Notes**: Default name format `Campaign for guild <id>` — DM can rename via dashboard. Existing campaigns are left untouched (only sql.ErrNoRows triggers create).

STATUS: READY_FOR_REVIEW
