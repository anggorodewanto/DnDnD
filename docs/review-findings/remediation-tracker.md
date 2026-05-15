# Remediation Tracker

## Status Legend
- `pending` — not yet started
- `in_progress` — worker subagent active
- `implemented` — code change done, awaiting review
- `review_failed` — reviewer rejected, needs rework
- `review_passed` — reviewer approved, ready to commit
- `committed` — committed to branch

---

## Findings

### Authorization & Cross-Tenant (Priority 1)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-01 | High | DM/dashboard authorization not resource-scoped | committed | 974bfde | PASS |
| F-02 | High | Map/encounter-template routes not campaign-scoped | review_passed | — | PASS |
| F-14 | Medium | Open5e cache POST endpoints public/global | pending | — | — |

### Server-Authoritative Mutation/Publish/Audit (Priority 2)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-03 | High | Combat Workspace PATCH bypasses locked service paths | review_passed | — | PASS |
| F-04 | High | Action Resolver effects bypass snapshot publishing | pending | — | — |
| F-11 | High | Enemy-turn mutations don't publish WebSocket snapshot | pending | — | — |
| F-12 | High | Enemy-turn path uses hard-coded 20x20 grid | pending | — | — |

### D&D Mechanics Correctness (Priority 3)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-05 | High | AoE DEX save cover bonuses never applied | pending | — | — |
| F-06 | High | Flying movers blocked by ground occupants | pending | — | — |
| F-07 | High | Defense fighting style AC bonus ignored | pending | — | — |
| F-08 | High | Counterspell accepts invalid low-level slots | pending | — | — |
| F-09 | High | Material components consumed before validation fails | pending | — | — |
| F-10 | High | Expired readied spells leave concentration set | pending | — | — |
| F-19 | Medium | AoE full cover not used to block targets | pending | — | — |
| F-20 | Medium | Wild Shape doesn't use beast speed | pending | — | — |
| F-21 | Medium | Timeout saves roll raw 1d20 ignoring modifiers | pending | — | — |

### DB Constraints & Data Integrity (Priority 4)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-13 | Medium | Active encounter membership not DB-enforced | pending | — | — |
| F-15 | Medium | /retire doesn't block active-combat retirement | pending | — | — |
| F-16 | Medium | Retired PC rows satisfy active registration lookups | pending | — | — |
| F-17 | Medium | /setup doesn't allow bot to post in #the-story | pending | — | — |

### UI/Persistence/Test-Harness/Coverage (Priority 5)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-18 | Medium | Map background opacity not persisted | pending | — | — |
| F-22 | Medium | Turn Builder roll fudging unreachable | pending | — | — |
| F-23 | Medium | Mobile Approvals renders wrong component | pending | — | — |
| F-24 | Medium | Phase 120a e2e omits Discord output assertions | pending | — | — |
| F-25 | Coverage | make cover-check fails: internal/errorlog below 85% | pending | — | — |

---

## Detailed Notes

### F-01: DM/dashboard authorization not resource-scoped
- **Source**: agent-01, agent-05
- **Files**: `internal/dashboard/dm_middleware.go`, `cmd/dndnd/main.go`, `internal/campaign/handler.go`, `internal/gamemap/handler.go`, `internal/encounter/handler.go`
- **Test plan**: Add middleware test verifying DM for campaign A cannot access campaign B resources
- **Implementation notes**: Added `IsCampaignDM(ctx, discordUserID, campaignID string) (bool, error)` to `DMVerifier` interface. Added `RequireCampaignDM` middleware that extracts campaign ID from chi URL params (`id` then `campaign_id`) and verifies ownership. Production verifier queries `GetCampaignByID` and checks `dm_user_id == discordUserID && status != "archived"`. Applied to campaign pause/resume routes via `RegisterRoutes` middleware parameter. Tests prove DM of campaign A gets 403 on campaign B's routes.
- **Changed files**: `internal/dashboard/dm_middleware.go`, `internal/dashboard/dm_middleware_test.go`, `internal/campaign/handler.go`, `cmd/dndnd/main.go`, `cmd/dndnd/auth_dm_routes_test.go`
- **Reviewer verdict**: **PASS**
  - **Spec conformance**: The spec (line 65) requires "System verifies the authenticated Discord user ID matches the campaign's designated DM." The `RequireCampaignDM` middleware extracts the campaign UUID from the chi URL param (`{id}` or `{campaign_id}`), calls `IsCampaignDM`, and the production implementation (`cmd/dndnd/main.go:181-196`) loads the campaign via `GetCampaignByID`, rejects archived campaigns, and checks `c.DmUserID == dmUserID`. This satisfies the spec requirement.
  - **Tenant scoping**: A DM for campaign A cannot access campaign B. The middleware denies access unless the authenticated user's Discord ID matches the `dm_user_id` column of the specific campaign being accessed. `internal/dashboard/dm_middleware.go:113-125` enforces this at the HTTP boundary before any handler runs.
  - **Regression risk**: Low. The existing `RequireDM` (generic "is any DM") remains for routes that don't target a specific campaign. `RequireCampaignDM` is additive and only applied inside the `{id}` route group via `RegisterRoutes(r, dashboard.RequireCampaignDM(...))` (`cmd/dndnd/main.go:397`). Legitimate DMs accessing their own campaigns pass through normally.
  - **Test coverage**: `TestRequireCampaignDM_RejectsDMOfDifferentCampaign` (`internal/dashboard/dm_middleware_test.go:148-172`) explicitly proves DM of campaign A gets 403 on campaign B. Additional tests cover: missing campaign ID → 403, nil verifier → 403, verifier error → 403, owner allowed → 200, `campaign_id` param fallback → 200, dev passthrough → 200.
  - **Scope note**: This fix covers campaign pause/resume routes. Map/encounter-template routes (F-02) remain unscoped and are tracked separately.

### F-02: Map/encounter-template routes not campaign-scoped
- **Source**: agent-01
- **Files**: `db/queries/maps.sql`, `db/queries/encounter_templates.sql`, `internal/gamemap/handler.go`, `internal/encounter/handler.go`
- **Test plan**: Test that get/update/delete by UUID fails without matching campaign ownership
- **Implementation notes**: Added `AND campaign_id = $N` to GetMapByID, UpdateMap, DeleteMap, GetEncounterTemplate, UpdateEncounterTemplate, DeleteEncounterTemplate SQL queries. Added `GetMapByIDUnchecked` and `GetEncounterTemplateUnchecked` queries for internal game-logic lookups (combat, pathfinding) that don't need campaign scoping. Updated service interfaces and handlers to require `campaign_id` query parameter on object routes (GET/PUT/DELETE by ID, duplicate). Updated combat `storeAdapter`, `workspaceStoreAdapter`, `mapProviderAdapter`, `castLookupAdapter`, and exploration service to use unchecked variants for internal lookups. Tests prove wrong campaign_id returns not-found/error and missing campaign_id returns 400.
- **Changed files**: `db/queries/maps.sql`, `db/queries/encounter_templates.sql`, `internal/refdata/maps.sql.go` (generated), `internal/refdata/encounter_templates.sql.go` (generated), `internal/gamemap/service.go`, `internal/gamemap/handler.go`, `internal/gamemap/service_test.go`, `internal/gamemap/handler_test.go`, `internal/encounter/service.go`, `internal/encounter/handler.go`, `internal/encounter/service_test.go`, `internal/encounter/handler_test.go`, `internal/combat/store_adapter.go`, `internal/exploration/service.go`, `internal/exploration/service_test.go`, `cmd/dndnd/main.go`, `cmd/dndnd/discord_handlers.go`, `cmd/dndnd/discord_adapters.go`, `cmd/dndnd/main_wiring_test.go`
- **Reviewer verdict**: **PASS**
  - **Spec conformance**: All map and encounter-template object routes (GET/PUT/DELETE by ID, duplicate) now require a `campaign_id` query parameter. The SQL queries (`db/queries/maps.sql:5`, `db/queries/encounter_templates.sql:5`) include `AND campaign_id = $2` on GetByID, UpdateMap includes `AND campaign_id = $8`, DeleteMap includes `AND campaign_id = $2`. Encounter template queries follow the same pattern. This satisfies the spec requirement that "all database queries are scoped by guild_id / campaign_id."
  - **Tenant scoping/security**: UUID knowledge alone cannot access another campaign's resources. Handlers parse `campaign_id` from the query string and pass it through the service layer to the DB WHERE clause (`internal/gamemap/handler.go:232-235`, `internal/gamemap/handler.go:280-283`, `internal/gamemap/handler.go:318-321`; `internal/encounter/handler.go:163-166`, `internal/encounter/handler.go:213-216`, `internal/encounter/handler.go:248-251`, `internal/encounter/handler.go:268-271`). Missing `campaign_id` returns 400. Wrong `campaign_id` returns not-found/error from the DB.
  - **Regression risk**: Low. Internal game-logic paths (combat, pathfinding, exploration) use `GetMapByIDUnchecked` / `GetEncounterTemplateUnchecked` variants that don't require campaign scoping, preserving functionality for server-side lookups that already have authorization context. These unchecked variants are not exposed via HTTP handlers.
  - **Test coverage**: Adequate. Cross-campaign access tests exist for both packages:
    - `internal/gamemap/handler_test.go:1042` `TestHandler_GetMap_WrongCampaignID` — proves wrong campaign returns 404
    - `internal/gamemap/handler_test.go:1066` `TestHandler_UpdateMap_WrongCampaignID` — proves wrong campaign update fails
    - `internal/gamemap/handler_test.go:1088` `TestHandler_DeleteMap_WrongCampaignID` — proves wrong campaign delete fails
    - `internal/gamemap/handler_test.go:1107` `TestHandler_GetMap_MissingCampaignID` — proves missing param returns 400
    - `internal/encounter/handler_test.go:674` `TestHandler_GetEncounter_WrongCampaignID` — proves wrong campaign returns 404
    - `internal/encounter/handler_test.go:691` `TestHandler_UpdateEncounter_WrongCampaignID` — proves wrong campaign update fails
    - `internal/encounter/handler_test.go:710` `TestHandler_DeleteEncounter_WrongCampaignID` — proves wrong campaign delete fails
    - `internal/encounter/handler_test.go:725` `TestHandler_DuplicateEncounter_WrongCampaignID` — proves wrong campaign duplicate fails
    - `internal/encounter/handler_test.go:740` `TestHandler_GetEncounter_MissingCampaignID` — proves missing param returns 400
  - **Required follow-up**: None.

### F-03: Combat Workspace PATCH bypasses locked service paths
- **Source**: agent-05
- **Files**: `internal/combat/workspace_handler.go`, `cmd/dndnd/main.go`
- **Test plan**: Test that workspace PATCH routes go through service layer with lock acquisition and snapshot publish
- **Implementation notes**: Added `WorkspaceCombatService` interface to `WorkspaceHandler` exposing `UpdateCombatantHP`, `UpdateCombatantConditions`, `UpdateCombatantPosition`, and `GetCombatant`. Routed all three PATCH handlers through the service instead of raw store writes. The service methods internally publish WebSocket snapshots and run domain hooks (concentration saves, silence-zone checks, incapacitation breaks). Updated `NewWorkspaceHandler` signature to accept the service; wired `*combat.Service` in `mountCombatDashboardRoutes`. Added 3 focused tests (`TestWorkspaceHandler_F03_*`) proving service routing.
- **Changed files**: `internal/combat/workspace_handler.go`, `internal/combat/workspace_handler_test.go`, `internal/combat/dm_dashboard_handler_test.go`, `cmd/dndnd/main.go`
- **Reviewer verdict**: **PASS**
  - **Spec conformance**: All three workspace PATCH routes (HP, conditions, position) now call through the `WorkspaceCombatService` interface (`internal/combat/workspace_handler.go:318-343` → `h.svc.UpdateCombatantHP`, `:346-382` → `h.svc.UpdateCombatantConditions`, `:385-421` → `h.svc.UpdateCombatantPosition`). The handler no longer calls raw store methods for mutations. The `NewWorkspaceHandler` constructor requires the service (`workspace_handler.go:60`), and production wiring passes `*combat.Service` at `cmd/dndnd/main.go:459`.
  - **Server-authoritative correctness**: `Service.UpdateCombatantHP` (`service.go:757-770`) persists and publishes. `Service.UpdateCombatantConditions` (`service.go:842-855`) persists and publishes. `Service.UpdateCombatantPosition` (`service.go:785-792`) delegates to `UpdateCombatantPositionWithTriggers` (`service.go:794-840`) which runs zone-anchor follow, silence-zone concentration break, zone enter triggers, and zone damage application before publishing. All domain hooks are triggered.
  - **Snapshot publishing**: Each service method calls `s.publish(ctx, c.EncounterID)` which invokes `publisher.PublishEncounterSnapshot` (`service.go:597-603`), satisfying the WebSocket state-sync spec requirement.
  - **Regression risk**: Low. All 19 `TestWorkspaceHandler_*` tests pass (verified via `go test`). The `WorkspaceStore` interface still exists for read-only operations (GET workspace, delete combatant). Only the three PATCH mutation paths were rerouted.
  - **Test coverage**: Three focused tests (`workspace_handler_test.go:1179-1286`) prove service routing: `TestWorkspaceHandler_F03_HPPatchRoutesViaService`, `TestWorkspaceHandler_F03_ConditionsPatchRoutesViaService`, `TestWorkspaceHandler_F03_PositionPatchRoutesViaService`. Each uses a mock `WorkspaceCombatService`, asserts the service method is called with correct arguments, and verifies HTTP 200 response.
  - **Minor note**: `DeleteCombatant` still calls `h.store.DeleteCombatant` directly (`workspace_handler.go:424-434`) and does not publish a snapshot. This is outside the original F-03 finding scope (which targeted HP/conditions/position PATCH) but could be a follow-up item.
  - **Required follow-up**: None for F-03 scope. Optional: route `DeleteCombatant` through service for snapshot publish consistency.

### F-04: Action Resolver effects bypass snapshot publishing
- **Source**: agent-04
- **Files**: `internal/combat/dm_dashboard_handler.go`, `dashboard/svelte/src/ActionResolver.svelte`
- **Test plan**: Test that ResolvePendingAction publishes snapshot and records before/after state
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-05: AoE DEX save cover bonuses never applied
- **Source**: agent-02
- **Files**: `internal/combat/aoe.go`, `db/queries/pending_saves.sql`
- **Test plan**: Test that cover bonus is applied to save total during resolution
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-06: Flying movers blocked by ground occupants
- **Source**: agent-02
- **Files**: `internal/combat/movement.go`, `internal/pathfinding/pathfinding.go`
- **Test plan**: Test that flying combatant can move through/to squares with ground occupants
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-07: Defense fighting style AC bonus ignored
- **Source**: agent-03
- **Files**: `internal/combat/attack.go`, `internal/combat/feature_integration.go`
- **Test plan**: Test that Defense fighting style +1 AC is applied during attack resolution
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-08: Counterspell accepts invalid low-level slots
- **Source**: agent-03
- **Files**: `internal/combat/counterspell.go`
- **Test plan**: Test that ResolveCounterspell rejects slot levels below 3
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-09: Material components consumed before validation fails
- **Source**: agent-03
- **Files**: `internal/combat/spellcasting.go`
- **Test plan**: Test that material/gold deduction happens after all validations pass
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-10: Expired readied spells leave concentration set
- **Source**: agent-03
- **Files**: `internal/combat/readied_action.go`
- **Test plan**: Test that expiring a readied spell clears concentration columns
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-11: Enemy-turn mutations don't publish WebSocket snapshot
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`
- **Test plan**: Test that ExecuteEnemyTurn publishes snapshot after mutations
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-12: Enemy-turn path uses hard-coded 20x20 grid
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`
- **Test plan**: Test that GenerateEnemyTurnPlan loads the encounter's actual map
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-13: Active encounter membership not DB-enforced
- **Source**: agent-05
- **Files**: `db/migrations/`, `db/queries/encounters.sql`
- **Test plan**: Migration adds partial unique index; test duplicate insert fails
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-14: Open5e cache POST endpoints public/global
- **Source**: agent-05
- **Files**: `cmd/dndnd/main.go`, `internal/open5e/handler.go`
- **Test plan**: Test that POST cache endpoints require DM auth
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-15: /retire doesn't block active-combat retirement
- **Source**: agent-01
- **Files**: `internal/discord/retire_handler.go`
- **Test plan**: Test that /retire returns error during active combat
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-16: Retired PC rows satisfy active registration lookups
- **Source**: agent-01
- **Files**: `db/queries/player_characters.sql`
- **Test plan**: Test that registration lookup excludes retired rows
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-17: /setup doesn't allow bot to post in #the-story
- **Source**: agent-01
- **Files**: `internal/discord/setup.go`
- **Test plan**: Test that #the-story permissions include bot SendMessages
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-18: Map background opacity not persisted
- **Source**: agent-01
- **Files**: `dashboard/svelte/src/MapEditor.svelte`, `db/queries/maps.sql`
- **Test plan**: Test that opacity is saved and restored on map load
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-19: AoE full cover not used to block targets
- **Source**: agent-02
- **Files**: `internal/combat/aoe.go`, `internal/combat/cover.go`
- **Test plan**: Test that targets with full cover from AoE origin are excluded
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-20: Wild Shape doesn't use beast speed
- **Source**: agent-03
- **Files**: `internal/combat/wildshape.go`, `internal/combat/turnresources.go`
- **Test plan**: Test that wild-shaped combatant uses beast speed for movement
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-21: Timeout saves roll raw 1d20 ignoring modifiers
- **Source**: agent-03
- **Files**: `internal/combat/timer_resolution.go`
- **Test plan**: Test that auto-resolved saves include ability modifier + proficiency
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-22: Turn Builder roll fudging unreachable
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`, `internal/combat/turn_builder.go`, `dashboard/svelte/src/TurnBuilder.svelte`
- **Test plan**: Test that rolls are generated at plan time so DM can fudge before confirm
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-23: Mobile Approvals renders wrong component
- **Source**: agent-04
- **Files**: `dashboard/svelte/src/MobileShell.svelte`
- **Test plan**: Test that mobile approvals tab renders CharacterApprovalQueue
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-24: Phase 120a e2e omits Discord output assertions
- **Source**: agent-05
- **Files**: `cmd/dndnd/e2e_scenarios_test.go`
- **Test plan**: Add Discord output assertions for registration, movement, loot scenarios
- **Implementation notes**: —
- **Reviewer verdict**: —

### F-25: Coverage gate - internal/errorlog below 85%
- **Source**: 00-summary
- **Files**: `internal/errorlog/`
- **Test plan**: Add tests to bring internal/errorlog above 85% coverage
- **Implementation notes**: —
- **Reviewer verdict**: —

---

## Final Reviewer Verdict
- **Status**: pending
- **Notes**: —
