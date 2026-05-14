# DnDnD Spec Review — Consolidated Findings

Date: 2026-05-12
Scope: all 121 phases of `docs/phases.md` cross-checked against `docs/dnd-async-discord-spec.md` (3,475 lines). 16 batches; per-batch detail in `batch-NN-*.md` files alongside this summary.

---

## 1. Critical (security / correctness — fix before next playtest)

1. **Dashboard DM-mutation routes mounted on bare router (systemic F-2 regression).** `cmd/dndnd/main.go:561` (map import — incl. Tiled), `:594` (statblock — reveals hidden enemy data), `:615` (homebrew), `:635` (campaign pause/resume), `:641` (narration), `:647` (narration templates), `:653` (character overview), `:665` (message-player DM), `:679` (combatHandler), `:703` (combat workspace — incl. HP/position/condition mutate, advance-turn, override/*, concentration/drop) all registered before `dmAuthMw` is constructed. Any authenticated Discord user can pause campaigns, mutate combat, read hidden HP, post as DM, DM other players. Batches 1, 13, 16. — *F-2 only re-gated a subset, regression confirmed.*
2. **Six `/dm-queue` posters omit `CampaignID`** → `PgStore.Insert` rejects after Discord message has already been sent, orphaning `#dm-queue` messages that 404 on Resolve and never appear in the dashboard. `reaction_handler.go:159`, `check_handler.go:499`, `rest_handler.go:70`, `retire_handler.go:116`, `undo_handler.go:75`, `use_handler.go:308`. Also: `Notifier.Post` ordering is send-then-insert; reverse it. Batch 15.
3. **`Bot.HandleGuildCreate` / `HandleGuildMemberAdd` never wired; no `Identify.Intents`.** `cmd/dndnd/main.go:1197-1211` only adds `InteractionCreate`. Welcome DMs (spec §183–200) and dynamic guild-join command registration (spec §179) silently inert. Batch 1.
4. **`/equip` bypasses `combat.Equip` entirely.** `internal/discord/equip_handler.go:91` → `inventory.Equip`, which only flips a JSONB `Equipped` flag. No AC recalculation, no 2H/shield validation, no in-combat armor block, no writes to `equipped_main_hand/off_hand/armor` columns that grapple/somatic/stealth/attack all read. The fully-tested `combat.Equip` service is dead. Batch 8.
5. **`/interact` bypasses `combat.Interact`.** `internal/discord/interact_handler.go:108` calls `combat.UseResource(turn, ResourceFreeInteract)` directly — second `/interact` rejected instead of falling back to the action; no `pending_actions` row; DM never sees it. Batch 8.
6. **Magic items never feed combat / save / turn pipelines.** All three callers of `BuildFeatureDefinitions` (`combat/attack.go:1584`, `combat/turnresources.go:262`, `discord/save_handler.go:281`) pass zero `extraDefs`. `magicitem.CollectItemFeatures` is correct and unit-tested but never invoked in production. +1 longsword adds nothing; Cloak of Protection adds nothing; Boots of Speed don't change movement. F-14 `modify_speed` parsing is correct but unreachable. Batch 11 / Phase 88a "done when" unmet.
7. **Character cards never auto-update outside combat (Phase 17 partial).** `OnCharacterUpdated` is fired only from `internal/combat/service.go`. None of `/equip`, `/use`, `/give`, `/loot`, `/attune`, `/rest`, `/prepare`, level-up, or DM inventory API trigger it — directly contradicts spec line 216. Compounded by `/equip` reading/writing different columns than the card (item 4). Batch 2.
8. **Fog of war is dead code in production.** `cmd/dndnd/discord_adapters.go:417-514` calls `ComputeVisibilityWithLights` with empty `VisionSources` / `LightSources` / `MagicalDarknessTiles`; `DMSeesAll` never set. Full shadowcasting algorithm + tests exist but the `if len(md.VisionSources) > 0` guard is always false. Maps render fully uncovered. Largest single user-facing gap. Batch 10.
9. **`feature_uses` JSON shape is forked.** `internal/combat/feature_integration.go:25` reads `map[string]int`; `internal/character/types.go:54` + `internal/rest/rest.go:170-178` read `{Current,Max,Recharge}`. Mutually unparseable — combat-written rage/ki rows never recharge on rest; dashboard-structured rows fail combat deductions. Batch 7.
10. **Sneak Attack `OncePerTurn` never enforced at runtime.** `populateAttackFES` (`internal/combat/attack.go:1556`) never sets `AttackInput.UsedThisTurn`; the `EffectConditions.OncePerTurn` filter only trips when the caller threads a non-nil map. Only tests do. Every qualifying attack in a turn re-adds Sneak Attack dice. Batch 7.
11. **Reckless first-attack gate broken.** `attack.go:870` checks `cmd.Turn.ActionUsed`, which `Service.Attack` never sets — only decrements `AttacksRemaining`. Reckless can be re-declared on every Extra Attack swing. No test covers the negative path. Batch 5.
12. **`UNIQUE(campaign_id, discord_user_id)` blocks re-register-after-retire.** Spec line 40 promises retired players can `/register` a new character; retire flow only sets status='retired' so the second INSERT fails. Either partial unique index or retire-frees-the-slot needed. Batch 1.
13. **AoE cylinder shape unsupported + AoE pipeline ignores Metamagic.** `internal/combat/aoe.go:226-238` has no `cylinder` case (silently breaks Moonbeam, Flame Strike, Ice Storm, Sleet Storm, Call Lightning, Reverse Gravity). `AoECastCommand` has no `Metamagic` field at all — Careful/Heightened/Twinned never reach AoE casts. Batch 9.
14. **Encounter zones persist but never render and never deal damage.** `MapData.ZoneOverlays` exists but no production caller populates it; `ParseTiledJSON(..., nil, nil)` everywhere. `CheckZoneTriggers` returns `Effect:"damage"` but no caller rolls/applies — Spirit Guardians, Wall of Fire, Moonbeam, Cloud of Daggers don't damage. `ZoneAffectedTilesFromShape` also has no `line` case → Wall of Fire becomes a single tile. Batch 9.
15. **DM-created multiclass dropped to level-1 single class.** `internal/dashboard/charcreate_service.go:71-79` maps only `primaryClass`/`primarySubclass` into `CreateCharacterParams`; `sub.Classes` never forwarded. `BuilderStoreAdapter.resolveClassEntries` falls back to a single L1 entry. Pre-approved status means it hits production without DM review. Batch 12.
16. **WS Origin verification disabled in prod.** `internal/dashboard/ws.go:119` `InsecureSkipVerify: true`. Same code path in dev + prod — CSRF-style cross-origin upgrade vector. Batches 2, 14.

---

## 2. High (broken or unimplemented spec promise — non-security)

- **`/cast spare-the-dying` is seeded as auto-resolve but never wired to `StabilizeTarget`.** Batch 6.
- **`/action help` advantage is silently broken.** `standard_actions.go:250-257` applies `help_advantage` condition, but no read site in `advantage.go`/`attack.go` consumes it. Help grants zero benefit. Batch 6.
- **Exhaustion is read-only.** Speed/HP/disadvantage/death effects wired, but nothing ever increments or decrements `exhaustion_level`. No `/exhaustion`, no dashboard mutation, no long-rest decrement, no forced-march hook. Batch 6.
- **Auto-skip Dodge never expires.** `timer_resolution.go:194-199` applies Dodge with `ExpiresOn: "start_of_next_turn"` and no `SourceCombatantID`; `isExpired` only recognizes `start_of_turn`/`end_of_turn` AND requires source. Condition leaks across rounds. Batch 6.
- **Rage damage resistance has no consumer.** `ProcessorResult.Resistances` populated but `internal/combat/damage.go` doesn't read it. Bludgeon/pierce/slash resistance likely doesn't apply. Batch 7.
- **Wild Shape doesn't swap physical ability scores.** HP/AC swap only; STR/DEX/CON of the beast form not applied. Batch 7.
- **Turned condition does not auto-clear on damage** (spec rule). Batch 7.
- **Paladin Aura of Protection not a registered class feature.** `BuildFeatureDefinitions` has no `aura_of_protection` mapping — real paladins get no aura via `/save`. Tests inject manually. Batch 11.
- **Metamagic interactive prompts are dead code.** `MetamagicPromptPoster.PromptEmpowered/Careful/Heightened` are built and unit-tested but never invoked by `cast_handler.go`. `CastCommand.TwinTargetID` is never read from Discord. Batch 9.
- **`dm_required` spells and high-level teleports never create dashboard queue rows.** Only set a string flag; no `dm_queue_items` insert; DM never sees them. Batch 9.
- **Narrative teleport poster framework-only — zero runtime callers.** `PostNarrativeTeleport` + `KindNarrativeTeleport` fully tested but `/cast` never invokes them. Batch 15.
- **OA: no DM dashboard prompt for DM-controlled hostiles; no end-of-round forfeiture sweep.** Move handler posts only to `#your-turn` (PC path); DM-controlled hostile OAs silently dropped. Batch 8.
- **Phase 21c lighting + elevation are write-only on the server.** Svelte editor persists 5 Tiled layers; `internal/gamemap/renderer/parse.go` reads only `terrain` + `walls`. Lighting/elevation tiles stored but never consumed → magical-darkness FoW demotion only fires for runtime zones, not for map-painted lighting. Batch 3 (compounded by Batch 10 obscurement gap).
- **Server's `generateDefaultTiledJSON` produces 2 layers vs editor's 5.** Maps created via raw `POST /api/maps` are missing lighting/elevation/spawn_zones. Becomes a bug once item above is fixed. Batch 3.
- **Explored cells in memory only.** `mapRegeneratorAdapter.exploredCells` has no DB persistence — bot restart wipes every party's exploration history; incompatible with async-first design. Batch 10.
- **`pending_queue_count` orphan field.** `CombatManager.svelte:878,900` reads `enc.pending_queue_count` for "queued" badges but `workspaceEncounterResponse` never returns it and the field exists nowhere in `internal/`. Phase 94a done-when "badges for pending #dm-queue items" silently broken. Batch 13.
- **Portal `ProficiencyBonus` hard-coded to L1** (`BuilderService.CreateCharacter:164`). Batch 12.
- **Ability-score generation methods missing across portal + dashboard.** Spec wants point-buy / standard array / roll gated by DM setting; portal only does point-buy; dashboard only manual entry. Batch 12.
- **`/fly` "loses fly speed" fall-damage trigger missing.** Only prone-while-airborne is wired. Spell expiration, polymorph, dispel → silent float. Batch 5.
- **`/dashboard/exploration/transition-to-combat` doesn't transition.** Returns merged positions JSON but never calls `UpdateEncounterMode` or starts an encounter. Endpoint name overstates the behavior. Batch 15.
- **`/bonus offhand` surface mismatch.** Spec uses `/bonus offhand`; implementation uses `/attack offhand:true` (`attack_handler.go:170`). Service-side logic correct. Batch 5.

---

## 3. Medium (divergent surface, partial impl, dead code)

- DDB re-sync pending entries in-memory only (Batch 12).
- DDB import advisories absent (e.g., "Wizard spell list includes Cure Wounds") and homebrew tagging incomplete (Batch 12).
- Feat sub-choice prompts missing (Resilient/Skilled/Elemental Adept) (Batch 12).
- "⏳ ASI/Feat pending" character-card indicator missing (Batch 12).
- Long rest does not reduce exhaustion -1 (Batch 6).
- `PassiveCheck` service exists but never called from production (Batch 11).
- `/prepare` paginated UX deferred (Batch 9).
- `requires_sight` for teleport spells unenforced (Batch 9).
- Material-component substitution string-match brittle (Batch 9).
- Counterspell retroactive slot cleanup manual (Batch 9).
- Drag-while-moving / Release-and-Move interactive prompt is decorative only (separate `/bonus release-drag`) (Batch 8).
- `/action grapple` alias missing (only via `/shove --mode grapple`) (Batch 8).
- Summons always share caster's turn — no independent-initiative mode (Batch 10).
- 100% auto-skip resolves don't decline Divine Smite / Bardic Inspiration via UI (Batch 10).
- Heal-from-0 removes prone (spec says still prone) (Batch 6).
- `DropProne` skips `Service.ApplyCondition` → airborne fall-damage hook doesn't fire (Batch 6).
- Temp HP duration tracking absent (Batch 6).
- Escape contested check is STR-only; spec allows STR or DEX (Batch 6).
- `/help` lacks "context-specific tips" (Batch 15).
- `/status` omits Channel Divinity uses + Smite slot summary (Batch 15).
- `errorlog` only fires on panic, not normal error returns (Batch 16); 24h dashboard badge will under-report.
- `Sacred Weapon` (Cleric Channel Divinity) attack-mod missing from mechanical_effects (Batch 7).
- Channel Divinity DM-routing for narrative subclass options has no DB row (Batch 7).
- Dead code: `RegisterCommands` delete-loop after bulk-overwrite (Batch 1).
- Bot permission validation defined but never called (Batch 1).
- Session cookie MaxAge doesn't slide with DB TTL (Batch 1).
- Map renderer has no golden PNG fixtures — only decode+pixel-dimension tests (Batch 3).
- Two-handed weapon damage swap on versatile via `/equip` UX unclear (Batch 8).
- Sneak Attack reaction-attack rider unverified (Batch 7).
- `/dashboard/levelup` legacy page + `/api/levelup` POST unauthenticated (Batch 14).

---

## 4. Low / observations

- Existing `AcquireAndRelease` callers (e.g., `move_handler.go:288`) still race; F-4 only migrated some handlers. Spec correctness preserved by re-validation, but worth auditing (Batch 4).
- F-17 UUID→int64 truncation documented + verified safe (Batch 4).
- `/distance` exemption from advisory-lock is justified read-only, but goes beyond spec (Batch 4).
- All 32 Discord handler setters in `router.go` map 1-to-1 with `attachPhase105Handlers` (Batch 14).
- Crash-recovery startup sequence in `main.go:458–996` matches spec verbatim; gateway kept dark during stale scan (Batch 14).
- F-9 publisher in `/attune` is correctly wired end-to-end (Batch 11).
- Dice engine fully covers Phase 18 spec (advantage/disadvantage cancellation, crit, breakdowns) — "keep-highest/exploding" was not in spec (Batch 2).
- Phase 121 is partially shipped under explicit `[x]` iteration markers despite top-level `[ ]`; only 121.4 deferred (Batch 16).
- Open5e per-campaign toggle (F-8) correctly wired (Batch 16).
- F-7 Tiled import button present (Batch 16).
- F-20 `WithContext` adopted in `/move`; not yet propagated everywhere (Batch 16).

---

## 5. Phases fully matching spec (no findings)

24, 25, 26a, 26b, 27, 28, 29, 30, 32, 33, 34, 35, 36, 37, 41, 46, 48a, 48b, 49, 51, 52, 53, 60, 64, 76a, 76b, 77, 78a, 78b, 78c, 80, 81, 83a, 83b, 85, 86, 87, 88b, 88c, 89, 89d, 91a, 91c, 92, 92b, 93b, 94b, 96, 97a, 97b, 98, 100b, 101, 102, 103, 104, 104b, 104c, 105, 105b, 105c, 106a, 106c, 106d, 106e, 106f, 107, 108, 109, 110, 110a, 111, 113, 114, 116, 117, 118, 118b, 118c, 119, 120, 120a.

---

## 6. By-batch index

- [batch-01-foundation.md](batch-01-foundation.md) — guild handlers + intents inert; pause/resume + register-after-retire need fixes.
- [batch-02-player-dashboard-dice.md](batch-02-player-dashboard-dice.md) — character cards don't auto-update outside combat; `/equip` writes wrong columns; WS Origin check disabled.
- [batch-03-maps-assets-encounter-builder.md](batch-03-maps-assets-encounter-builder.md) — Phase 21c lighting/elevation server never reads them; default map JSON drift; no golden renderer fixtures.
- [batch-04-encounters-combat-lifecycle.md](batch-04-encounters-combat-lifecycle.md) — all six phases match; F-17 documented safe; F-4 partially migrated.
- [batch-05-movement-attack.md](batch-05-movement-attack.md) — Reckless first-attack gate broken; `/fly` lose-speed trigger missing; `/bonus offhand` surface mismatch.
- [batch-06-conditions-damage-deathsaves.md](batch-06-conditions-damage-deathsaves.md) — `/action help` advantage unconsumed; exhaustion read-only; auto-Dodge leaks; `/cast spare-the-dying` no-op.
- [batch-07-features-classes.md](batch-07-features-classes.md) — `feature_uses` JSON shape forked; Sneak Attack `OncePerTurn` unenforced; rage resistance has no consumer; Wild Shape no physical-stat swap.
- [batch-08-tactical-equipment.md](batch-08-tactical-equipment.md) — `/equip` + `/interact` bypass their respective combat services; OA forfeiture sweep + DM hostile prompt missing.
- [batch-09-spells.md](batch-09-spells.md) — cylinder AoE unsupported; AoE has no Metamagic; encounter zones not rendered + no damage triggers; Metamagic prompts dead.
- [batch-10-fog-turnflow.md](batch-10-fog-turnflow.md) — FoW dead in prod; static lighting brush never reaches server; explored cells in-memory only.
- [batch-11-checks-rest-items.md](batch-11-checks-rest-items.md) — magic items never feed combat/save/turn; Aura of Protection not a feature def; PassiveCheck unused.
- [batch-12-leveling-creation-portal.md](batch-12-leveling-creation-portal.md) — DM multiclass dropped; portal proficiency hard-coded L1; ability-score methods missing.
- [batch-13-dashboard-combat.md](batch-13-dashboard-combat.md) — F-2 systemic auth gap (mutation routes on bare router); `pending_queue_count` orphaned; no FoW overlay.
- [batch-14-ws-recovery-multi.md](batch-14-ws-recovery-multi.md) — all six phases match; legacy `/dashboard/levelup` + `/api/levelup` unauth.
- [batch-15-dmqueue-help-exploration.md](batch-15-dmqueue-help-exploration.md) — six DM-queue posters drop CampaignID; narrative teleport never invoked; exploration→combat doesn't transition.
- [batch-16-late-phases.md](batch-16-late-phases.md) — pause/resume + map import unauth (F-2 still real); errorlog only on panic; Phase 121 mostly shipped under `[ ]`.

---

## 7. Recommended next-step bundles

**Bundle A — Auth gating (F-2 systemic regression).**
Move every `RegisterRoutes(router)` in `cmd/dndnd/main.go` below the `dmAuthMw` construction at `:763`. Wrap each affected mount in `router.Group(func(r){ r.Use(dmAuthMw); handler.RegisterRoutes(r) })`. Covers items 1, 16, and the Batch 14 legacy `/api/levelup` observation.

**Bundle B — DM-queue plumbing (data integrity).**
Add `CampaignID` to the six bad poster call sites (reaction, check, rest, retire, undo, use). Flip `Notifier.Post` ordering to insert-then-send (so a failed insert doesn't post a ghost Discord message). Wire `PostNarrativeTeleport` from `/cast`. Create real `dm_queue_items` rows for `dm_required` spells + high-level teleports. Covers item 2 and several Highs.

**Bundle C — Card auto-update + Discord handler rewires.**
Add `OnCharacterUpdated` (or equivalent publisher event) to all non-combat mutators (`/equip`, `/use`, `/give`, `/loot`, `/attune`, `/rest`, `/prepare`, level-up, DM inventory API). Route `/equip` through `combat.Equip` so AC + columns + hand validation actually happen; route `/interact` through `combat.Interact` so the action fallback + DM queue work. Render "⏳ ASI/Feat pending" badge on cards. Covers items 4, 5, 7.

**Bundle D — Feature-effect runtime wiring.**
Pass `magicitem.CollectItemFeatures` into `BuildFeatureDefinitions` callers (`combat/attack.go:1584`, `combat/turnresources.go:262`, `discord/save_handler.go:281`). Unify `feature_uses` JSON on `{Current,Max,Recharge}` and migrate combat write paths. Populate `AttackInput.UsedThisTurn` from the turn state so `OncePerTurn` and Reckless first-attack actually trip. Register `aura_of_protection` (Paladin) as a real FeatureDefinition. Confirm `damage.go` consumes `ProcessorResult.Resistances`. Covers items 6, 9, 10, 11 + Aura/Wild-Shape Highs.

**Bundle E — Encounter visuals + spell-zone effects + FoW activation.**
Populate `MapData.ZoneOverlays` from `ListZonesForEncounter` in `cast_handler.go`, `move_handler.go`, `attack_handler.go`. Add `cylinder` + `line` shape cases in `aoe.go` and `ZoneAffectedTilesFromShape`. Auto-apply zone damage from `CheckZoneTriggers` (roll dice, route through damage pipeline). Populate `VisionSources` / `LightSources` / `MagicalDarknessTiles` / `DMSeesAll` in `RegenerateMap`. Parse Tiled `lighting` + `elevation` layers in `renderer/parse.go`. Persist `exploredCells` to DB. Covers items 8, 13, 14 + most lighting/FoW Highs.

**Bundle F (smaller) — Onboarding + register-after-retire.**
Wire `Bot.HandleGuildCreate` / `HandleGuildMemberAdd` in `main.go:1197-1211`. Set `Identify.Intents = GuildMembers`. Make the `player_characters` unique constraint partial (`WHERE status != 'retired'`) or have retire free the slot. Covers items 3, 12.

---

End of summary. Detail per batch in `.spec-review/batch-NN-*.md`.
