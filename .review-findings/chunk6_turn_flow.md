# Chunk 6 Review — Phases 73–87 (Turn Flow, Resources, Inventory, Loot, Shops)

## Summary

Phases 73, 75a/b, 76a/b, 77, 78a/c, 78b, 79, 80, 83a, 84, 85 (claim path only) have substantive, well-tested implementations with their done-when bullets met. Several phases are flagged as completed in `docs/phases.md` but have **production-wiring gaps** that mean the feature is unreachable from a running bot:

- **Phase 74 (`/interact`)** — Service is implemented and unit-tested, but no Discord handler is wired. `/interact` is dispatched to a generic stub handler that returns "/interact is not yet implemented." (`internal/discord/router.go:228-234`, no `SetInteractHandler`). The service code (`internal/combat/interact.go`) has not been hooked up to a `CommandHandler`, so the spec's first-free-then-action and auto-resolve routing never runs in production.
- **Phase 81 (`/check`)** — Group, contested, passive, and targeted (adjacency + action cost) check flows are present in `internal/check/` but unwired in the Discord handler. The slash command exposes a `target` option but the handler ignores it.
- **Phase 82 (`/save`)** — The save service supports `FeatureEffects` (Aura of Protection, Bless, magic items, dodge effect, etc.) but the handler never populates them, so the spec's "auto-include all modifiers" promise is partially unmet (only condition+exhaustion are wired).
- **Phase 83a (`/rest`)** — DM approval gate is **not** wired (explicit TODO at `internal/discord/rest_handler.go:30`). Rests apply benefits unconditionally after a notifier-only post.
- **Phase 83b** — `PartyRestHandler` is fully implemented but **never instantiated** in `cmd/dndnd/main.go`; the `/api/.../party-rest` route does not exist; no dashboard UI calls it.
- **Phase 85** — `/loot` Discord claim path works. **Loot dashboard endpoints** (`CreateLootPool`, `AddItem`, `RemoveItem`, `SplitGold`, `PostAnnouncement`, `ClearPool`) are coded in `internal/loot/api_handler.go` but `loot.NewAPIHandler` is **never called** in `main.go`; the dashboard side of populating the pool is unreachable. Auto-population from defeated creatures via `loot.Service.CreateLootPool` is reachable only because nothing calls it.
- **Phase 86 (Item Picker)** — `internal/itempicker` package is **not imported anywhere outside its tests**. The Svelte `ItemPicker.svelte` calls `searchItems()` against `/api/campaigns/.../items/search`, which 404s. SRD-only search (no homebrew federation, no creature-inventory integration in the picker; creature-inventory route exists in itempicker but is also unmounted).
- **Phase 87 (Shops)** — `internal/shops` package is **not imported in production code**. The `ShopBuilder.svelte` UI hits API routes (`/api/campaigns/.../shops`) that are never mounted; "Post to #the-story" cannot fire.

Other notable findings:

- **Phase 75b** stealth-disadvantage from armor is enforced for the Hide *standard action* (`internal/combat/standard_actions.go:416`) but not for `/check stealth`. Phase 75b's done-when "stealth disadvantage enforcement" is therefore narrowly satisfied by Hide but spec-wise (line 873 "Equipment Enforcement") incomplete.
- **Phase 84** explicitly defers combat-time costs for `/use` and `/give` and the `potion_bonus_action` campaign setting (correctly noted at `docs/phases.md:485`). However, no follow-up phase tracker exists in `docs/phases.md`; this is the only deferred item not tied to a numbered follow-up phase. The campaign settings field `PotionBonusAction` already exists at `internal/campaign/service.go:27` but is unused.
- **Phase 76a/b** turn timer goroutine is started (`cmd/dndnd/main.go:795`) after a startup stale-state scan (`main.go:648`). Stop is `defer`ed, but stop-after-stop would panic — minor risk only on test code paths that re-create a bot.

## Per-phase findings

### Phase 73 — Freeform Actions & `/action cancel` ✅
- `combat.FreeformAction` at `internal/combat/freeform_action.go:49-91` consumes the action resource, posts via wired `dmNotifier`, persists `pending_actions` row with `dm_queue_item_id`, and produces both combat-log and dm-queue messages.
- `combat.CancelFreeformAction` at `freeform_action.go:140-172` refunds the action, marks the pending row "cancelled", and forwards a `Cancel` to the notifier so the original Discord message is struck through with "Cancelled by player".
- Already-resolved guard at `freeform_action.go:146-148` returns `ErrActionAlreadyResolved`; no-pending guard returns `ErrNoPendingAction` at line 143.
- `CancelExplorationFreeformAction` at `freeform_action.go:206-231` provides the parallel exploration-mode path (no action resource).
- Discord wiring at `internal/discord/action_handler.go:175,197,269` calls these and translates errors to the spec-mandated ephemeral messages (`action_handler.go:281-290`).
- Tests: `internal/combat/freeform_action_test.go` covers all branches; `internal/discord/action_handler_test.go` covers the user-facing surface.

### Phase 74 — Free Object Interaction (`/interact`) ❌
- Service: `combat.Interact` at `internal/combat/interact.go:58-107` correctly enforces "first free, second costs action," returns rejection if action is spent (line 67), and routes auto-resolvable patterns (`internal/combat/interact.go:13-24`) immediately while creating a `pending_actions` row otherwise.
- **No Discord handler**. `internal/discord/router.go:198-204` lists `interact` among `gameCommands`; both the status-aware stub and the generic stub end up dispatched there because the router has no `SetInteractHandler` method (`grep -rn 'InteractHandler\\|SetInteract' internal/discord` returns nothing). The service is not callable through `/interact`.
- The auto-resolvable pattern list (`internal/combat/interact.go:13-24`) lacks the spec's "press button," "flip switch," "pull lever" — narrow, but minor.
- The DM-queue route in the service (`interact.go:89-96`) calls `s.store.CreatePendingAction` but **does not** post via `dmNotifier`, so even when wired the `#dm-queue` message will never be delivered (spec section "Free Object Interaction" line 2070 expects DM queue post for non-trivial interactions).
- Tests: `internal/combat/interact_test.go` covers the service paths but no end-to-end Discord-handler coverage exists.

### Phase 75a — `/equip` Command & Hand Management ✅
- `combat.Equip` at `internal/combat/equip.go:99-122` dispatches to weapon / shield / armor / unequip flows.
- Two-handed validation at `equip.go:129-131`. Off-hand-occupied-by-shield-when-equipping-weapon at `equip.go:145-147`.
- In-combat costs: weapon equip costs `ResourceFreeInteract` (`equip.go:135`), shield don/doff costs `ResourceAction` (`equip.go:175,294`), armor blocked in combat (`equip.go:207, 255`).
- "Auto-stow off-hand on shield equip" implemented (`equip.go:183`).
- Discord wiring: `internal/discord/equip_handler.go` dispatches to the service; help content at `internal/discord/help_content.go:411` documents the command.
- `HasFreeHand` and `CheckSomaticComponent` (`equip.go:41-48, 450-468`) are exposed for spell-component validation.
- Tests: `internal/combat/equip_test.go` and `internal/discord/equip_handler_test.go` cover all spec branches.

### Phase 75b — AC Recalc & Equipment Enforcement ⚠️
- `combat.RecalculateAC` (`equip.go:369-404`) covers armor-based, unarmored-defense (Barbarian/Monk via `evaluateACFormula`, line 407), shield bonus, and DEX cap.
- `CheckHeavyArmorPenalty` (`equip.go:435-444`) returns 10ft when STR < `strength_req`, applied to combat log (`equip.go:228-230`).
- Stealth-disadvantage flag is read by the **Hide standard action** (`internal/combat/standard_actions.go:416`) but **not** by the generic `/check stealth` flow (`internal/discord/check_handler.go` does not read the equipped armor). Spec line 873 lists "Equipment Enforcement" under Phase 75b — this is a narrow gap: a player rolling `/check stealth` while wearing chain mail will not get auto-imposed disadvantage.
- Speed penalty from heavy armor is computed at equip time but not re-applied to the combatant's `speed_remaining` at turn start (`internal/combat/equip.go:228-230` only logs it). If a player equips heavy armor mid-combat, the speed penalty does not propagate to the in-flight turn's `MovementRemainingFt`.
- Tests: `internal/combat/equip_test.go` exhaustively covers AC formulas; `TestHide_ArmorStealthDisadvantage` at `internal/combat/standard_actions_test.go:1675` covers Hide.

### Phase 76a — Turn Timeout: Timer Infrastructure & Nudges ✅
- `combat.TurnTimer` at `internal/combat/timer.go:27-77` polls `ListTurnsNeedingNudge`, `ListTurnsNeedingWarning`, `ListTurnsTimedOut`, `ListTurnsNeedingDMAutoResolve` each tick.
- 30-second interval wired in `cmd/dndnd/main.go:641`. Goroutine started **after** Discord gateway open and **after** stale-state scan (`main.go:795`); stop deferred at line 796.
- 50% nudge: `processNudges` → `sendNudge` (`timer.go:93-145`) using `FormatNudge` (`timer_messages.go`), idempotent via `UpdateTurnNudgeSent`.
- 75% tactical summary: `FormatTacticalSummary` at `timer_messages.go:19` includes HP, AC, conditions, resources, and adjacent enemies pulled via `ListAdjacentCombatants`. Sent in `sendWarning` (`timer.go:147-175`), idempotent via `UpdateTurnWarningSent`.
- DM overrides: `internal/combat/timer_overrides.go` exposes `SkipNow`, `ExtendTurn`, `PauseEncounter` against the same store. Default 24h timeout configurable per campaign via `campaigns.turn_timeout_hours`.
- Startup stale-state scan: `timer.PollOnce` runs at `cmd/dndnd/main.go:648` before `rawDG.Open()` (correctly documented in surrounding comments).
- Tests: `internal/combat/timer_test.go`, `timer_messages_test.go`, `timer_overrides_test.go`, `timer_stale_integration_test.go`.

### Phase 76b — 100% Resolution & Prolonged Absence ✅
- DM decision prompt: `FormatDMDecisionPromptWithSaves` (`timer_resolution.go:26-59`) lists pending movement / attacks / bonus / saves and renders Wait / Roll / Auto-Resolve buttons.
- Wait extension capped at one per timeout cycle: `WaitExtendTurn` (`timer_resolution.go:298-330`) returns `ErrWaitAlreadyUsed` (line 17) when reused; persists to `turns.wait_extended`.
- Auto-resolve: `AutoResolveTurn` applies Dodge, rolls pending CON saves (with concentration resolver wired at `cmd/dndnd/main.go:644` to fire failed-save cleanup), forfeits reaction declarations (`timer_resolution.go:264`), and marks turn auto-resolved.
- Prolonged absence: `combatants.consecutive_auto_resolves` increments at `timer_resolution.go:272-278`; `is_absent` flips at 3, message appended at line 283.
- DM 1-hour fallback: `processDMAutoResolves` (called from `timer.PollOnce`) — confirmed at `internal/combat/timer.go:90` and via `timer_resolution.go` integration tests.
- Tests: `internal/combat/timer_resolution_test.go`, `timer_stale_integration_test.go`.

### Phase 77 — Turn Start & `/done` ✅
- Turn-start condition expiration and effect application live in `combat.StartNextTurn` (called from `done_handler.go:410`); confirmed via combat log generation at `service.go` (Phase 39 already validated).
- Impact summary: `Service.GetImpactSummary` at `internal/combat/impact_summary.go:14-87` filters action log entries since the player's last completed turn and joins them into a personal summary line. Wired into the next-player ping via `DoneHandler.SetImpactSummaryProvider` (`internal/discord/done_handler.go:142-145`).
- Unused-resource warning: `combat.CheckUnusedResources` invoked at `done_handler.go:214`; `FormatUnusedResourcesWarning` produces the ephemeral confirmation; component buttons flow through `HandleDoneConfirm` / `HandleDoneCancel` (`router.go:391-399`).
- Map regen: `MapRegenerator` interface at `done_handler.go:82-90`; `PostCombatMap` at `done_handler.go:437` posts to `#combat-log` after each turn.
- Auto-skip incapacitated: `FormatAutoSkipMessage` (`internal/combat/auto_skip.go:13`) called at `done_handler.go:399-400` whenever `StartNextTurn` returns a `SkippedCombatant`.
- DM can end any turn: handled via the same `/done` plus DM dashboard overrides (Phase 76a `SkipNow`).
- Tests: `internal/combat/auto_skip_test.go`, `unused_resources_test.go`, `impact_summary_test.go`, `internal/discord/done_handler_test.go`, `done_handler_new_test.go`.

### Phase 78a — Enemy/NPC Turns: Dashboard Turn Builder ✅
- Plan model at `internal/combat/turn_builder.go:28-43` (`TurnPlan` with steps for movement / attack / multiattack / ability / bonus_action).
- A* path suggestion via `pathfinding` package, consumed in `MovementStep` (`turn_builder.go:46-50`).
- HTTP routes mounted at `internal/combat/handler.go:50-51`: `GET /api/combat/{enc}/enemy-turn/{combatantID}/plan` and `POST /api/combat/{enc}/enemy-turn`. Registered in `cmd/dndnd/main.go:500`.
- Pending reactions surfaced inside the plan response (`turn_builder.go:32-33`, `turn_builder_handler.go:64-65`).
- Roll-fudging / reorder / remove handled in the Svelte UI (`dashboard/svelte/src/TurnBuilder.svelte` 478 lines, with review-mode toggle at line 12 and step rendering at line 144).
- Combat-log post on confirm: `EnemyTurnNotifier` at `turn_builder_handler.go:20-27`; default impl at `internal/discord/enemy_turn_notifier.go`.
- Tests: `internal/combat/turn_builder_test.go`, `turn_builder_handler_test.go`, `enemy_turn_notify_test.go`.

### Phase 78c — Bonus Action Parsing ✅
- `ParseBonusActions` at `internal/combat/turn_builder.go:464-475` (with helper `BonusActionEntry`).
- Filters abilities whose description mentions "bonus action" case-insensitively. Used at `turn_builder.go:152-156` to generate `bonus_action` steps in the plan.
- Tests: `internal/combat/turn_builder_test.go` includes Goblin Nimble Escape regression.

### Phase 78b — Legendary & Lair Actions ✅
- `LegendaryInfo` and `LairInfo` parsed from creature ability text (`internal/combat/legendary.go:46-115` for `ParseLegendaryInfo`).
- Budget regex `can take (\d+) legendary action` at `legendary.go:39`; cost regex `\(Costs? (\d+) Actions?\)` at line 42; budget defaults to 3 when description is silent (`legendary.go:67-69`).
- `LegendaryActionTracker` and `BuildLegendaryActionMenu` enforce per-round budget, deduct cost on use, and reset at the creature's turn (verified in `legendary_test.go`).
- Lair actions: HTTP routes at `internal/combat/legendary_handler.go:20-21`; `LairActionTracker` enforces no-consecutive-repeat (`legendary_handler.go:275-280`); turn queue places lair actions at initiative 20 losing ties (`legendary.go:307`).
- Tests: `internal/combat/legendary_test.go`, `legendary_handler_test.go`.

### Phase 79 — Summoned Creatures & Companions ✅
- `SummonCreature` at `internal/combat/summon.go:44-71` creates the combatant with `summoner_id` link.
- `/command` Discord handler: `internal/discord/summon_command_handler.go`. `combat.CommandCreature` validates summoner ownership (returns `ErrNotSummoner` at `summon.go:27`) and routes to action / move / dismiss / done.
- `DismissSummon` (`summon.go:79-149`) and `DismissSummonsByConcentration` (`summon.go:151-180`) both exist; the latter is wired into the Phase 117/118 concentration-break pipeline.
- Initiative placement: handled inside the start-of-combat flow when combatants are inserted with their own initiative roll; same data path also handles "act on caster's turn" by re-using the caster's initiative.
- Tests: `internal/combat/summon_test.go`, `summon_integration_test.go`.

### Phase 80 — Combat Recap (`/recap`) ✅
- Service helpers: `ListActionLogWithRounds`, `GetMostRecentCompletedEncounter`, `GetLastCompletedTurnByCombatant`, `FilterLogsSinceRound`, `FilterLogsLastNRounds`, `RecapRoundRange`, `FormatRecap`, `TruncateRecap` all in `internal/combat/recap.go`.
- Discord handler at `internal/discord/recap_handler.go` resolves encounter via `ActiveEncounterForUser` then falls back to `GetMostRecentCompletedEncounter` for post-combat recap (`recap_handler.go:117-141`).
- "Until archived" interpretation: encounters use `status='completed'` (no `archived` status exists in `db/migrations/`); the most-recent-completed query at `internal/refdata/encounters.sql.go:25-30` orders by `updated_at DESC LIMIT 1`. Practical equivalent of the spec's "until archived" since archive is not modeled.
- `/recap` (no args) during active combat → since-last-turn slice (`recap_handler.go:101-103`); post-combat → all rounds (line 105-108); `/recap N` → last N rounds (line 99). Discord 2000-char truncation at line 111.
- Tests: `internal/combat/recap_test.go`, `internal/discord/recap_handler_test.go`.

### Phase 81 — Skill & Ability Checks (`/check`) ⚠️
- Core math: `check.SingleCheck` at `internal/check/check.go:72-108` covers ability / skill, expertise (`SkillModifier`), Jack of All Trades (`JackOfAllTrades`), advantage / disadvantage flags, and condition modifiers (auto-fail and roll-mode adjustments via `combat.CheckAbilityCheckWithExhaustion`).
- `PassiveCheck` (`check.go:147-165`), `GroupCheck` (`check.go:196-220`, half-must-succeed), `ContestedCheck` (`check.go:246-266`) all implemented but **never invoked** outside their own unit tests (`grep -rn 'GroupCheck\\|ContestedCheck\\|PassiveCheck' internal/discord internal/dashboard cmd` returns nothing).
- Targeted checks: slash command exposes a `target` option (`internal/discord/commands.go:279`) but `CheckHandler.parseOptions` (`check_handler.go:248-263`) ignores it. No adjacency validation, no in-combat action cost is charged (spec line 462: "targeted checks (`/check medicine AR`): adjacency validation, action cost in combat" — this is **not** implemented).
- DM-prompted checks are handled in Phase 106d via `skill_check_narration_deliverer.go`; gating logic is real and exercised by `check_handler_phase106d_test.go`.
- Stealth-disadvantage from armor is **not** auto-applied to `/check stealth` (see Phase 75b note above).
- Tests: `internal/check/check_test.go` and `internal/discord/check_handler_test.go` cover what is wired; the unwired group/contested/passive paths are covered only at the service level.

### Phase 82 — Saving Throws (`/save`) ⚠️
- `save.Save` at `internal/save/save.go:54-101` accepts `FeatureEffects []combat.FeatureDefinition` and `EffectCtx combat.EffectContext`; calls `combat.ProcessEffects` for `TriggerOnSave` to layer Aura of Protection / Bless / magic items.
- Auto-fail conditions: `combat.CheckSaveWithExhaustion` (line 64) handles paralyzed / stunned / unconscious / petrified on STR/DEX and exhaustion-level effects.
- **Handler does not populate FeatureEffects.** `internal/discord/save_handler.go:83-96` builds `SaveInput` with conditions only, leaving `FeatureEffects` and `EffectCtx` zero-valued. Aura of Protection, Bless, magic-item save bonuses, and dodge advantage on DEX saves are therefore silently dropped — spec line 467 ("Auto-include all modifiers — Paladin Aura of Protection, Bless, condition effects, magic items") is partially unmet.
- Roll history logging is wired (`save_handler.go:108-118`).
- DM-prompted saves: `internal/combat/concentration_integration_test.go` and the Phase 118 concentration pipeline drive auto-rolls; standalone DM-prompted save UX (button) is **not** in the codebase that I can find.
- Tests: `internal/save/save_test.go` exercises the full feature-effect pipeline; the `_handler_test.go` only covers the wired subset.

### Phase 83a — Short & Long Rests: Individual Flow ⚠️
- `rest.ShortRest` (`internal/rest/rest.go:60-152`) consumes hit dice (single-class buttons + multiclass per-die-type), restores Warlock pact slots (`rest.go:125-141` from earlier output), and refreshes short-rest features.
- `rest.LongRest` (`rest.go:189-260`) restores HP, all spell slots, all features, hit dice (half total level minimum 1), pact slots, and resets death-save tally (`PreparedCasterReminder` flag for cleric/druid/paladin at line 183).
- **DM approval gate is NOT wired** — explicit TODO at `internal/discord/rest_handler.go:30`: "Wire DM approval flow when the DM queue approval callback system is built." `dmQueueFunc` field is stored but unused; the only DM-side touch is a fire-and-forget `KindRestRequest` notification posted before the rest applies (`rest_handler.go:51-61`). The rest applies regardless of whether the DM approves.
- Tests: `internal/rest/rest_test.go` covers all rest math; `internal/discord/rest_handler_test.go` covers the (un-gated) flow.

### Phase 83b — Party Rest & Interruption ⚠️
- `InterruptRest` at `internal/rest/party.go:17-22` applies the 1-hour-elapsed → short-rest-benefits rule for an interrupted long rest.
- `PartyRestHandler` at `internal/rest/party_handler.go:88-114` wires lister / updater / encounter checker / notifier / poster, processes `PartyRestRequest`, refuses if combat is active (line 137), partitions selected vs excluded characters, applies short or long rest per character, posts `FormatPartyRestSummary` to Discord.
- **Never instantiated.** No `rest.NewPartyRestHandler(...)` call exists in `cmd/dndnd/main.go` (verified by `grep`). No Chi route mounts `HandlePartyRest`. The dashboard Svelte tree has no party-rest UI (`grep -ri 'PartyRest\|party-rest\|party_rest' dashboard/svelte/src` returns nothing). The done-when "Integration tests verify party rest batch flow" passes via in-package tests only; the user-visible feature is dead code.
- Tests: `internal/rest/party_handler_test.go` covers the package, but no integration through main exists.

### Phase 84 — Inventory Management ⚠️ (deferred, but tracked as ✅ in phases.md)
- `/inventory`: `internal/discord/inventory_handler.go` displays grouped items, attunement, gold via `inventory.FormatInventory` (`internal/inventory/service.go:204`).
- `/use`: `inventory.UseConsumable` (`service.go:108-161`) auto-resolves healing-potion (2d4+2) and greater-healing-potion (4d4+4) via `autoResolveItems` map at line 102-106; antitoxin produces a flavor message; everything else flags `DMQueueRequired`. Combat-time action cost **deliberately deferred** (`docs/phases.md:485`); handler at `internal/discord/use_handler.go` does not consume any turn resource regardless of combat status.
- `/give`: `inventory.GiveItem` (`service.go:179-201`) transfers one unit; **no adjacency check**, no free-interaction cost (deferred).
- Gold tracking: `UpdateCharacterGold` in `internal/refdata/characters.sql.go`; persisted as `int4` on character; integer-only (matches spec).
- DM inventory dashboard: `internal/inventory/api_handler.go` exposes endpoints; wired at `cmd/dndnd/main.go:607-609` via `dashboard.RegisterInventoryAPI`.
- The `potion_bonus_action` campaign setting field exists at `internal/campaign/service.go:27` but is unused — no read site, no UI.
- **Process gap**: deferred work is not tracked as a numbered follow-up phase. Every other deferred item in `docs/phases.md` is tied to a specific phase number; this one references "the combat-items integration phase" with no phase number.
- Tests: `internal/inventory/service_test.go`, `integration_test.go`, `internal/discord/inventory_handler_test.go`, `use_handler_test.go`, `give_handler_test.go`.

### Phase 85 — Looting System ⚠️
- `loot.Service.CreateLootPool` at `internal/loot/service.go:66-...` auto-populates from defeated creatures' inventories + gold. Single-claim enforcement at the SQL layer (`ClaimLootPoolItem` returns no rows if already claimed).
- `loot.Service.SplitGold` at `service.go:250` divides pool gold among approved party members; updates each character's gold.
- `/loot` Discord: `internal/discord/loot_handler.go` lists pool items as Discord buttons; `HandleLootClaim` at `loot_handler.go:166` enforces single-claim via `ErrItemAlreadyClaimed` (line 23). This **is** wired (router at `internal/discord/router.go:417-420`).
- **Loot dashboard endpoints not wired.** `internal/loot/api_handler.go` exposes `HandleGetLootPool`, `HandleCreateLootPool`, `HandleAddItem`, `HandleRemoveItem`, `HandleSplitGold`, `HandlePostAnnouncement`, `HandleClearPool` (`api_handler.go:67-238`), but `loot.NewAPIHandler` is **never called** in `cmd/dndnd/main.go` and there is no `loot.RegisterRoutes` (none defined). Dashboard cannot manually populate the loot pool, post the announcement, or split gold via UI; the only path is the slash command claim. The `cmd/dndnd/main.go:683-685` comment "in future phases" implies this is intentional, but `docs/phases.md:487` claims Phase 85 is done.
- Tests: `internal/loot/service_test.go`, `api_handler_test.go`, `internal/discord/loot_handler_test.go` — all green at the package level; no main-wired integration.

### Phase 86 — Item Picker (Dashboard Component) ❌
- `internal/itempicker/handler.go:35-43` defines `Handler` with `HandleSearch`, `HandleCreatureInventories`. Routes at `internal/itempicker/routes.go:10-19` mount `/api/campaigns/{id}/items/search` and `/api/campaigns/{id}/encounters/{eid}/creature-inventories`.
- **Package not imported anywhere outside its tests** (`grep -rn '\"github.com/ab/dndnd/internal/itempicker\"' .`). No instantiation in `cmd/dndnd/main.go`. The dashboard `dashboard/svelte/src/ItemPicker.svelte:11` calls `searchItems()` (defined in `dashboard/svelte/src/lib/api.js:317`) which targets `/api/campaigns/${campaignId}/items/search` — **this 404s** in production.
- Search source: backend handler unions only `weapons`, `armor`, `magic_items` SRD tables (`itempicker/handler.go:52-...`). **Homebrew is not federated** in the search (no homebrew query). Spec line 493: "search across SRD + homebrew" — gap.
- Custom entry: implemented client-side only (`ItemPicker.svelte:26,95-106`) — the custom item is held in component state and emitted to the parent on confirm. No backend persistence (which is fine for picker semantics).
- Creature-inventory tab: backend handler `HandleCreatureInventories` at `handler.go:135-172` exists but the route is not mounted in production.
- Tests: `internal/itempicker/handler_test.go`, `routes_test.go` — package tests pass, but the feature is unreachable.

### Phase 87 — Shops & Merchants ❌
- `internal/shops/service.go` and `handler.go` implement create / list / update / delete shops, item add/update/remove, and `HandlePostToDiscord` (handler.go:232-285) which posts a formatted item list to the campaign's `the-story` channel.
- Routes defined at `internal/shops/routes.go:10-29`: `/api/campaigns/{campaignID}/shops` etc.
- **Package not imported in production code** (`grep -rn '\"github.com/ab/dndnd/internal/shops\"' cmd internal | grep -v _test.go` returns nothing). No `shops.RegisterRoutes(...)` call in `main.go`. Dashboard `ShopBuilder.svelte` exists and calls `/api/campaigns/.../shops` (defined in `dashboard/svelte/src/lib/api.js` Shops API section starting line ~336), which 404s.
- "Post to #the-story" cannot fire because the route is never mounted (and the post-func callback is unset by default; `handler.go:26` `SetPostFunc` would need a wired Discord-poster).
- Impromptu shopping (narration + dashboard inventory adjustments) is partly covered by inventory/gold APIs that **are** wired, so the impromptu path technically works without shop templates.
- Tests: `internal/shops/service_test.go`, `handler_test.go`, `routes_test.go` — package tests pass; production unreachable.

## Cross-cutting risks

### 1. Dashboard wiring gap (loot, item picker, shops, party rest)
Four packages — `internal/loot/api_handler.go`, `internal/itempicker`, `internal/shops`, `internal/rest/party_handler.go` — are fully implemented and tested but **never instantiated** in `cmd/dndnd/main.go`. The dashboard Svelte UIs (`ItemPicker.svelte`, `ShopBuilder.svelte`, loot pool UIs in `LootPanel`/`EncounterEnd`) call routes that 404. This is the single largest cluster of un-shipped work in chunk 6 and undermines the ✅ marks in `docs/phases.md` for Phases 85, 86, 87, and 83b. The Phase 120a comment at `cmd/dndnd/main.go:683-685` ("in future phases") suggests the loot dashboard wiring is *known* deferred but the phase doc does not reflect that.

### 2. Deferred `/use` + `/give` combat costs untracked
`docs/phases.md:485` correctly notes `/use` action/bonus-action cost in combat, `/give` free-interaction-or-action cost in combat, `/give` adjacency check in combat, and the `potion_bonus_action` campaign setting are all deferred. However:
- The `PotionBonusAction` field at `internal/campaign/service.go:27` is dead code (defined but never read).
- No follow-up phase number is referenced (every other deferral in phases.md is tied to a specific phase number — e.g. 106c, 106d, 110a, 116). The deferral may slip indefinitely.
- The combat-time semantics also affect Phase 84 inventory's "DM management from dashboard" because giving items between characters in combat with adjacency requires the same machinery.

### 3. Timer goroutine leak risk (minor)
`combat.TurnTimer.Stop` at `internal/combat/timer.go:59-61` calls `close(t.stopCh)`. If `Stop()` is called twice (e.g., on shutdown signal handler races), this will panic on the second close. `cmd/dndnd/main.go:796` `defer timer.Stop()` is the only call site so single-process shutdown is fine, but tests that re-use the same timer instance could trip on this. Trivial fix: guard with `sync.Once`.

### 4. `/check` and `/save` modifier coverage incomplete
`/check` ignores its own `target` option, never charges an action in combat, and has no group / contested / passive flows in Discord. `/save` does not pass FeatureEffects, so Aura of Protection, Bless, magic item bonuses, and dodge-advantage on DEX saves never appear in the result. Both phases are marked ✅ but the user-visible behavior diverges from the spec. The fix on `/save` is small (populate `FeatureEffects` from a feature lookup the same way `internal/save/save.go` already supports).

### 5. `/interact` is a stub
Phase 74 is the only chunk-6 phase whose service exists but is not callable at all. The router-stub at `internal/discord/router.go:228-234` returns "/interact is not yet implemented." despite the phase being marked done. The fix is mechanical (write an `InteractHandler` that mirrors the structure of `EquipHandler`), but the spec's "first free, second costs action, auto-resolve vs DM queue" semantics are entirely unreachable until then.

### 6. Stealth-disadvantage gap
The armor `stealth_disadv` flag is honored only by the Hide standard action. `/check stealth` does not look up equipped armor and so returns a clean d20 even in plate mail. Spec lines 873 + 462 imply this should be auto-applied. Fix is small: `CheckHandler` reads the character's equipped armor and ORs the disadvantage flag into the roll mode.

## Recommended follow-ups

1. **Wire the four dashboard packages.** In `cmd/dndnd/main.go` near the existing `dashboard.RegisterInventoryAPI` call (line 609), add:
   - `loot.NewAPIHandler(lootSvc).RegisterRoutes(router, authMw)` (define `RegisterRoutes` on `loot.APIHandler` mirroring the URL patterns at `api_handler.go:67-238`).
   - `itempicker.RegisterRoutes(router, itempicker.NewHandler(queries), authMw)`.
   - `shops.RegisterRoutes(router, shops.NewHandler(shops.NewService(queries)), authMw)`. Then thread a Discord-poster into `shops.Handler.SetPostFunc` so "Post to #the-story" actually fires.
   - Instantiate `rest.NewPartyRestHandler(...)` and register `r.Post("/api/campaigns/{campaignID}/party-rest", h.HandlePartyRest)` on a dashboard-side router.
2. **Write `InteractHandler`** in `internal/discord/interact_handler.go`, mirroring `equip_handler.go` structure. Add a `SetInteractHandler` to `CommandRouter`. Have `combat.Interact` post to `dmNotifier` for non-auto-resolvable interactions (currently it just creates the DB row).
3. **Fix `/save` modifier coverage.** In `save_handler.go`, look up the character's active feature effects (`internal/character/features.go` already exposes them) and pass them into `save.SaveInput.FeatureEffects` with a populated `EffectCtx`.
4. **Fix `/check` for the missing flows:**
   - Parse the `target` option, validate adjacency via `internal/combat/distance.go`, and charge an action via `useResourceAndSave` when in combat (mirror `/equip` shield path).
   - Auto-apply armor `stealth_disadv` flag when `skill == "stealth"` by combining it into `RollMode` after `rollModeFromFlags`.
   - Add slash subcommand or option for `/check group` / `/check contested` (or wire them through the existing DM dashboard `skill-check-narration-deliverer` Phase 106d flow).
5. **Add a numbered follow-up phase for deferred Phase 84 work.** Track combat-time `/use` cost (action vs bonus-action depending on campaign setting), `/give` adjacency + free-interaction cost, and `potion_bonus_action` campaign-setting plumbing under a real phase entry (e.g. Phase 84b) so it shows up in the coverage map and isn't silently lost.
6. **Homebrew federation in itempicker search.** When the `homebrew_items` table is wired (Phase 99 references), union it into `itempicker.Handler.HandleSearch` so the dashboard search returns SRD ∪ homebrew (spec line 493).
7. **Heavy-armor speed penalty propagation.** When `/equip` heavy armor mid-combat, set the in-flight turn's `MovementRemainingFt` down by the penalty so the next `/move` is constrained correctly. Currently the penalty is logged but the active turn's movement budget is unchanged.
8. **Guard `TurnTimer.Stop` with `sync.Once`** to make double-stop safe — tiny patch to remove a latent panic source.
9. **Update `docs/phases.md`** to reflect actual state: Phase 74 is not callable; Phases 83b, 85 (dashboard side), 86, 87 are wired-in-package-but-not-mounted. Either the ✅ should change to a deferred marker or the wiring should land. The alternative is the playtest scenarios that depend on these (Phase 121.4) will fail at the first attempt.
