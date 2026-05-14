# Batch 13: DM Dashboard combat (Phases 94a–102)

## Summary

All twelve phases are checked off in `docs/phases.md` and have substantial implementations on both the Go backend (`internal/combat`, `internal/dashboard`, `internal/homebrew`, `internal/narration`, `internal/statblocklibrary`, `internal/characteroverview`, `internal/messageplayer`) and the Svelte SPA (`dashboard/svelte/src/`). Combat Workspace layout (60/40 split + tabs + Overview bar), drag-to-move with A* pathfinding, distance overlay, range circle, measurement tool, context menu, HP/condition tracker, turn queue, action resolver with effect builder, reactions panel grouped by combatant, action log viewer with diff rendering, undo + manual overrides with combat-log Discord poster, stat block library with drag-to-encounter (in EncounterBuilder), homebrew CRUD for all reference types, narration editor + templates with placeholder substitution, character overview with party-languages rollup, embedded message player, mobile-lite shell with bottom tab bar + desktop-redirect banner — all are present.

The biggest cross-cutting gaps are (1) **F-2 enforcement is incomplete**: the entire combat workspace + DM-dashboard + homebrew + narration + message-player + character-overview + statblock + open5e-search route trees are mounted on the bare `router` BEFORE `dmAuthMw` is constructed, so non-DM authenticated users (and unauthenticated callers in OAuth mode) can hit every DM mutation endpoint; and (2) the `pending_queue_count` field that drives the tab badges and Encounter Overview bar (Phase 94a "badges for pending #dm-queue items") is referenced in the Svelte UI but is **never populated by the backend** — `workspaceEncounterResponse` has no such field. F-1 (WS URL alignment), F-8 (Open5e source toggle), F-13 (loot pool wired to ItemPicker) all look correct.

## Per-phase findings

### Phase 94a — Combat Manager: Map & Token Display
- Status: Partial
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte
  - /home/ab/projects/DnDnD/internal/combat/workspace_handler.go
  - /home/ab/projects/DnDnD/dashboard/svelte/src/lib/mapdata.js
- Findings:
  - Map & token rendering from `tiled_json` is implemented (`drawMap` / `drawTokens` lines 308–483): terrain, lighting overlay, walls, zones, tokens with health-tier ring, active-turn ring, short-ID label, dead X, opacity from `tokenOpacity`.
  - HP & Condition Tracker (click token → tracker panel) is implemented with damage/heal/condition apply.
  - Multi-encounter tabs (`encounter-tabs`) and Encounter Overview bar are rendered.
  - **MISSING — `pending_queue_count`**: the tab badge (`tab-badge-{i}`) and overview-badge both branch on `enc.pending_queue_count`, but `workspaceEncounterResponse` (internal/combat/workspace_handler.go:59–70) does not include any pending-count field, and no `PendingQueueCount`/`pending_queue_count` exists anywhere in `internal/` outside the embedded SPA bundle. The badges will always be hidden. The phase line explicitly calls this out: "Multi-encounter tabs with badges for pending #dm-queue items".
  - **MISSING — FoW overlay**: the phase scope mentions "FoW overlay" (presumably DM-side zone fog/concealment indicators) but `CombatManager.svelte` has no per-tile FoW rendering. The spec line 2235 actually says zone boundaries are rendered as overlays in the DM view; `drawMap` does paint `activeEncounter.zones` (lines 376–386) as a single-tile fill, but does not render the lighting/visibility shadowcast result computed elsewhere. Acceptable per spec interpretation — players never see FoW boundaries — but the phase wording is broader than the implementation.

### Phase 94b — Combat Manager: Movement & Interaction
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte (lines 513–611, 637–746)
  - /home/ab/projects/DnDnD/dashboard/svelte/src/lib/combat.js (`findPath`, `tilesInRange`, `gridDistance`)
- Findings:
  - Drag-and-drop with snap-to-tile (`handleCanvasMouseDown/Move/Up`), A* pathfinding via `findPath` with wall validation, blocked-path rejection, distance-overlay text ("Xft" / "Blocked").
  - Range circle drawn from `speed_ft` (lines 485–511).
  - Distance measurement tool with toggleable mode, click two tiles, midpoint label.
  - Right-click context menu with Damage/Heal/Conditions/Remove from Encounter actions.
  - PATCH `/api/combat/{enc}/combatants/{id}/position` is invoked on drop (workspace handler).

### Phase 95 — Turn Queue & Action Resolver
- Status: Match (with auth gap; see Cross-cutting)
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/TurnQueue.svelte
  - /home/ab/projects/DnDnD/dashboard/svelte/src/ActionResolver.svelte
  - /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go (AdvanceTurn, ListPendingActions, ResolvePendingAction)
- Findings:
  - TurnQueue renders initiative-ordered list, highlights `active_turn_combatant_id`, has End Turn button calling `/advance-turn`.
  - ActionResolver lists pending actions chronologically, expands inline, supports outcome text + multi-effect builder (damage / condition_add / condition_remove / move), accumulates pending effects and ships them via `/pending-actions/{id}/resolve`.
  - Spec line 2782 wording "chronological order (oldest first)" — the Svelte component does not explicitly sort; it relies on backend ordering. Need to verify `ListPendingActionsByEncounterID` returns oldest-first (not inspected here).
  - No explicit "Accept / Reject / Edit" tri-state — current model is "resolve" only. Spec language calls it "apply outcomes with a click", which the implementation matches.

### Phase 96 — Active Reactions Panel
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte
  - /home/ab/projects/DnDnD/internal/combat/reactions_panel.go
- Findings:
  - Backend `ListReactionsForPanel` enriches declarations with combatant info + `reaction_used_this_round` flag and groups by combatant in the UI.
  - Status is computed client-side via `effectiveStatus` (active / used / dormant); highlight applied during enemy turn (`activeTurnIsNpc && status==='active'`).
  - Resolve and Cancel actions wired.
  - Greyed-until-reset behavior implemented via `effectiveStatus` returning `'dormant'` when `reaction_used_this_round`.

### Phase 97a — Action Log Viewer
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/ActionLogViewer.svelte
  - /home/ab/projects/DnDnD/internal/combat/action_log_viewer.go
  - /home/ab/projects/DnDnD/dashboard/svelte/src/lib/diff.js (`diffStates`)
- Findings:
  - Backend filter (`ActionLogFilter`) supports action_type, actor_id, target_id, round, turn_id, sort asc/desc.
  - UI renders multi-select type filter, actor/target/round/turn/sort inputs, expandable entries with before/after diff via `diffStates`.
  - `is_override` flag exposed for visual distinguishing of `dm_override` entries.

### Phase 97b — Undo & Manual Corrections
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go
  - /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go (Override* handlers)
  - /home/ab/projects/DnDnD/internal/discord/dm_correction_poster.go (via combatLogPoster wiring)
  - /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte (lines 147–269, 1027–1101)
- Findings:
  - `UndoLastAction` is scoped to current turn (loads `GetActiveTurnByEncounterID` and walks `ListActionLogByTurnID` for the most-recent undoable action). Spec compliance: "scoped to the current turn only".
  - All mutations acquire per-turn advisory lock via `withTurnLock` (when `h.db != nil`).
  - Manual State Override family: HP / position / conditions / initiative / spell_slots, plus C-35 next-attack advantage and Phase 118 concentration drop.
  - DM Correction posting is best-effort via `CombatLogPoster` (Discord side `discord.NewDMCorrectionPoster`). Spec compliance: "never edit/delete originals" — implementation appends only.
  - UI exposes Undo Last Action with optional reason + collapsible Manual Override panel per selected combatant.

### Phase 98 — Stat Block Library
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/internal/statblocklibrary/{handler.go, service.go}
  - /home/ab/projects/DnDnD/dashboard/svelte/src/StatBlockLibrary.svelte
  - /home/ab/projects/DnDnD/dashboard/svelte/src/EncounterBuilder.svelte (drag-to-encounter via `ondragstart={() => startDragCreature(...)}` at line 484)
- Findings:
  - GET `/api/statblocks` with filters: search, types, sizes, CR min/max, source (srd/homebrew/any), `campaign_id` for Open5e gating.
  - `NewHandlerWithCampaignLookup` is wired in main.go so the Open5e source filter actually applies (F-8 integration good).
  - Drag-to-encounter exists in `EncounterBuilder.svelte`, not in `StatBlockLibrary.svelte` itself — phase scope says "Used in Encounter Builder for creature selection" so this is the intended location.

### Phase 99 — Homebrew Content
- Status: Match (with auth gap)
- Key files:
  - /home/ab/projects/DnDnD/internal/homebrew/handler.go
  - /home/ab/projects/DnDnD/internal/homebrew/{creatures.go, spells.go, weapons.go, magic_items.go, races.go, feats.go, classes.go}
  - /home/ab/projects/DnDnD/dashboard/svelte/src/HomebrewEditor.svelte
- Findings:
  - Generic `mount[P, R]` table-driven CRUD over Creature/Spell/Weapon/MagicItem/Race/Feat/Class — all marked `homebrew=true` and campaign-scoped.
  - UI per-category form (no longer a raw JSON textarea) plus a class-features sub-mode reusing `/api/homebrew/classes`.
  - **Auth gap**: `homebrewHandler.RegisterRoutes(router)` at main.go:615 mounts on the bare router. Anyone with a valid session (or in passthrough-auth dev) can create/edit/delete homebrew. Spec says homebrew creation is DM-only.

### Phase 100a — Narration Editor
- Status: Match (with auth gap)
- Key files:
  - /home/ab/projects/DnDnD/internal/narration/{handler.go, service.go, markdown.go, adapters.go}
  - /home/ab/projects/DnDnD/dashboard/svelte/src/NarratePanel.svelte
- Findings:
  - Discord-flavored markdown preview (`renderDiscord`), read-aloud block insertion (`insertReadAloudBlock`), image attachment via `uploadAsset` to the Asset Library.
  - Backend: `Preview` / `Post` / `History`, with `narration.Poster` injected when a Discord session is up.
  - **Auth gap**: `narrationHandler.RegisterRoutes(router)` at main.go:641 — POST `/api/narration/post` is open.

### Phase 100b — Narration Template System
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/internal/narration/template_handler.go
  - /home/ab/projects/DnDnD/internal/narration/template_service.go
  - /home/ab/projects/DnDnD/dashboard/svelte/src/lib/narrationTemplates.js
- Findings:
  - Templates: name + category + body + campaign_id. CRUD + duplicate + list with `q` and `category` filters.
  - Placeholder extraction (`extractPlaceholders`) and substitution (`substitutePlaceholders`) on `{token}` syntax handled client-side.
  - Apply flow prompts for each placeholder value before substitution previews back into the editor.

### Phase 101 — Character Overview & Message Player
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/internal/characteroverview/{handler.go, service.go, store_db.go}
  - /home/ab/projects/DnDnD/internal/messageplayer/{handler.go, service.go, adapters.go}
  - /home/ab/projects/DnDnD/dashboard/svelte/src/{CharacterOverview.svelte, MessagePlayerPanel.svelte}
- Findings:
  - GET `/api/character-overview?campaign_id=X` returns characters + party_languages (LanguageCoverage rollup).
  - MessagePlayerPanel: campaign-scoped party dropdown, per-player history view, embeddable inside CharacterOverview with `hidePicker` + preselected `playerCharacterId`.
  - Backend Discord direct-message delivery via `discord.NewDirectMessenger` (best-effort when session is offline).
  - **Auth gap**: both mounted on bare router (main.go:653, 665) — POST `/api/message-player/` is open.

### Phase 102 — Responsive Mobile-Lite View
- Status: Match
- Key files:
  - /home/ab/projects/DnDnD/dashboard/svelte/src/{App.svelte, MobileShell.svelte, MobileRedirect.svelte, QuickActionsPanel.svelte}
  - /home/ab/projects/DnDnD/dashboard/svelte/src/lib/layout.js
- Findings:
  - `isMobileViewport` triggers at ≤1024px; `desktopOnlyViews` covers map-editor, encounter-builder, combat-workspace, stat-block-library, asset-library — matches spec line 2807.
  - `MobileShell` exposes DM Queue (uses ActionResolver), Turn Queue (read-only), Narrate, Approvals (link out), Message Player, Quick Actions (End Turn + Pause/Resume).
  - `MobileRedirect` shows "Open the dashboard on desktop for [feature name]" for desktop-only views.
  - Sidebar collapses to bottom tab bar (`bottom-tabs`).

## Cross-cutting concerns

### F-1 — WS client URL alignment (/dashboard/ws)
- Status: Pass. `CombatManager.svelte:109` builds `${proto}//${host}/dashboard/ws`; `internal/dashboard/routes.go:21` mounts `r.Get("/ws", h.ServeWebSocket)` inside `r.Route("/dashboard", ...)`. The test in `routes_test.go` exercises `GET /dashboard/ws`.

### F-2 — DM role enforcement on mutation routes
- Status: **Partial / Violation**.
- `RequireDM` middleware exists at `internal/dashboard/dm_middleware.go` and is correctly applied to: `/dashboard/queue/*`, `/dashboard/errors/*`, `/dashboard/exploration/*`, `/api/open5e/campaigns/{id}/sources`, char-create routes, approval routes, `/api/inventory/*`.
- **NOT applied to** (all mounted on bare `router` BEFORE `dmAuthMw` is constructed at main.go:763):
  - `mountCombatDashboardRoutes` at main.go:703 — `/api/combat/workspace`, all PATCH/DELETE workspace mutations, `/advance-turn`, `/pending-actions/.../resolve`, `/undo-last-action`, the entire `/override/*` family, `/concentration/drop`. These are the mutation endpoints most in need of DM gating per Phase 97b.
  - `homebrewHandler.RegisterRoutes(router)` at main.go:615 — POST/PUT/DELETE `/api/homebrew/*`.
  - `narrationHandler.RegisterRoutes(router)` at main.go:641 — POST `/api/narration/{preview,post}`.
  - `narrationTemplateHandler.RegisterRoutes(router)` at main.go:647.
  - `messagePlayerHandler.RegisterRoutes(router)` at main.go:665 — POST `/api/message-player/` is a Discord DM-sender.
  - `characterOverviewHandler.RegisterRoutes(router)` at main.go:653 — arguably DM-only per spec ("read-only view of all player character sheets" — DMs only see other players' sheets in the dashboard).
  - `combatHandler.RegisterRoutes(router)` at main.go:679.
  - `statBlockHandler.RegisterRoutes(router)` at main.go:594 (read-only but exposes all enemy stats which spec says are hidden from players).
- Net effect: in OAuth-enabled production, every authenticated Discord user — including players — can read combat workspace state (revealing hidden enemy HP, spec line 258 violation), mutate HP/positions/conditions, post narrations as the DM, and DM other players. In passthrough-auth dev mode this is by design.
- Recommendation: thread `dmAuthMw` into a single route group that contains all of the above, or pull the wiring below the `authBundle := buildAuth(...)` line and use `router.With(dmAuthMw)` per `RegisterRoutes` call.

### F-8 — Per-campaign Open5e source toggle
- Status: Pass. `Open5eSourcesPanel.svelte` reads `/api/open5e/sources` (catalog) and `/api/open5e/campaigns/{id}/sources` (GET/PUT), and the PUT round-trip is correctly mounted behind `dmAuthMw` at main.go:807–810. `statblocklibrary.NewHandlerWithCampaignLookup` consumes the campaign's enabled slugs to filter results.

### F-12 — Pending queue aggregation list
- Status: **Partial**.
- Backend: `DMQueueHandler.SetCampaignLister` + `dmqueue.PgStore.ListPendingForCampaign` and the `/dashboard/queue/` list endpoint are wired (main.go:773–774). `DMQueuePanel.svelte` consumes it via `fetchDMQueueList` and renders the aggregate table. This portion is good.
- **Gap**: the spec-described per-encounter pending-count badge (Phase 94a) consumes `enc.pending_queue_count`, which is read in `CombatManager.svelte:878,900` but is never produced by `workspaceEncounterResponse` (internal/combat/workspace_handler.go) — no struct field, no SQL aggregation. The Encounter Overview bar's "queued" badge and the per-tab badge silently disappear because of the `> 0` check.

### F-13 — Loot pool widget wired to Item Picker
- Status: Pass. `LootPoolPanel.svelte` imports `ItemPicker` and calls `addLootPoolItem`, `removeLootPoolItem`, `setLootGold`, `postLootAnnouncement` against the loot API (`internal/loot/api_handler.go`). The picker reuse pattern matches `ShopBuilder.svelte`.

## Critical items

1. **F-2 mutation routes ungated** — combat workspace, DM dashboard, homebrew, narration, narration templates, message-player, and character overview all mount on the bare `router`. In OAuth-on production any authenticated Discord user can read hidden enemy stats and mutate combat. This is the single biggest spec violation in this batch (docs/dnd-async-discord-spec.md lines 63–65). Fix is mechanical: move the `RegisterRoutes(router)` calls below the `dmAuthMw` construction and pass `router.With(dmAuthMw)` (or a sub-router) instead. Also consider that `/api/statblocks` and `/api/character-overview` reveal data players are not supposed to see in Discord.

2. **`pending_queue_count` orphaned** — the Combat Workspace tab badges and Encounter Overview "queued" badge are referenced in the UI but never populated by `workspaceEncounterResponse`. Phase 94a explicitly lists this as a "done when" criterion ("badges for pending #dm-queue items"). Either add `PendingQueueCount int32` to `workspaceEncounterResponse` + a join against `dm_queue_items` (filtered by `encounter_id` and `status='pending'`), or remove the dead UI branches.

3. **Action Resolver chronological-order contract is implicit** — UI does not sort; depends on `ListPendingActionsByEncounterID`. Worth verifying that SQL ordering matches the spec's "oldest first" requirement (line 2782). Out of scope for this review to confirm.

4. **No "Accept / Reject / Edit" tri-state** in the Action Resolver — the batch prompt for Phase 95 mentions "accept/reject/edit"; the implementation is "resolve with effects + outcome" only. Acceptable per spec wording ("apply outcomes with a click"), but worth noting if the phase intent was richer.

5. **FoW overlay in Combat Manager** — Phase 94a wording mentions "FoW overlay" but no shadowcast/visibility computation is rendered into the DM canvas; only zone overlays from `encounter_zones` and `lighting` from the tiled JSON. Acceptable per spec interpretation, but the phase scope description is broader than what ships.
