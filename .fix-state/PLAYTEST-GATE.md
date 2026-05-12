verdict: READY
reviewer: final-playtest-gate
date: 2026-05-12

## 11 scenarios traced

### 1. Combat round with opportunity attack
- Status: READY
- Trace: `/move` → `move_handler` → `combat.Service.MoveCombatant` → `combatants.position_col` DB update → `combat_log.Poster` → `#combat-log`. OA fires via `combat.Service.MaybeOpportunityAttack` on adjacent-leave detection; prompt posts through `OpportunityAttackPrompter`. `/done` → `done_handler` advances turn through `combat.Service.AdvanceTurn`.
- Citations: internal/discord/commands.go:56 (move); internal/discord/move_handler.go; internal/discord/done_handler.go; internal/discord/attack_handler.go; cmd/dndnd/discord_handlers.go:482 (`SetMapProvider`); internal/combat/opportunity_attack.go; internal/combat/handler.go RegisterRoutes at cmd/dndnd/main.go:654.

### 2. Spell with saving throw (DC vs. ability)
- Status: READY
- Trace: `/cast` → `cast_handler` → `combat.Service.CastSpell` → AoE save resolver populates `pending_actions` → DM resolves via `/api/combat/{enc}/pending-actions/{id}/resolve` → `save_handler` rolls saves → `combat_log.Poster`. `SetMaterialPromptStore` + `SetAoESaveResolver` wired.
- Citations: internal/discord/commands.go:133; internal/discord/cast_handler.go; cmd/dndnd/discord_handlers.go:268 (SetAoESaveResolver), :495 (SetMaterialPromptStore); cmd/dndnd/main.go:246 (pending-actions route).

### 3. Exploration → initiative transition
- Status: READY
- Trace: Dashboard Narrate Panel POSTs to `narrationHandler` → DM clicks Roll Initiative → `combat.Service.RollInitiative` writes order + flips `encounters.mode` → `combat_log.Poster` → `#combat-log`; narration → `#the-story` via `campaign_announcer`.
- Citations: cmd/dndnd/main.go:616 (narrationHandler.RegisterRoutes); internal/discord/campaign_announcer.go; internal/combat/initiative.go; cmd/dndnd/main.go:244 (advance-turn route).

### 4. Death save sequence
- Status: READY
- Trace: `/deathsave` → `deathsave_handler` → `combat.Service.RollDeathSave` increments `combatants.death_save_failures` / `_successes`; on 3rd fail → `status='dead'`; 3rd success → `status='stable'`; nat-20 heal-reset wired (C-43-followup closed). Output to `#combat-log` + DM via `direct_messenger`.
- Citations: internal/discord/commands.go:286; internal/discord/deathsave_handler.go; internal/combat/death_save.go; cmd/dndnd/discord_handlers.go:548; cmd/dndnd/discord_handlers.go:250 (SetStabilizeStore).

### 5. Short rest
- Status: READY
- Trace: `/rest short` → `rest_handler` posts hit-dice button prompt → on click `HandleHitDiceComponent` → `rest.Service.ShortRest` updates `current_hp` + `hit_dice_remaining`; class features reset.
- Citations: internal/discord/commands.go:395; internal/discord/rest_handler.go:113,163,209; internal/rest/service.go; cmd/dndnd/discord_handlers.go:524.

### 6. Long rest
- Status: READY
- Trace: `/rest long` → `rest_handler` → `rest.Service.LongRest` resets HP, spell_slots, restores half HD. Magic item publisher fires (H-104b closed). Long-rest prepare reminder appended (E-65 closed). Narration to `#the-story` via campaign_announcer.
- Citations: internal/discord/rest_handler.go:165,540; internal/rest/service.go; cmd/dndnd/discord_handlers.go:524.

### 7. Loot claim
- Status: READY
- Trace: `/loot` → `loot_handler.Handle` posts pool embed → Claim button → `HandleLootClaim` → `loot.Service.Claim` updates `characters.inventory` JSONB, decrements pool, deletes when empty.
- Citations: internal/discord/commands.go:492; internal/discord/router.go:499; cmd/dndnd/discord_handlers.go:533 (SetLootHandler); B-26b-loot-auto-create closed.

### 8. Item give between players
- Status: READY
- Trace: `/give item:... to:@p` → `give_handler` → DM to recipient with accept button → on accept `give.Service.Transfer` moves inventory entry, posts to `#the-story` via campaign_announcer. /give cost wiring fix in commit 0a9ef2d.
- Citations: internal/discord/commands.go:474; internal/discord/give_handler.go; cmd/dndnd/discord_handlers.go:566.

### 9. Attune / unattune
- Status: READY
- Trace: `/attune` → `attune_handler.Handle` → enforces 3-item cap → `AttuneCharacterStore.SaveAttunement` updates `characters.attuned_items`; AC/save bonuses recompute via character card service. `/unattune` reverse.
- Citations: internal/discord/commands.go:496,508; internal/discord/attune_handler.go:44,104,113; cmd/dndnd/discord_handlers.go:569,572.

### 10. Equip swap
- Status: READY
- Trace: `/equip` → `equip_handler` → `equip.Service.Equip` updates slot; armor recomputes AC; weapon becomes default for `/attack` (no `weapon:` arg). Implicit unequip on next /equip.
- Citations: internal/discord/commands.go:425; internal/discord/equip_handler.go; cmd/dndnd/discord_handlers.go:563.

### 11. Dashboard-side encounter edit during live combat
- Status: READY
- Trace: Dashboard PATCH `/api/combat/{enc}/combatants/{id}/hp|conditions|position` → `combat.WorkspaceHandler` → DB advisory-lock-serialized write → notifier posts to `#combat-log` via `combat_log.Poster`. `/status` reflects via `combat.Service.GetStatus`.
- Citations: cmd/dndnd/main.go:236-241 (workspace routes); cmd/dndnd/main.go:249-254 (override family); internal/combat/workspace_handler.go; G-94a + G-95 closed.

## Wiring spot-checks
- [x] /api/combat/workspace mounted (cmd/dndnd/main.go:237)
- [x] /api/combat/{enc}/advance-turn (cmd/dndnd/main.go:244)
- [x] /api/combat/{enc}/turn-queue — present as part of pending-actions / action-log surface (cmd/dndnd/main.go:245-247); turn-queue specifically served via `combatHandler.RegisterRoutes` at line 654 (its own /api/combat/{enc}/turn subroute)
- [x] /api/combat/{enc}/pending-actions (cmd/dndnd/main.go:245)
- [x] /api/combat/{enc}/pending-actions/{id}/resolve (cmd/dndnd/main.go:246)
- [x] /api/combat/{enc}/action-log (cmd/dndnd/main.go:247)
- [x] /api/combat/{enc}/undo-last-action (cmd/dndnd/main.go:248)
- [x] /api/combat/{enc}/override/... family (cmd/dndnd/main.go:249-253)
- [x] /api/combat/{enc}/combatants/{id}/concentration/drop (cmd/dndnd/main.go:254)
- [x] SetMapProvider (cmd/dndnd/discord_handlers.go:482)
- [x] SetStabilizeStore (cmd/dndnd/discord_handlers.go:250)
- [x] SetMaterialPromptStore (cmd/dndnd/discord_handlers.go:495)
- [x] SetAoESaveResolver (cmd/dndnd/discord_handlers.go:268)
- [x] SetZoneLookup — check + action (cmd/dndnd/discord_handlers.go:310,311)
- [x] SetClassFeaturePromptPoster (cmd/dndnd/discord_handlers.go:496)
- [x] SetClassFeatureService (cmd/dndnd/discord_handlers.go:497)
- [x] SetPCStore retire (cmd/dndnd/discord_handlers.go:415)
- [x] SetPendingStore ASI (cmd/dndnd/discord_handlers.go:404)
- [x] HydratePending ASI (cmd/dndnd/discord_handlers.go:405)
- [x] combatHandler.SetEnemyTurnNotifier invoked via wireEnemyTurnNotifier (cmd/dndnd/main.go:1087, cmd/dndnd/discord_handlers.go:43-47)
- [x] discord.WithDDBImporter invoked (internal/discord/router.go:293; threaded from cmd/dndnd/main.go:1039,1066)

## Closed/deferred ledger
- Total tasks: 91 (matches `.fix-state/tasks/` file count)
- Closed: 86
- Deferred: 5
  - H-104c-public-levelup-deferred (.fix-state/tasks/H-104c-public-levelup-deferred.md — justified, anchored to docs/phases.md:632-829)
  - E-68-fov-minor (.fix-state/tasks/E-68-fov-minor.md — Phase 68 partial-but-acceptable; front-matter still says `status: open`, ledger authoritative `deferred`)
  - C-35-dm-adv-flags (.fix-state/tasks/C-35-dm-adv-flags.md — DM dashboard adv/dis flags wired at data layer only; front-matter `open`, ledger `deferred`)
  - H-121.4-playtest-transcripts (.fix-state/tasks/H-121.4-playtest-transcripts.md — needs live session, docs/playtest-checklist.md cites task)
  - PLAYTEST-REPLAY-followup-path-handling (.fix-state/tasks/PLAYTEST-REPLAY-followup-path-handling.md — trivial path UX, absolute path works today)
- Per-task worklog files: campaign uses batch-grouped worklog naming (e.g. CMD-WIRE-impl/rev, AOE-CAST-impl/rev). 34 worklog files cover all 86 closed tasks via batch groupings; not per-task 1:1.

## Test re-run
- make test: pass (all packages PASS)
- make cover-check: pass (`OK: coverage thresholds met`, overall ≥90%, all packages ≥85%)
- make build: pass (bin/dndnd + bin/playtest-player)
- make e2e: pass (TestE2E_RecapEmptyScenario PASS, 22.134s)
- make playtest-replay TRANSCRIPT=<abs>: pass (TestE2E_ReplayFromFile PASS, 2.766s)

## Quickstart staleness
- `make build` target exists (Makefile:11-13), produces both binaries as documented.
- `make playtest-replay` target exists (Makefile:54).
- `bin/playtest-player` flags (`--record`, env vars `DISCORD_BOT_TOKEN`, `DISCORD_APPLICATION_ID`, `GUILD_ID`) match cmd/playtest-player/main.go.
- `docs/testdata/sample.tmj` present (referenced at quickstart step 7).
- Only known caveat: relative TRANSCRIPT to make playtest-replay fails — documented in PLAYTEST-REPLAY-followup-path-handling (deferred LOW). Absolute path works and is what the quickstart users will copy from the doc anyway.
- No stale targets, flags, or paths.

## Notes on deferred-task front-matter drift
Three deferred tasks (E-68, C-35-dm-adv-flags, H-121.4) have stale `status: open` in their per-task front-matter while the master ledger correctly marks them `deferred`. This is a docs-hygiene drift, not a blocker — the ledger is authoritative (per campaign rules) and each task body contains the deferral justification + spec anchor. Filing as a doc-hygiene follow-up is optional; does not affect playtest readiness.

## Verdict
READY. No P0 blockers. All five test gates green. All wiring spot-checks confirmed. All 11 playtest-checklist scenarios are traceable end-to-end through code paths that match the playtest-quickstart 30-minute flow. All 5 deferred tasks have written justifications. Recommend proceeding to live playtest. The captured transcripts target (H-121.4) flips from `pending` to `captured` during the first real session.
