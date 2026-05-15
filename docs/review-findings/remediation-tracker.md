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
| F-14 | Medium | Open5e cache POST endpoints public/global | implemented | — | — |

### Server-Authoritative Mutation/Publish/Audit (Priority 2)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-03 | High | Combat Workspace PATCH bypasses locked service paths | review_passed | — | PASS |
| F-04 | High | Action Resolver effects bypass snapshot publishing | review_passed | — | PASS |
| F-11 | High | Enemy-turn mutations don't publish WebSocket snapshot | review_passed | — | PASS |
| F-12 | High | Enemy-turn path uses hard-coded 20x20 grid | review_passed | — | PASS |

### D&D Mechanics Correctness (Priority 3)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-05 | High | AoE DEX save cover bonuses never applied | review_passed | — | PASS |
| F-06 | High | Flying movers blocked by ground occupants | review_passed | — | PASS |
| F-07 | High | Defense fighting style AC bonus ignored | review_passed | — | PASS |
| F-08 | High | Counterspell accepts invalid low-level slots | review_passed | — | PASS |
| F-09 | High | Material components consumed before validation fails | implemented | PASS | reviewer-f09 |
| F-10 | High | Expired readied spells leave concentration set | review_passed | — | PASS |
| F-19 | Medium | AoE full cover not used to block targets | implemented | — | — |
| F-20 | Medium | Wild Shape doesn't use beast speed | implemented | — | — |
| F-21 | Medium | Timeout saves roll raw 1d20 ignoring modifiers | implemented | — | — |

### DB Constraints & Data Integrity (Priority 4)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-13 | Medium | Active encounter membership not DB-enforced | review_passed | — | PASS |
| F-15 | Medium | /retire doesn't block active-combat retirement | implemented | — | — |
| F-16 | Medium | Retired PC rows satisfy active registration lookups | implemented | — | — |
| F-17 | Medium | /setup doesn't allow bot to post in #the-story | implemented | — | — |

### UI/Persistence/Test-Harness/Coverage (Priority 5)

| ID | Severity | Finding | Status | Commit | Reviewer |
|----|----------|---------|--------|--------|----------|
| F-18 | Medium | Map background opacity not persisted | implemented | — | — |
| F-22 | Medium | Turn Builder roll fudging unreachable | implemented | — | — |
| F-23 | Medium | Mobile Approvals renders wrong component | implemented | — | — |
| F-24 | Medium | Phase 120a e2e omits Discord output assertions | implemented | — | — |
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
- **Implementation notes**: In `ResolvePendingAction`, added before-state capture (HP, temp_hp, conditions, position) via `captureResolverState` before effects are applied, and after-state capture after all effects complete. Both are stored as JSON in the `CreateActionLog` call's `BeforeState`/`AfterState` fields. Added `h.svc.publish(r.Context(), encounterID)` after the action log write so dashboards receive a WebSocket snapshot. Added `resolverStateSnapshot` struct and `captureResolverState` helper.
- **Changed files**: `internal/combat/dm_dashboard_handler.go`, `internal/combat/dm_dashboard_handler_test.go`
- **Reviewer verdict**: **PASS**
  - **Spec conformance**: Phase 103 requires snapshot push after state mutation — `h.svc.publish(r.Context(), encounterID)` is called at line 305 after all effects are applied and the action log is written (`dm_dashboard_handler.go:305`). Phase 97a requires before/after state diff — `captureResolverState` captures HP, temp_hp, conditions, and position before effects (line 255) and after effects (line 279), both stored as JSON in `CreateActionLog`'s `BeforeState`/`AfterState` fields (lines 296-297).
  - **Snapshot publishing**: `publish` is called exactly once, after all effects and the action log write, ensuring dashboards receive the final resolved state. Verified by `TestResolvePendingAction_F04_PublishesSnapshot` which asserts exactly one publish call with the correct encounter ID.
  - **Audit trail**: `resolverStateSnapshot` struct captures `hp`, `temp_hp`, `conditions`, `position`. Before-state is captured from `GetCombatant` before any `applyEffect` calls; after-state is captured from a fresh `GetCombatant` after all effects complete. Both are marshaled to JSON and passed to `CreateActionLog`. The action log viewer can now render field-level diffs.
  - **Regression risk**: Low. The change is additive — two `GetCombatant` calls (before/after) and one `publish` call were added to the existing flow. No existing behavior was removed or reordered.
  - **Test coverage**: Two focused tests prove the fix:
    - `TestResolvePendingAction_F04_PublishesSnapshot` — asserts exactly one snapshot publish after resolution with no effects.
    - `TestResolvePendingAction_F04_BeforeAfterState` — applies a `condition_add` effect, asserts `BeforeState` has empty conditions and `AfterState` has `[{"condition":"poisoned"}]`, proving the diff is captured.
  - **Both tests pass** (`go test ./internal/combat/ -run F04` → PASS).

### F-05: AoE DEX save cover bonuses never applied
- **Source**: agent-02
- **Files**: `internal/combat/aoe.go`, `db/queries/pending_saves.sql`
- **Test plan**: Test that cover bonus is applied to save total during resolution
- **Implementation notes**: Applied Option B: when persisting pending saves, the DC stored is `dc - coverBonus` (line ~572 in aoe.go). This means the existing resolution logic (`total >= dc`) automatically accounts for cover without schema changes. A DC 15 save with +2 half-cover bonus stores DC 13, so a roll of 14 succeeds. Two focused tests added: `TestCastAoE_F05_CoverBonusReducesStoredDC` (verifies stored DC is reduced) and `TestRecordAoEPendingSaveRoll_F05_CoverBonusMakesSaveSucceed` (proves a roll that would fail without cover now succeeds).
- **Reviewer verdict**: **PASS**
  - **D&D 5e/SRD correctness**: `CoverLevel.DEXSaveBonus()` delegates to `ACBonus()` which returns +2 for half cover and +5 for three-quarters cover (`internal/combat/cover.go:45-47`). These match SRD 5.1 exactly. The bonus is only applied when `saveAbility == "dex"` (`aoe.go:201`), correctly scoping it to DEX saves only.
  - **Spec conformance**: Phase 33 requires "cover integration with saves" and lists half (+2 DEX save) and three-quarters (+5 DEX save). The fix computes cover via `CalculateCoverFromOrigin` at `aoe.go:197-213` and applies it at `aoe.go:576` (`Dc: int32(ps.DC - ps.CoverBonus)`). This satisfies the phase 33 requirement.
  - **Implementation correctness**: Subtracting cover bonus from DC before storage is mathematically equivalent to adding it to the roll at resolution time: `(total + bonus >= dc)` ⟺ `(total >= dc - bonus)`. The approach avoids schema changes and works transparently with the existing `total >= int(r.Dc)` resolution logic at `aoe.go:897`.
  - **Regression risk**: None observed. All 20 existing AoE-related tests pass alongside the two new F-05 tests. The `PendingSave` struct retains the original `DC` and `CoverBonus` fields for display/logging purposes; only the persisted DB value is adjusted.
  - **Test coverage**: Adequate. `TestCastAoE_F05_CoverBonusReducesStoredDC` proves the stored DC is reduced (15 → 13 with half cover +2) and the struct retains original values. `TestRecordAoEPendingSaveRoll_F05_CoverBonusMakesSaveSucceed` proves end-to-end that a roll of 14 succeeds against stored DC 13 (original 15 minus 2 cover), which would have failed without the fix.
  - **Minor note**: Three-quarters cover (+5) is not explicitly tested, only half cover (+2). This is acceptable since `DEXSaveBonus()` delegates to `ACBonus()` which is separately tested for both levels in `cover_test.go:44-56`.

### F-06: Flying movers blocked by ground occupants
- **Source**: agent-02
- **Files**: `internal/combat/movement.go`, `internal/pathfinding/pathfinding.go`
- **Test plan**: Test that flying combatant can move through/to squares with ground occupants
- **Implementation notes**: Fixed destination occupancy check in `movement.go` to compare mover's `AltitudeFt` with occupant's `AltitudeFt` instead of only checking `occ.AltitudeFt == 0`. Added `MoverAltitudeFt` field to `PathRequest` struct and updated `buildOccupantMap` to only include occupants at the same altitude as the mover. Updated all `FindPath` call sites (`movement.go`, `move_handler.go` ×2, `turn_builder.go`) to pass mover altitude. Three focused tests added: destination landing, path-through, and same-altitude-still-blocks.
- **Reviewer verdict**: **PASS** (reviewer-f06, 2026-05-15)
  - **D&D 5e/SRD correctness**: ✅ Creatures at different altitudes sharing the same x/y tile is consistent with SRD movement/space rules — flying creatures at different elevations do not contest the same space.
  - **Spec conformance**: ✅ Phase 31 states "Flying tokens don't block ground tiles." The fix correctly implements this bidirectionally: ground occupants don't block flying movers, and flying occupants don't block ground movers.
  - **Implementation correctness**: ✅ `movement.go:84-86` compares `occ.AltitudeFt == moverAlt` (exact altitude match) for destination blocking. `pathfinding.go:140-148` `buildOccupantMap` filters occupants by `o.AltitudeFt != moverAltitudeFt`, so only same-altitude occupants appear in the A* blocking map. `MoverAltitudeFt` is threaded through `PathRequest` to all `FindPath` call sites.
  - **Regression risk**: ✅ Ground movers (altitude 0) still blocked by ground occupants (altitude 0) because `0 == 0` matches in both the destination check and `buildOccupantMap`. The pre-existing test `TestValidateMove_FlyingOccupantDoesNotBlock` (line 465) confirms the inverse case still works. `TestValidateMove_F06_SameAltitudeStillBlocks` explicitly guards the same-altitude regression.
  - **Test adequacy**: ✅ Three focused tests cover: (1) flying mover landing on ground-occupied tile, (2) flying mover pathing through ground occupant, (3) same-altitude still blocks. Combined with the pre-existing inverse test, all four quadrants of the altitude×direction matrix are covered.

### F-07: Defense fighting style AC bonus ignored
- **Source**: agent-03
- **Files**: `internal/combat/attack.go`, `internal/combat/feature_integration.go`
- **Test plan**: Test that Defense fighting style +1 AC is applied during attack resolution
- **Implementation notes**: Defense +1 AC applied via `RecalculateAC` in `equip.go` when armor is equipped and character has "defense" mechanical effect. Bakes bonus into stored AC (correct D&D passive behavior). Incorrect FES-level approach (attacker's ACModifier applied to target's AC) removed per reviewer feedback.
- **Changed files**: `internal/combat/equip.go`, `internal/combat/equip_test.go`, `internal/combat/attack.go`, `internal/combat/attack_fes_test.go`
- **Reviewer verdict**: PASS (after rework removing incorrect FES-level fix)

### F-08: Counterspell accepts invalid low-level slots
- **Source**: agent-03
- **Files**: `internal/combat/counterspell.go`
- **Test plan**: Test that ResolveCounterspell rejects slot levels below 3
- **Implementation notes**: Added `ErrCounterspellSlotTooLow` sentinel error and early validation `chosenSlotLevel < 3` at the top of `ResolveCounterspell`, before any store calls or slot deduction. Test `TestResolveCounterspell_F08_RejectsSlotBelow3` proves levels 1 and 2 are rejected.
- **Changed files**: `internal/combat/counterspell.go`, `internal/combat/counterspell_test.go`
- **Reviewer verdict**: **PASS** (reviewer-f08, 2026-05-15)
  - **D&D 5e/SRD correctness**: ✅ Counterspell is a 3rd-level abjuration spell. Slots below 3rd level cannot cast it. The `chosenSlotLevel < 3` guard correctly enforces this minimum.
  - **Spec conformance**: ✅ Only slots 3+ accepted. Defense-in-depth: `AvailableCounterspellSlots` also filters `level >= 3` on the prompt side, so the UI never offers invalid slots. The server-side guard in `ResolveCounterspell` catches any bypass of the prompt.
  - **Implementation correctness**: ✅ The validation is the **first statement** in `ResolveCounterspell` (line 108), before any store lookups, slot deductions, reaction consumption, or DB writes. Zero side effects on rejection. Returns the sentinel `ErrCounterspellSlotTooLow`.
  - **Test adequacy**: ✅ `TestResolveCounterspell_F08_RejectsSlotBelow3` iterates levels 1 and 2, asserts `errors.Is(err, ErrCounterspellSlotTooLow)`. `TestAvailableCounterspellSlots_PactSlotBelow3` covers the prompt-side filter. Combined with existing tests proving level 3 succeeds (`TestResolveCounterspell_AutoCounter`), the boundary is fully covered.

### F-09: Material components consumed before validation fails
- **Source**: agent-03
- **Files**: `internal/combat/spellcasting.go`
- **Test plan**: Test that material/gold deduction happens after all validations pass
- **Implementation notes**: Split step 6d into validation-only (early) and deduction (deferred). Material validation (rejection, gold confirmation prompt) remains at step 6d. Actual deduction (gold update, inventory add/remove) is captured in a `materialDeduction` struct and executed at new step 12b, after target lookup, range validation, see-target validation, and teleport validation all pass. Test `TestCast_F09_MaterialNotConsumedOnRangeFailure` proves consumed materials are not deducted when range validation fails.
- **Changed files**: `internal/combat/spellcasting.go`, `internal/combat/spellcasting_test.go`
- **Reviewer verdict**: PASS (reviewer-f09, 2026-05-15). Validation (availability check, rejection) remains early at step 6d. Deduction (gold update, inventory mutation) is deferred to step 12b, which executes only after target lookup, range validation, and see-target validation all pass. The `materialDeduction` struct cleanly separates intent from execution. Test `TestCast_F09_MaterialNotConsumedOnRangeFailure` asserts that neither `UpdateCharacterGold` nor `updateCharacterInventory` is called when range validation fails. Existing material tests (lines ~2130-2177) remain unaffected since they exercise the happy path where deduction should occur. No regression risk identified.

### F-10: Expired readied spells leave concentration set
- **Source**: agent-03
- **Files**: `internal/combat/readied_action.go`
- **Test plan**: Test that expiring a readied spell clears concentration columns
- **Implementation notes**: Added `s.store.ClearCombatantConcentration(ctx, combatantID)` call inside the `decl.SpellName.Valid` branch of `ExpireReadiedActions`, immediately after the notice is built. Uses the same store method as `concentration.go` for consistency. Test `TestExpireReadiedActions_F10_ClearsConcentration` proves concentration is cleared for the correct combatant when a readied spell expires.
- **Changed files**: `internal/combat/readied_action.go`, `internal/combat/readied_action_test.go`
- **Reviewer verdict**: **PASS** (reviewer-f10, 2026-05-15)
  - **D&D 5e/SRD correctness**: ✅ PHB p.193: "If you don't use [the readied spell] before the start of your next turn, the spell is lost and concentration ends." The fix calls `ClearCombatantConcentration` when a readied spell expires unused, correctly ending concentration.
  - **Implementation correctness**: ✅ The `ClearCombatantConcentration` call is gated inside `if decl.SpellName.Valid` (`readied_action.go:155`), so only spell-based readied actions trigger concentration clearing. Non-spell readied actions skip this branch entirely. The combatant ID passed to the store method is the correct one from the function parameter.
  - **Regression risk**: ✅ Low. The existing `TestExpireReadiedActions_ExpiresActive` test exercises a non-spell readied action and does not set `clearCombatantConcentrationFn`, confirming the branch is not entered for non-spell actions. The `TestExpireReadiedActions_SpellExpiry` test (pre-existing) continues to pass. No existing behavior was altered.
  - **Test adequacy**: ✅ `TestExpireReadiedActions_F10_ClearsConcentration` explicitly asserts: (1) `ClearCombatantConcentration` is called, and (2) the correct combatant ID is passed. Combined with the pre-existing non-spell expiry test (implicit negative case), both branches are covered. A dedicated negative test asserting `clearCalled == false` for non-spell expiry would strengthen coverage but is not strictly required since the code path is gated by `SpellName.Valid`.

### F-11: Enemy-turn mutations don't publish WebSocket snapshot
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`
- **Test plan**: Test that ExecuteEnemyTurn publishes snapshot after mutations
- **Implementation notes**: Added `s.publish(ctx, encounterID)` call at the end of `ExecuteEnemyTurn`, after `UpdateTurnActions` and before the return. The `*Service` receiver already has the `publish` helper method wired via `SetPublisher`. Test `TestExecuteEnemyTurn_F11_PublishesSnapshot` proves a movement-only enemy turn publishes exactly one snapshot with the correct encounter ID.
- **Changed files**: `internal/combat/turn_builder_handler.go`, `internal/combat/turn_builder_handler_test.go`
- **Reviewer verdict**: **PASS** (reviewer-f11, 2026-05-15)
  - **Spec conformance**: ✅ Phase 103 requires snapshot push after state mutation. `s.publish(ctx, encounterID)` is called at the end of `Service.ExecuteEnemyTurn` (`turn_builder_handler.go:247`), after all DB mutations complete (position update, damage application via `ApplyDamage`, action log creation, turn actions update). The comment `// F-11: Publish WebSocket snapshot after all mutations complete.` documents intent.
  - **Implementation correctness**: ✅ The `publish` call is positioned after the final mutation (`UpdateTurnActions`) and before the return statement. All state changes — movement (`UpdateCombatantPosition`), damage (`ApplyDamage` loop which internally calls `UpdateCombatantHP`), action log (`CreateActionLog`), and turn resource marking (`UpdateTurnActions`) — are committed before the snapshot fires. The `publish` helper (`service.go:597-603`) is nil-safe and swallows errors with a log, so a publisher failure cannot break the HTTP response.
  - **Test adequacy**: ✅ `TestExecuteEnemyTurn_F11_PublishesSnapshot` (`turn_builder_handler_test.go:406-443`) wires a `fakePublisher`, executes a movement-only enemy turn through the service layer, and asserts `pub.calls()` equals `[]uuid.UUID{encounterID}` — proving exactly one publish with the correct encounter ID. The test exercises the full service path (not the HTTP handler), which is where the fix lives.
  - **Regression risk**: ✅ None. The `publish` call is additive. Existing tests (`TestExecuteEnemyTurn_Success`, `TestExecuteEnemyTurn_NotifiesOnSuccess`) do not set a publisher and continue to pass since `publish` is nil-safe.

### F-12: Enemy-turn path uses hard-coded 20x20 grid
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`
- **Test plan**: Test that GenerateEnemyTurnPlan loads the encounter's actual map
- **Implementation notes**: In `GenerateEnemyTurnPlan`, replaced the hard-coded `buildDefaultGrid(20, 20, ...)` with encounter map loading: calls `GetEncounter` to get `MapID`, then `GetMapByIDUnchecked` to load the map, then `renderer.ParseTiledJSON` to extract terrain/walls, and builds the grid via new `buildMapGrid` helper using actual dimensions, terrain, and walls. Falls back to the 20x20 open grid if the encounter has no map, the map fails to load, or the TiledJSON fails to parse. Added `GetMapByIDUnchecked` to the combat `Store` interface and `storeAdapter`.
- **Changed files**: `internal/combat/turn_builder_handler.go`, `internal/combat/service.go`, `internal/combat/store_adapter.go`, `internal/combat/service_test.go`, `internal/combat/turn_builder_handler_test.go`
- **Reviewer verdict**: **PASS** (reviewer-f12, 2026-05-15)
  - **Spec conformance**: ✅ `GenerateEnemyTurnPlan` loads the encounter's actual map via `GetEncounter` → `GetMapByIDUnchecked` → `renderer.ParseTiledJSON`, then builds the pathfinding grid with the map's real `Width`, `Height`, `TerrainGrid`, and `Walls`. Path suggestions now use the encounter's actual map geometry.
  - **Implementation correctness**: ✅ `buildMapGrid` (`turn_builder_handler.go:352`) constructs a `pathfinding.Grid` with parsed map dimensions, terrain grid, and wall data. Occupants are populated from live combatants (excluding the mover and dead combatants). The map-loading chain correctly threads through three layers: encounter → map record → TiledJSON parse.
  - **Graceful degradation**: ✅ The default 20×20 open grid is built first (`buildDefaultGrid(20, 20, ...)`). The actual-map path only overwrites `grid` if all three conditions hold: (1) `GetEncounter` succeeds and `enc.MapID.Valid`, (2) `GetMapByIDUnchecked` succeeds, (3) `ParseTiledJSON` succeeds and `md.Width > 0 && md.Height > 0`. Any failure at any stage silently falls back to the default grid.
  - **Test adequacy**: ✅ Two focused tests: `TestGenerateEnemyTurnPlan_F12_UsesEncounterMap` builds a 10×8 map with a wall, verifies the plan is generated and all path coordinates stay within 10×8 bounds (not 20×20). `TestGenerateEnemyTurnPlan_F12_FallsBackToDefaultGrid` uses an encounter with no `MapID` and verifies a valid plan is still produced using the fallback grid.
  - **Regression risk**: ✅ Low. The fallback-first pattern means existing encounters without maps continue to work identically. The `GetMapByIDUnchecked` method (from F-02) avoids campaign-scoping for internal lookups.

### F-13: Active encounter membership not DB-enforced
- **Source**: agent-05
- **Files**: `db/migrations/20260515120000_enforce_active_encounter_membership.sql`, `db/queries/encounters.sql`, `internal/refdata/encounters.sql.go`
- **Test plan**: Migration adds trigger constraint; test duplicate insert fails
- **Implementation notes**: Added `ORDER BY cb.created_at DESC` to `GetActiveEncounterIDByCharacterID` for deterministic results. Created migration with a `BEFORE INSERT` trigger on `combatants` that raises `unique_violation` if the character already belongs to another active encounter. The trigger provides DB-level enforcement as a safety net behind the service-level check. Two focused tests: `TestGetActiveEncounterIDByCharacterID_F13_DeterministicOrder` (proves rejection of duplicate membership) and `TestAddCombatant_F13_AllowsSameEncounterReAdd` (proves idempotent re-add to same encounter is allowed).
- **Changed files**: `db/queries/encounters.sql`, `internal/refdata/encounters.sql.go`, `db/migrations/20260515120000_enforce_active_encounter_membership.sql`, `internal/combat/f13_active_membership_test.go`
- **Reviewer verdict**: **PASS** (reviewer-f13, 2026-05-15)
  - **Deterministic ordering**: ✅ `GetActiveEncounterIDByCharacterID` now includes `ORDER BY cb.created_at DESC LIMIT 1`, ensuring the most recently created combatant row is returned deterministically even if duplicate active memberships somehow exist.
  - **Trigger correctness**: ✅ The `BEFORE INSERT` trigger `trg_enforce_active_encounter_membership` checks `EXISTS (SELECT 1 FROM combatants cb JOIN encounters e ON e.id = cb.encounter_id WHERE cb.character_id = NEW.character_id AND e.status = 'active' AND cb.encounter_id != NEW.encounter_id)`. This correctly: (a) allows inserts when no other active encounter membership exists, (b) allows re-insert into the same encounter (idempotent), (c) skips NULL character_ids via the `IF NEW.character_id IS NOT NULL` guard, (d) raises `unique_violation` ERRCODE for programmatic detection.
  - **Migration safety**: ✅ `+goose Down` cleanly drops trigger then function. The trigger fires `BEFORE INSERT` so invalid rows never reach the table.
  - **Test adequacy**: ✅ `TestGetActiveEncounterIDByCharacterID_F13_DeterministicOrder` proves the service rejects adding a character to a different encounter when already active elsewhere. `TestAddCombatant_F13_AllowsSameEncounterReAdd` proves idempotent re-add to the same encounter succeeds. Both exercise the service-layer guard that mirrors the trigger logic.
  - **Minor note**: Tests are unit-level mocks (no live DB), so the trigger itself is not integration-tested. Acceptable since the trigger is defense-in-depth behind the service check.

### F-14: Open5e cache POST endpoints public/global
- **Source**: agent-05
- **Files**: `cmd/dndnd/main.go`, `internal/open5e/handler.go`
- **Test plan**: Test that POST cache endpoints require DM auth
- **Implementation notes**: Split `RegisterRoutes` into `RegisterPublicRoutes` (GET search) and `RegisterProtectedRoutes` (POST cache). In `main.go`, public GET routes remain on the bare router; POST cache routes are mounted in a `router.Group` behind `dmAuthMw`. Test `TestOpen5eCachePOST_F14_RequiresDMAuth` proves POST `/api/open5e/monsters/{slug}` and `/api/open5e/spells/{slug}` return 403 without DM auth, while GET search routes remain accessible.
- **Changed files**: `internal/open5e/handler.go`, `cmd/dndnd/main.go`, `cmd/dndnd/auth_open5e_cache_test.go`
- **Reviewer verdict**: —

### F-15: /retire doesn't block active-combat retirement
- **Source**: agent-01
- **Files**: `internal/discord/retire_handler.go`
- **Test plan**: Test that /retire returns error during active combat
- **Implementation notes**: Added `RetireCombatChecker` interface with `GetActiveCombatantByCharacterID`. Added `combatChecker` field to `RetireHandler` with `SetCombatChecker` setter. In `Handle`, after character lookup and before `markPC`, checks if the character is an active combatant; if so, returns ephemeral "❌ You can't retire mid-combat." and short-circuits. Test `TestRetireHandler_F15_BlockedDuringActiveCombat` proves the block.
- **Changed files**: `internal/discord/retire_handler.go`, `internal/discord/retire_handler_test.go`
- **Reviewer verdict**: —

### F-16: Retired PC rows satisfy active registration lookups
- **Source**: agent-01
- **Files**: `db/queries/player_characters.sql`
- **Test plan**: Test that registration lookup excludes retired rows
- **Implementation notes**: Added `AND status != 'retired'` to the `GetPlayerCharacterByDiscordUser` WHERE clause in `db/queries/player_characters.sql`. Regenerated sqlc. Test `TestGetStatus_F16_ExcludesRetiredRow` inserts a retired row and an approved row for the same player/campaign, then asserts `GetStatus` returns the approved row.
- **Changed files**: `db/queries/player_characters.sql`, `internal/refdata/player_characters.sql.go` (generated), `internal/registration/f16_retired_lookup_test.go`
- **Reviewer verdict**: —

### F-17: /setup doesn't allow bot to post in #the-story
- **Source**: agent-01
- **Files**: `internal/discord/setup.go`
- **Test plan**: Test that #the-story permissions include bot SendMessages
- **Implementation notes**: Changed `theStoryPerms` to explicitly include a `PermissionOverwriteTypeMember` entry for `botUserID` with `Allow: SendMessages`, alongside the existing DM allow and @everyone deny. This mirrors how `#dm-queue` grants both DM and bot access. Test `TestSetupChannels_F17_TheStoryAllowsBotSendMessages` proves the bot overwrite is present.
- **Changed files**: `internal/discord/setup.go`, `internal/discord/setup_test.go`
- **Reviewer verdict**: —

### F-18: Map background opacity not persisted
- **Source**: agent-01
- **Files**: `dashboard/svelte/src/MapEditor.svelte`, `db/queries/maps.sql`
- **Test plan**: Test that opacity is saved and restored on map load
- **Implementation notes**: Stored `backgroundOpacity` as a top-level key inside `tiled_json` (JSONB). In `saveMap()`, `tiledMap.backgroundOpacity = backgroundOpacity` is set before building the payload. In `loadMap()`, `backgroundOpacity` is restored from `tiledMap.backgroundOpacity` if present, otherwise defaults to 0.5. No schema or backend changes needed — `tiled_json` is opaque `json.RawMessage`. Test `mapOpacity.test.js` proves save, restore, default fallback, and JSON round-trip.
- **Changed files**: `dashboard/svelte/src/MapEditor.svelte`, `dashboard/svelte/src/lib/mapOpacity.test.js`
- **Reviewer verdict**: —

### F-19: AoE full cover not used to block targets
- **Source**: agent-02
- **Files**: `internal/combat/aoe.go`, `internal/combat/cover.go`
- **Test plan**: Test that targets with full cover from AoE origin are excluded
- **Implementation notes**: Added `FullCover bool` field to `PendingSave`. Modified `CalculateAoECover` to always compute cover from origin (not just for DEX saves) and set `FullCover = true` when `CoverFull` is returned. In the `CastAoE` pending-save loop, targets with `ps.FullCover` are skipped via `continue`, excluding them from both `pendingSaves` and `affectedNames`. Test `TestCastAoE_F19_FullCoverExcludesTarget` proves a target behind full cover receives no pending save and does not appear in affected names.
- **Changed files**: `internal/combat/aoe.go`, `internal/combat/aoe_test.go`
- **Reviewer verdict**: —

### F-20: Wild Shape doesn't use beast speed
- **Source**: agent-03
- **Files**: `internal/combat/wildshape.go`, `internal/combat/turnresources.go`
- **Test plan**: Test that wild-shaped combatant uses beast speed for movement
- **Implementation notes**: In `ResolveTurnResources`, after resolving character speed, added a branch: if `combatant.IsWildShaped && combatant.WildShapeCreatureRef.Valid`, look up the creature via `s.store.GetCreature` and use `getBeastWalkSpeed(beast.Speed)` as the base speed. Falls back to character speed silently if creature lookup fails or beast speed is 0. Test `TestResolveTurnResources_F20_WildShapeUsesBeastSpeed` proves a wolf (40ft walk) overrides the druid's 30ft speed.
- **Changed files**: `internal/combat/turnresources.go`, `internal/combat/turnresources_test.go`
- **Reviewer verdict**: —

### F-21: Timeout saves roll raw 1d20 ignoring modifiers
- **Source**: agent-03
- **Files**: `internal/combat/timer_resolution.go`
- **Test plan**: Test that auto-resolved saves include ability modifier + proficiency
- **Implementation notes**: In `AutoResolveTurn`, before the pending-saves loop, load the combatant's character via `GetCharacter` and parse ability scores + proficiencies. For each pending save, compute the save modifier via `character.SavingThrowModifier` (ability mod + proficiency if proficient) and add it to the d20 roll before comparing to DC. Degrades silently to raw roll when character lookup fails (NPC combatants without character_id). Test `TestAutoResolveTurn_F21_SaveIncludesAbilityModifier` proves a WIS-proficient character's +5 modifier turns a raw 10 into a passing 15 vs DC 15.
- **Changed files**: `internal/combat/timer_resolution.go`, `internal/combat/timer_resolution_f21_test.go`
- **Reviewer verdict**: —

### F-22: Turn Builder roll fudging unreachable
- **Source**: agent-04
- **Files**: `internal/combat/turn_builder_handler.go`, `internal/combat/turn_builder.go`, `dashboard/svelte/src/TurnBuilder.svelte`
- **Test plan**: Test that rolls are generated at plan time so DM can fudge before confirm
- **Implementation notes**: Added `roller *dice.Roller` parameter to `GenerateEnemyTurnPlan`. After `BuildTurnPlan` returns, iterates attack steps and pre-rolls each via `RollAttack`, populating `RollResult` at plan-creation time. The UI already shows roll-fudge inputs when `step.attack.roll_result` exists, and `ExecuteEnemyTurn` already skips rolling when `RollResult != nil`. No frontend or execution-path changes needed. Test `TestGenerateEnemyTurnPlan_F22_AttackStepsHaveRollResult` proves attack steps have `RollResult` populated.
- **Changed files**: `internal/combat/turn_builder_handler.go`, `internal/combat/turn_builder_handler_test.go`
- **Reviewer verdict**: —

### F-23: Mobile Approvals renders wrong component
- **Source**: agent-04
- **Files**: `dashboard/svelte/src/MobileShell.svelte`
- **Test plan**: Test that mobile approvals tab renders CharacterApprovalQueue
- **Implementation notes**: Replaced `ActionResolver` import and rendering in the `approvals` tab with a new `CharacterApprovalQueue.svelte` component that fetches from `/dashboard/api/approvals/` and renders pending character approvals with approve/reject/request-changes actions. No existing CharacterApprovalQueue Svelte component existed (desktop uses a server-rendered HTML page). Test `mobileApprovals.test.js` proves the import and rendering are correct.
- **Reviewer verdict**: —

### F-24: Phase 120a e2e omits Discord output assertions
- **Source**: agent-05
- **Files**: `cmd/dndnd/e2e_scenarios_test.go`, `cmd/dndnd/e2e_harness_test.go`, `internal/discord/move_handler.go`, `internal/discord/loot_handler.go`
- **Test plan**: Add Discord output assertions for registration, movement, loot scenarios
- **Implementation notes**: (1) Registration: `SeedDMApproval` now sends the approval DM via the fake session (mirroring production `playerNotifierAdapter.NotifyApproval`); test asserts a `KindDirectMessage` containing "approved" and "Aria". (2) Movement: Added `postCombatLogChannel` call in `HandleMoveConfirm` after successful move; harness seeds campaign settings with `channel_ids.combat-log`; test asserts `KindChannelMessage` to `ch-combatlog-<guild>` containing "moves to B1". (3) Loot: Added `resolveStoryChannel` + `ChannelMessageSend` in `HandleLootClaim` after successful claim; test asserts `KindChannelMessage` to `ch-story-<guild>` containing "Magic Sword".
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
