# Group I: Dashboard Manual + Combat Manager (Phases 93a-102) — Review Findings

Review scope: docs/dnd-async-discord-spec.md §Manual Character Creation (2402-2416), §DM Notification System (2745-2772), §DM Dashboard (2773-2926); phases.md lines 558-627.

---

## [Critical] DM-created characters never inherit class or racial features
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/feature_provider.go:38, 49, 63-65 ; /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:583, 552
- **Spec/Phase ref:** Phase 93b — "features auto-populated from SRD class data by level"; spec §Manual Character Creation step 7.
- **Problem:** `RefDataFeatureProvider` indexes class features by `cls.ID` (slug, e.g. `"wizard"`) and races by `r.ID`. But the dashboard wizard form sets `<option value="r.name">` / `<option value="c.name">` (`charcreate_handler.go:552, 583`), so the DM submission carries display names (e.g. `"Wizard"`, `"Mountain Dwarf"`). `CollectFeatures` then does `classFeatures[c.Class]` with the *name* and gets nothing → DM-created characters are saved with empty `features` arrays and the wizard's "Features" step shows the empty-state message.
- **Suggested fix:** Either store the class/race id (slug) as the option's `value` (and submit that), or have the feature provider expose features keyed by both id and lowercased name (or do a normalized lookup in `CollectFeatures` / `RacialTraits`).

## [Critical] Pending #dm-queue badge count is campaign-wide, not per-encounter
- **Location:** /home/ab/projects/DnDnD/internal/combat/workspace_handler.go:173-180, 271 ; query /home/ab/projects/DnDnD/db/queries/dm_queue_items.sql:56-63
- **Spec/Phase ref:** Spec §DM Dashboard "Each tab shows a badge with the count of pending #dm-queue items for that encounter" (2792); Phase 94a "Multi-encounter tabs with badges for pending #dm-queue items".
- **Problem:** `GetWorkspace` calls `CountPendingDMQueueItemsByCampaign` once and assigns the same number to every encounter response (`PendingQueueCount: pendingQueueCount`). With two active encounters and one pending queue item, both tabs display `"1 queued"`, defeating the cross-encounter prioritization the spec describes. The SQL query has no `encounter_id` filter and the `dm_queue_items` table doesn't appear to track encounter linkage either.
- **Suggested fix:** Add `encounter_id` to `dm_queue_items` and write a per-encounter aggregation (`CountPendingDMQueueItemsByEncounter`), then compute counts in a single batched query keyed by `encounter_id`.

## [Critical] Narration-template Get/Update/Delete/Duplicate/Apply leak across campaigns
- **Location:** /home/ab/projects/DnDnD/internal/narration/template_handler.go:110-193 ; /home/ab/projects/DnDnD/internal/narration/template_service.go:89-143
- **Spec/Phase ref:** Phase 100b "Templates are campaign-scoped".
- **Problem:** Only `Create` and `List` accept `campaign_id`; `Get`, `Update`, `Delete`, `Duplicate`, and `Apply` look the template up purely by UUID and never check that the row's `campaign_id` matches the requesting DM. Any authenticated DM can read, edit, duplicate, delete, or apply another campaign's templates by guessing/leaking the UUID. The DM-only middleware (`RequireDM`) just confirms the caller is *a* DM, not the DM of that campaign.
- **Suggested fix:** Require `campaign_id` on every template endpoint and have the service verify `tpl.CampaignID == in.CampaignID` (returning `ErrTemplateNotFound` on mismatch). Alternatively switch to `RequireCampaignDM` and resolve the template's campaign before authorization.

## [High] Dashboard DM-created chars miss background skill proficiencies
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:221-253, 117 ; compare portal asset bundle `appendBackgroundSkillProficiencies` (`Ts`)
- **Spec/Phase ref:** Spec §Manual Character Creation step 4 — "skill proficiencies auto-calculated from race + classes + ability scores + total level using SRD rules"; phases §93a — "same steps as the player portal builder".
- **Problem:** `classSkillProficiencies` returns only the primary class's default skills. The portal applies an additional `appendBackgroundSkillProficiencies(skills, background)` pass that grants the SRD-defined skills (acolyte → insight+religion, etc.). The DM submission carries `Background` but it's persisted only as a label — the derived `Skills` map and the stored skill proficiencies never include background grants, so a DM-built sage Wizard has no Arcana/History proficiency.
- **Suggested fix:** Apply the background-skill table (same lookup the portal uses) before computing `SkillModifier` in `DeriveDMStats`, and ensure those skills land in the persisted character's skill_proficiencies array.

## [High] DM character form doesn't pass campaign_id to spell/equipment refdata
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:30-34, 281-318, 236-258 ; cf. /home/ab/projects/DnDnD/internal/portal/api_handler.go:74-75
- **Spec/Phase ref:** Phase 93b — "spell selection (for casters, filtered by class/level)"; spec §Homebrew "Used alongside SRD data in all contexts".
- **Problem:** `RefDataForCreate.ListSpellsByClass(ctx, class)` has no campaign param, and `HandleListRefSpells` never reads `campaign_id`. The portal's analogous interface (`portal.RefDataStore.ListSpellsByClass(ctx, class, campaignID)`) and `ListEquipment(ctx, campaignID)` take a campaign id specifically so homebrew spells/equipment (and Open5e-gated sources) show up. DM character creation will never see campaign-scoped homebrew spells/items — so a wizard can't be given a custom spell that exists in the same campaign's library.
- **Suggested fix:** Add `campaignID` to the dashboard's ref-data interfaces, accept `campaign_id` on `/dashboard/api/characters/ref/spells` and `/equipment`, and thread it through to the same scoping logic the portal uses.

## [High] Encounter Builder doesn't place PC tokens at combat start
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/EncounterBuilder.svelte:368-389
- **Spec/Phase ref:** Spec §Encounter Builder "Place PC tokens — DM visually places each PC token on the map using the same drag-and-drop interface" (2853).
- **Problem:** The Start-Combat panel checkboxes select PCs but `character_positions: {}` is always submitted empty. The inline comment even acknowledges "PC selection/positions aren't managed by this builder yet — the backend treats missing character_ids as 'no PCs'". PCs therefore enter encounters with no map position (default cell), making the spec's spatial workflow non-functional.
- **Suggested fix:** Add the same drag-to-map UX used for creature tokens to the PC list, collecting `{character_id: {col, row}}` and submitting it in `character_positions`.

## [High] Action Resolver `move` effect bypasses turn lock, walls, and concentration hooks
- **Location:** /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:215-313, 400-421
- **Spec/Phase ref:** Spec §Undo & Corrections "All overrides go through per-turn lock"; spec §DM Dashboard "Movement validation against walls/obstacles" (Phase 94b).
- **Problem:** `ResolvePendingAction` is not wrapped in `withTurnLock`, and `applyMoveEffect` writes directly to `h.svc.store.UpdateCombatantPosition` rather than going through `h.svc.UpdateCombatantPosition`. The override path (`OverrideCombatantPosition`) explicitly routes through the service "so the silence-zone concentration-break hook fires" — the resolver path skips that hook, skips wall/range validation, and races with concurrent player input (turn lock not acquired).
- **Suggested fix:** Wrap `ResolvePendingAction` in `withTurnLock`, and replace the raw store call inside `applyMoveEffect` with `svc.UpdateCombatantPosition` (or another service method that runs the same validations as Combat Manager drag-and-drop).

## [High] Active reactions panel highlights every active reaction on enemy turns
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:65-68
- **Spec/Phase ref:** Spec §Active Reactions Panel — "When the DM is resolving an enemy turn, **matching** declarations are highlighted" (2815).
- **Problem:** `shouldHighlight(reaction)` returns true whenever the active combatant is an NPC and the reaction is in `'active'` status — without consulting the reaction's description, trigger condition, target, or readied-action match. A reaction "Counterspell if I see a spell" lights up on every enemy attack, eroding the highlight's signal value.
- **Suggested fix:** Server-side, surface a `matches_active_turn` boolean (or trigger metadata) derived from the reaction's trigger and the current turn's action context, then highlight only when that flag is true.

## [High] Cross-tenant reads on character overview / narration history / message history
- **Location:** /home/ab/projects/DnDnD/internal/characteroverview/handler.go:35-47 ; /home/ab/projects/DnDnD/internal/narration/handler.go:95-118 ; /home/ab/projects/DnDnD/internal/messageplayer/handler.go:74-108
- **Spec/Phase ref:** Spec §65 "System verifies the authenticated Discord user ID matches the campaign's designated DM."
- **Problem:** The DM-only middleware (`dashboard.RequireDM`) only verifies the caller is *a* DM. These three handlers accept `campaign_id` as a query arg and return party sheets / narration log / DM-player message log without checking that the caller is the DM of that specific campaign. Any DM with a valid session can hit `?campaign_id=<other-campaign-uuid>` and read another group's roster, story posts, and private whispers.
- **Suggested fix:** Wrap each route under `dashboard.RequireCampaignDM` (and require `campaign_id` in the URL/query for that to work), or have each service verify campaign ownership against the authenticated user before returning rows.

## [High] Narration & message-player handlers trust author_user_id from request body
- **Location:** /home/ab/projects/DnDnD/internal/narration/handler.go:49-91 ; /home/ab/projects/DnDnD/internal/messageplayer/handler.go:32-71
- **Spec/Phase ref:** Phase 100a / Phase 101 — author attribution.
- **Problem:** The request payload for `POST /api/narration/post` and `POST /api/message-player/` includes `author_user_id` and the service blindly stores it. A DM could post-or-DM in another DM's name (visible in the dashboard history), and a malicious frontend could falsify audit entries. The authenticated user id is already in the request context (`auth.DiscordUserIDFromContext`).
- **Suggested fix:** Drop `author_user_id` from the request body and populate it from the request context inside the handler.

## [High] Movement-validation rules differ between drag-and-drop UI and DM Override
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:691-717 ; /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go:458-480
- **Spec/Phase ref:** Phase 94b — "Movement validation against walls/obstacles."
- **Problem:** Drag-to-move in `CombatManager.svelte` runs `findPath` and cancels the API call if `!pathResult.found`, so walls block player+DM token moves on the canvas. But the override endpoint (`OverrideCombatantPosition`) has no wall/altitude/distance validation server-side — anything goes. A misclick on the override form can teleport a token through walls, and there's no server enforcement preventing client bugs from doing the same on the canvas path.
- **Suggested fix:** Move pathfinding/wall validation into the service layer (so both client paths and DM override agree); the override flow should still allow bypass for explicit cases but log the bypass reason.

## [High] Manual character creation skips ability-score method validation
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:72-77
- **Spec/Phase ref:** Phase 93a — "manual input or point-buy".
- **Problem:** When `s.AbilityMethod == ""` (no method supplied) the validator never calls `portal.ValidateAbilityScores`. The wizard's submit collects `ability_method: abilityMethod` so this is normally non-empty, but the server-side validator never enforces a non-empty method nor performs cross-method checks (e.g. point-buy totals, standard-array equality). A craft request bypasses point-buy rules entirely.
- **Suggested fix:** Default `AbilityMethod` to the campaign's selected method at validation time (already happens in service for `validateAllowedAbilityMethod`) and always run `ValidateAbilityScores`.

## [High] Race speed table is hard-coded; ignores DB and homebrew races
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:204-217
- **Spec/Phase ref:** Phase 93a — derived stats from race; Phase 99 — homebrew races.
- **Problem:** `raceSpeed` is a switch over a tiny hard-coded list (dwarf/halfling/gnome/wood elf) keyed by name. Any other race — including the campaign's homebrew races created via `/api/homebrew/races` — falls through to `30`. The portal uses `RaceInfo.SpeedFt` from the database; the dashboard ignores that. Centaurs/dragonborn/homebrew speeds are wrong.
- **Suggested fix:** Look the race row up via `refData.ListRaces` (already accessible to the handler) and read its `speed_ft`; only fall back to 30 if missing.

## [High] DM character creation handler is not protected by DM auth
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:83-103, 112-138 ; cmd/dndnd/main.go:1123
- **Spec/Phase ref:** Spec §DM Dashboard — DM-only surface; phases 93a/93b.
- **Problem:** `requireAuthHelper` only asserts the request has a Discord user ID in context (any authenticated user). The wiring `charCreateHandler.RegisterCharCreateRoutes(router.With(dmAuthMw))` does add `dmAuthMw`, but that is `RequireDM` only — not the per-campaign DM check. Combined with the fact that `campaign_id` is a free-form body field on `POST /dashboard/api/characters`, any DM can create a character inside another DM's campaign (pre-approved status, ready for `/register`).
- **Suggested fix:** Apply `RequireCampaignDM` (or have the service validate `dm_user_id == request.User`) before mutating; ideally drop trust in client-supplied `campaign_id` and bind it to the URL.

## [Medium] Action Log viewer doesn't flag dm_override_undo entries
- **Location:** /home/ab/projects/DnDnD/internal/combat/action_log_viewer.go:144
- **Spec/Phase ref:** Spec §Action Log — "Override entries are visually distinguished (e.g., highlighted or badged) for quick scanning."
- **Problem:** `IsOverride = row.ActionType == "dm_override"` — but undo writes a row with `ActionType = "dm_override_undo"` (see `dm_dashboard_undo.go:158`). Undos are not visually highlighted in the viewer even though they're DM-driven corrections.
- **Suggested fix:** Treat both `dm_override` and `dm_override_undo` as override entries (or use a `HasPrefix("dm_override")` test).

## [Medium] Undo-of-undo re-applies the same undo instead of redoing
- **Location:** /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go:103-113
- **Spec/Phase ref:** Spec §Undo & Corrections — "Repeatable to walk back multiple steps".
- **Problem:** `mostRecentUndoable` skips entries with `ActionType == "dm_override_undo"`, so after an undo, repeating "Undo Last Action" picks the *same* original entry again. With no idempotency guard, this issues a second undo (writing `currentSnapshot` as the new `before_state`) — effectively a no-op (HP/conditions already match the previous undo's target) but it pollutes the action log and posts a duplicate `#combat-log` correction.
- **Suggested fix:** Either (a) mark previously-undone rows so they are skipped, or (b) treat consecutive undos as walking further back (so the second undo targets the action *before* the one just reverted).

## [Medium] Pending-action resolve effect: no audit row per effect, after-state misses post-damage hooks
- **Location:** /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:255-303, 423-441
- **Spec/Phase ref:** Phase 95 — "Items marked resolved with outcome summary"; spec §Action Log — "before/after diff of the affected fields".
- **Problem:** `captureResolverState` snaps only HP/temp/position/conditions on the action's `combatant_id` (the *acting* combatant). When DM effects target other creatures (damage to a goblin, condition on the target), those targets' before/after never make it into the audit log row. The resolver writes a single `resolve_pending_action` row attributed to the actor, so the action-log diff view loses the actual mutations.
- **Suggested fix:** Emit one audit row per effect (target-scoped), or expand the resolver state snapshot to include all affected combatants.

## [Medium] Character Overview lacks live HP/condition snapshots
- **Location:** /home/ab/projects/DnDnD/internal/characteroverview/service.go:24-39 ; dashboard/svelte/src/CharacterOverview.svelte:75-89
- **Spec/Phase ref:** Phase 101 — "Read-only view of all player character sheets"; spec line 2828.
- **Problem:** `CharacterSheet` ships HP max/current/AC but no `conditions`, `temp_hp`, `exhaustion_level`, or `is_alive`. The DM glance-overview can't tell whether a character is poisoned or unconscious without opening combat manager — and outside an active encounter it can't tell at all.
- **Suggested fix:** Add `conditions`, `temp_hp`, `exhaustion_level`, `concentration_active` to the projection (joined from the latest combatant row in any active encounter, falling back to character defaults).

## [Medium] DM action-resolver "move" effect doesn't write to action log even via effects
- **Location:** /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:400-421
- **Spec/Phase ref:** Spec §Action Log — "Sort chronologically", filter by action type `move`.
- **Problem:** A resolver-driven move uses `UpdateCombatantPosition` directly without creating a `move` action_log entry; movement only shows up under the umbrella `resolve_pending_action` row (with limited before/after fields). Filtering the Action Log by `move` won't surface DM-applied moves.
- **Suggested fix:** Insert a `move` action_log entry for each resolved move effect (same shape that direct player moves write).

## [Medium] DM character creation does not store skill proficiencies
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate_service.go:93-116
- **Spec/Phase ref:** Phase 93a — "skill proficiencies auto-calculated".
- **Problem:** `CreateCharacterParams` includes `Saves` (save proficiencies) and `Equipment`, but nothing maps `stats.Skills` (or the underlying skill_proficiencies list) into the persisted character. Compare `portal.CreateCharacterParams` (which accepts a `Skills` list from the wizard). DM-built characters end up with no skill proficiencies in the DB and the live `/check` command can't apply their proficiency bonus.
- **Suggested fix:** Add a `Skills []string` (or `SkillProficiencies []string`) field to `DMCharacterSubmission` and `CreateCharacterParams`, derived from class+background, and persist it.

## [Medium] DM character submission allows duplicate class entries
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:28-79
- **Spec/Phase ref:** Phase 93a — "Multiclass characters add multiple entries (e.g., Fighter 5 / Rogue 3)".
- **Problem:** `ValidateDMSubmission` doesn't detect duplicate class entries (e.g. `[{Fighter:3},{Fighter:2}]`). The wizard does filter the class dropdown to exclude already-chosen classes (in portal), but the dashboard's char-create page doesn't enforce that and the server validator doesn't reject it either. Multiclass HP/spellcasting derivations will treat them as separate progressions, producing nonsense.
- **Suggested fix:** Reject submissions with a duplicate class name in `ValidateDMSubmission`.

## [Medium] Spell selection isn't filtered by class or capped at max spell level on submit
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:11-25 ; charcreate_handler.go:281-330
- **Spec/Phase ref:** Phase 93b — "select known/prepared spells from class spell list (filtered by level)".
- **Problem:** `HandleListRefSpells` filters the list shown in the UI, but the submitted `Spells` field is just a flat `[]string` of spell IDs with zero server-side validation that the IDs are on the character's combined class spell list or at-or-below the max known level. A DM could submit Wish on a level-1 cleric.
- **Suggested fix:** Validate every submitted spell ID against the union of `ListSpellsByClass(class, max_level)` for each character class entry.

## [Medium] Combat workspace omits character_id when emitting combatant for spell-slot override
- **Location:** /home/ab/projects/DnDnD/internal/combat/workspace_handler.go:87-106
- **Spec/Phase ref:** Phase 97b — "edit HP, position, conditions, spell slots, initiative order directly".
- **Problem:** `workspaceCombatantResponse` does not include the underlying `character_id` field. The Svelte override panel guards the spell-slots section with `{#if selectedCombatant.character_id}`, but since the API never returns that key, the DM cannot reach the spell-slots override at all for any player combatant.
- **Suggested fix:** Add `CharacterID *string \`json:"character_id,omitempty"\`` to the response (populated from `c.CharacterID.UUID` when valid).

## [Medium] Reaction panel resolve/cancel calls aren't atomic with turn-lock
- **Location:** /home/ab/projects/DnDnD/internal/combat/handler.go:461-498 (ResolveReaction/CancelReaction)
- **Spec/Phase ref:** Spec §Undo & Corrections — "Overrides go through the per-turn lock" — and Phase 96 reactions panel.
- **Problem:** `Service.ResolveReaction` / `CancelReaction` write to `reaction_declarations` without acquiring the per-turn advisory lock. While not a "DM override" per se, the dashboard reaction-resolve mutates encounter state concurrently with player turn writes; if a player's `/reaction cancel` or `/move` interleaves, the reaction status can flip-flop.
- **Suggested fix:** Wrap reaction resolve/cancel in the same turn-lock used by the override handlers (or document why it's safe to skip).

## [Medium] Stat block library Get returns SRD entries even when ?source=homebrew
- **Location:** /home/ab/projects/DnDnD/internal/statblocklibrary/service.go:101-119, 147-166
- **Spec/Phase ref:** Phase 98.
- **Problem:** `visibleForSource` filters list results by source, but `GetStatBlock`/`GetStatBlockWithSources` doesn't consult the source filter — only the homebrew/Open5e visibility checks. A request to "get this homebrew row" can fall back to returning an SRD row with the same id (low risk, but the symmetry with `List` is broken).
- **Suggested fix:** Accept an optional source filter in `Get*` and reject non-matching rows.

## [Medium] Homebrew create/update has no structural validation beyond name
- **Location:** /home/ab/projects/DnDnD/internal/homebrew/service.go:99-145
- **Spec/Phase ref:** Phase 99 — "monsters (full stat block editor), spells, weapons, items, races, class features" + spec §Import validation "structural validation is mandatory: required fields present, types correct, values within sane bounds".
- **Problem:** `requireCreate`/`requireUpdate` only validate that name and campaign_id are present. Creating a homebrew creature with `hp_average = -1`, `ac = 0`, or `level = 99` succeeds silently. Spec applies bounds for imports; homebrew likely deserves the same sanity checks since these rows feed encounter builder math.
- **Suggested fix:** Add per-type structural validation (HP > 0, AC ≥ 0, CR matches `1/8|1/4|1/2|integer`, spell level 0–9, etc.) mirroring the importer's bounds.

## [Medium] DM Override "spell slots" endpoint does not log per-character before-state for audit diff
- **Location:** /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go:594-659
- **Spec/Phase ref:** Spec §Action Log — "before/after diff of the affected fields".
- **Problem:** `OverrideCharacterSpellSlots` attributes the audit row to `turn.CombatantID`, not the targeted character's owning combatant. The diff therefore shows the wrong actor; filtering by character won't surface their spell-slot overrides.
- **Suggested fix:** Resolve the combatant tied to `character_id` for the active encounter (if any) and use that as `actor_id` so filters work.

## [Medium] Race lookup is case-sensitive in raceSpeed and feature provider
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:206-216 ; feature_provider.go:63-65
- **Spec/Phase ref:** Phase 93a.
- **Problem:** `raceSpeed` ToLowers correctly but only matches a closed list; `RacialTraits(race)` does a direct map lookup with no normalization. Different casing or extra whitespace between the DDB importer / portal / dashboard yields a silent zero-traits result.
- **Suggested fix:** Centralize race lookup (case-folded, trimmed) in a single helper and reuse it across stat derivation and feature loading.

## [Medium] HP/condition tracker doesn't validate damage doesn't go negative for healing path
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:832-847
- **Spec/Phase ref:** Phase 94a.
- **Problem:** `handleApplyHealing` reads `result.hp_current` and pairs it with the unchanged `selectedCombatant.temp_hp` and `result.is_alive`. If `selectedCombatant.temp_hp` has been advanced server-side since last poll (player damage absorbed temp HP), the DM's heal request will overwrite it back to its pre-damage value, restoring temp HP unintentionally.
- **Suggested fix:** Either omit temp_hp in PATCH (server-side compute) or refetch the latest combatant before sending the heal.

## [Medium] Encounter Builder does not respect spec's "Auto-generated short ID" uniqueness
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/EncounterBuilder.svelte (lines around `short_id`)
- **Spec/Phase ref:** Spec §Encounter Builder — "Each creature gets an auto-generated short ID (G1, G2, OS, etc.)".
- **Problem:** Short IDs are generated client-side on creature add; once a creature is deleted and another added, ID re-use depends on the local naming logic. No server-side dedup against an encounter's existing combatants (especially for templates with quantity changes mid-edit).
- **Suggested fix:** Generate short IDs server-side at encounter-instance start (or validate uniqueness on save).

## [Medium] Manual char creation handler ignores starting-equipment "guaranteed:N" quantity
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go (loadStartingEquipment JS, lines 693-715)
- **Spec/Phase ref:** Phase 93b — "equipment selection (from class/background defaults + manual add)".
- **Problem:** `loadStartingEquipment` parses `"item:N"` strings as `id = item.split(':')[0]` and adds the ID once, dropping the quantity. A pack containing "arrows:20" populates as a single "arrows" entry. The wire payload `Equipment []string` similarly can't carry quantities.
- **Suggested fix:** Carry `(id, quantity)` pairs through the form/payload (`[]struct{ID string; Qty int}`) so packs faithfully populate.

## [Medium] Manual character creation can submit ability scores violating point-buy without method gating
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:67-77
- **Spec/Phase ref:** Phase 93a — "manual input or point-buy (DM's choice — system doesn't enforce a generation method)".
- **Problem:** The spec literally says the system "doesn't enforce a generation method" for the DM, but `validateAllowedAbilityMethod` then rejects submissions that don't match the campaign's allowed methods list. The two intents conflict — either the DM is allowed manual input regardless, or the campaign gating applies. Today the rejection path makes the DM's flow more restrictive than the spec describes.
- **Suggested fix:** Clarify intent in the spec; if "DM's choice" wins, skip `validateAllowedAbilityMethod` for the DM flow.

## [Medium] Reactions panel resolve flow doesn't post correction to #combat-log
- **Location:** /home/ab/projects/DnDnD/internal/combat/handler.go:461-498
- **Spec/Phase ref:** Spec §Undo & Corrections — "every undo or override posts a correction to #combat-log"; Phase 96.
- **Problem:** DM-initiated resolve/cancel of a reaction declaration mutates encounter state without any `#combat-log` write. The spec is ambiguous (correction language targets undo/override specifically), but the dashboard's resolve flow alters whether a reaction was consumed/cancelled — informationally relevant to players.
- **Suggested fix:** Document the intended behavior and (if appropriate) emit a low-priority `#combat-log` note for DM-resolved reactions.

## [Medium] Combat Manager polls every 5s in addition to WebSocket
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:101-109
- **Spec/Phase ref:** Phase 103 — push-only WebSocket.
- **Problem:** Phase 103 wires a per-encounter WebSocket explicitly so the dashboard doesn't poll. CombatManager still sets a `setInterval(loadWorkspace, 5000)` *in addition* to the WS subscription — every 5 seconds it overwrites the merged WS state with an HTTP refetch, defeating the dirty-field preservation logic that `mergeSnapshot` implements.
- **Suggested fix:** Make the 5s poll a fallback only when WS is disconnected (or remove it once WS coverage is verified).

## [Medium] HomebrewEditor list endpoint includes `?homebrew=true` but services don't filter by that query param
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/HomebrewEditor.svelte:60-72 ; statblocklibrary handler / others
- **Spec/Phase ref:** Phase 99.
- **Problem:** The Svelte editor appends `&homebrew=true` to list calls (creatures, spells, ...) but none of the underlying handlers (statblocklibrary, refdata listings) read that key — they ignore it and return SRD + homebrew. The editor then filters client-side. Fine in dev, but a paranoid filter (`e.homebrew !== false`) means the UI silently leaks foreign-campaign SRD as "homebrew" if a row's `homebrew` field is missing.
- **Suggested fix:** Either accept `homebrew=true` server-side (filter rows) or use the existing `source=homebrew` filter on stat-block library and equivalent for others.

## [Low] CharCreateHandler accepts campaign_id from request body, not URL
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:106-138
- **Spec/Phase ref:** §65 DM verification.
- **Problem:** `dmCreateRequest.CampaignID` comes from the JSON body; can't be matched to URL-bound `RequireCampaignDM`. Trust depends on `dmAuthMw = RequireDM` only.
- **Suggested fix:** Move campaign_id to the URL or apply per-request campaign ownership check inside the handler.

## [Low] DMCharacterSubmission missing subrace / SubraceID
- **Location:** /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:12-25
- **Spec/Phase ref:** Phase 93a — "name, race, background" (portal supports subrace).
- **Problem:** The portal builder collects subrace ID for races that have subraces (hill dwarf vs mountain dwarf etc.); the DM submission has no `Subrace` field, so those subraces and their stat bonuses cannot be applied to DM-created characters.
- **Suggested fix:** Add `Subrace string` to `DMCharacterSubmission`, render the subrace picker in the wizard when `Yt.subraces` is non-empty, and feed it into derived-stat / feature collection.

## [Low] Mobile view always renders TurnQueue as read-only — no quick-action to End Turn
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/MobileShell.svelte:47-49 ; QuickActionsPanel
- **Spec/Phase ref:** Spec §Responsive & Mobile — "Quick Actions — end turn, pause/resume campaign" (2805).
- **Problem:** `TurnQueue readOnly={true}` hides the End Turn button on mobile. The spec lists "end turn" as a mobile Quick Action; the QuickActionsPanel must surface that since the TurnQueue won't. Verify it does (we did not deep-dive QuickActionsPanel here, but the TurnQueue read-only-only choice forces the QuickActionsPanel to carry that feature).
- **Suggested fix:** Confirm QuickActionsPanel exposes End Turn; otherwise move End Turn into the mobile TurnQueue.

## [Low] Stat block library handler ignores ?homebrew param
- **Location:** /home/ab/projects/DnDnD/internal/statblocklibrary/handler.go:53-66
- **Spec/Phase ref:** Phase 98.
- **Problem:** `?homebrew=true` from `HomebrewEditor` is silently dropped; spec says homebrew should appear alongside SRD which it does, but the editor wants a homebrew-only listing.
- **Suggested fix:** Translate `homebrew=true` → `source=homebrew` server-side.

## [Low] DM display name change for active encounter persists without re-pinning to current turn
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:178-186 ; updateEncounterDisplayName
- **Spec/Phase ref:** Phase 105c.
- **Problem:** Renaming the player-facing encounter name during combat doesn't post a `#combat-log` notice. Spec doesn't strictly require it — but the action is visible to players (changes the `#initiative-tracker` footer or similar), so a silent rename can be jarring. Minor UX gap.
- **Suggested fix:** Optional: post a low-key `#combat-log` note on rename.

## [Low] Damage input doesn't allow damage types in HP & Condition Tracker
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:815-830
- **Spec/Phase ref:** Spec §Combat Manager.
- **Problem:** `handleApplyDamage` calls `applyDamage(selectedCombatant, damageInput)` with no damage type; resistance/immunity/vulnerability never apply. ApplyDamage server-side has `RawDamage` and no type, so untyped damage bypasses the R/I/V table.
- **Suggested fix:** Add a damage-type selector (radiant, fire, slashing, etc.) and forward it to `applyDamage` and the server.

## [Low] Action Resolver "move" target field uses col+row only — no altitude
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActionResolver.svelte:209-217
- **Spec/Phase ref:** Phase 95.
- **Problem:** The resolver's move builder lacks altitude_ft input; flying creatures resolved via the queue lose vertical state.
- **Suggested fix:** Add an altitude field beside row/col.

## [Low] ActionLogViewer formatValue stringifies arrays/objects as JSON — hard to read
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActionLogViewer.svelte:81-86
- **Spec/Phase ref:** Phase 97a — "Expandable entries showing full detail".
- **Problem:** Condition arrays render as `[{"condition":"poisoned"}]` raw JSON; less readable than even a comma-joined list.
- **Suggested fix:** Pretty-print arrays of strings/objects with a short helper.

## [Low] No "Resolve →" deep link inserted into #dm-queue messages
- **Location:** /home/ab/projects/DnDnD/internal/dmqueue/ — not audited line-by-line
- **Spec/Phase ref:** Spec §DM Notification System — "Each notification includes the player name, action context, and a 'Resolve →' link to the relevant dashboard panel" (2761).
- **Problem:** Outside the strict G-I scope but called out in the spec section we were asked to review. We did not see evidence of a Discord-side deep link to the dashboard panel.
- **Suggested fix:** Render a dashboard URL (campaign-scoped) inside `#dm-queue` notifications.

## [Low] Reactions panel doesn't fade once-per-round used reactions until creature's next turn
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:59-63
- **Spec/Phase ref:** Spec §Active Reactions Panel — "Consumed reactions are greyed out until the creature's next turn resets them".
- **Problem:** `effectiveStatus` collapses "used this round" into `dormant`, which the CSS class fades via `.dormant { opacity: 0.6; }`. That's marked as dormant but the panel also has a `.used { opacity: 0.45 }` rule — depending on which status flag is set in storage, the UX may differ. Cosmetic but worth verifying matches spec.
- **Suggested fix:** Unify dormant/used styling so consumed-this-round always renders identically.

## [Low] Encounter Builder list query doesn't return display_name in some responses
- **Location:** /home/ab/projects/DnDnD/internal/encounter/handler.go:58-82
- **Spec/Phase ref:** Spec §Encounter Builder.
- **Problem:** `display_name` is added when `et.DisplayName.Valid`, but the workspace handler's encounter response uses a separate `EncounterDisplayName(enc)` helper (combat package). Two code paths compute display name differently; minor risk of drift.
- **Suggested fix:** Share a single `DisplayName(enc)` helper across the encounter and combat packages.

## [Low] Narration history endpoint accepts unbounded offset
- **Location:** /home/ab/projects/DnDnD/internal/narration/handler.go:107
- **Spec/Phase ref:** Phase 100a.
- **Problem:** `parseIntDefault` allows arbitrarily large offset; service has no upper bound. Trivially DOSable.
- **Suggested fix:** Cap offset to a reasonable max (e.g., 10_000).

## [Low] Message-player history endpoint similarly unbounded
- **Location:** /home/ab/projects/DnDnD/internal/messageplayer/handler.go:96-101
- **Spec/Phase ref:** Phase 101.
- **Problem:** Same as above for limit/offset.
- **Suggested fix:** Cap.

## [Low] CharacterOverview message panel embeds full MessagePlayerPanel — leaks history across cards
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/CharacterOverview.svelte:97-103
- **Spec/Phase ref:** Phase 101.
- **Problem:** Toggling between message panels keeps each character's history mounted briefly; minor leak/Q regression.
- **Suggested fix:** Reset state when switching characters.

---

## Per-phase summary

- Phase 93a: see findings (Critical features bug, High background-skill, High race speed, High ability-method gating).
- Phase 93b: see findings (High refdata campaign id, Medium duplicate-class, Medium spell-list validation, Medium starting-equipment quantities).
- Phase 94a: see findings (Critical pending-queue badge, Medium combat workspace missing character_id).
- Phase 94b: see findings (High movement-validation parity).
- Phase 95: see findings (High move-effect bypass, Medium effect-audit coverage, Medium move-effect log, Low altitude/damage type).
- Phase 96: see findings (High highlight matching, Medium reaction-resolve turn lock).
- Phase 97a: see findings (Medium dm_override_undo flag, Low ActionLog formatValue).
- Phase 97b: see findings (Medium undo-of-undo, Medium spell-slots audit attribution).
- Phase 98: see findings (Medium statblock Get source mismatch, Low homebrew query param).
- Phase 99: see findings (Medium structural validation, Medium homebrew listing source).
- Phase 100a: see findings (High author trust, Low offset bound).
- Phase 100b: see findings (Critical cross-campaign template access).
- Phase 101: see findings (High cross-tenant reads, Medium HP/condition snapshot in overview, Low limit/offset, Low panel state).
- Phase 102: see findings (Low mobile TurnQueue read-only + QuickActions coverage).
