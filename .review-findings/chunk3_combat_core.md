# Chunk 3 Review — Combat Core (Phases 23–38)

## Summary

Phases 23–38 are largely complete at the **service / domain layer**: every spec mechanic has corresponding Go code in `internal/combat`, `internal/pathfinding`, and `internal/encounter`, and unit-test coverage of pure functions (initiative tiebreaking, advantage detection, pathfinding, attack resolution, modifier flags, cover, distance, ammunition) is excellent. The most serious gap is at the **Discord-handler layer**: `/attack`, `/cast`, `/bonus`, `/shove`, and several other combat commands listed in `commands.go:42` are registered but routed to a "not yet implemented" stub in `router.go:228-230,495-497`, so end-users cannot exercise the Phase 34/36/37/38 attack pipeline through Discord today. Phase 27's advisory-lock + turn-ownership flow is implemented and integration-tested (`turnlock_integration_test.go`) but is **not actually wired into the player-facing `/move`, `/fly`, or `/distance` handlers** — see the explicit `TODO: turn ownership validation will be wired when full turn lock is available` at `move_handler.go:155`. Phase 26b is missing several spec-mandated cleanup steps (concentration end, ammunition recovery prompt, timer cancellation), and Phase 25 lacks an auto-post + auto-update of the initiative tracker message in Discord — the format function exists but only its string is returned to the dashboard. None of these are correctness bugs in the implemented code; they are coverage holes that show up the moment a real player session runs.

## Per-phase findings

### Phase 23: Encounter Templates & Encounter Builder ✅
- Migration: `db/migrations/20260312120001_create_encounter_templates.sql:2-14` (campaign_id, map_id, name, display_name, creatures JSONB).
- Service: `internal/encounter/service.go:45-137` — Create / Get / List / Update / Delete / Duplicate / ListCreatures.
- HTTP handlers: `internal/encounter/handler.go:27-37,289-310`.
- Dashboard: `dashboard/svelte/src/EncounterBuilder.svelte:47-48,313-348,483-484,557-559` implements drag-drop placement (`draggingCreature`, `dragPreviewPos`, `handleCanvasDrop`).
- Saved Encounters list: `dashboard/svelte/src/EncounterList.svelte`.
- Done-when satisfied.

### Phase 24: Encounters / Combatants / Turns / Action Log ✅
- Schema in single migration `db/migrations/20260312120002_create_encounters.sql:3-98` covering `encounters`, `combatants`, `turns`, `action_log` plus the deferred FK from `encounters.current_turn_id` -> `turns.id` at lines 80-81.
- sqlc query files all present: `internal/refdata/{encounters,combatants,turns,action_log}.sql.go` with 13/18/22/5 queries respectively.
- `CombatantFromCharacter` factory wired through `service.go` (StartCombat path).

### Phase 25: Initiative System ⚠️
- `SortByInitiative` (`internal/combat/initiative.go:163-173`) tiebreaks on roll DESC, DEX DESC, alphabetical — correct.
- `RollInitiative` (`initiative.go:267-334`) rolls d20+DEX, sorts, persists `initiative_order`, sets round=1 / status=active.
- Surprise: `IsSurprised`, `SurprisedCondition`, `MarkSurprised`, plus auto-skip in round 1 inside `AdvanceTurn` (`initiative.go:453-476,550-570`).
- Round counter advances when no candidates remain (`initiative.go:432-446,486-496`).
- Tracker formatter `FormatInitiativeTracker` (`initiative.go:206-220`) and `FormatCompletedInitiativeTracker` (`initiative.go:222-234`) produce the strings.
- ⚠️ **The tracker message is never posted to / updated in `#initiative-tracker`** — `grep -rn 'InitiativeTracker' internal/discord` shows zero auto-posts. The string is returned via `combat.handler.go:252,355` (HTTP) but nothing tails turn changes to update a persisted Discord message ID. Spec line 1696 / Phase 25 done-when require auto-update.
- Tests: `initiative_test.go` covers tiebreaks (lines 73-105), surprised auto-skip, all-surprised round advance (line 625), round advancement; ~40 cases in this file.

### Phase 26a: Combat Lifecycle — Start ✅ / ⚠️
- `StartCombat` (`internal/combat/service.go:573-627`) creates encounter from template, adds PCs, marks surprised by short ID, rolls initiative, advances to first turn.
- HTTP entry: `combat/handler.go:203-260` (`POST /api/combat/start`).
- ⚠️ No Discord-side trigger or first-combatant ping — `StartCombat` is dashboard-only. The first turn's prompt only fires from `done_handler.go:415-423` *after* the first `/done`, leaving the very first PC/NPC un-pinged on combat start. `resume_turn_pinger.go:94` sends prompts only on bot startup recovery.

### Phase 26b: Combat Lifecycle — End & Cleanup ⚠️
- `EndCombat` (`service.go:657-740`) sets `status=completed`, completes active turn, clears combat-only conditions, deletes encounter zones, cleans reaction declarations, returns summary + tracker text.
- `AllHostilesDefeated` poll: `service.go:743-759`, exposed at `combat/handler.go:34,360` (`GET …/hostiles-defeated`).
- ⚠️ Missing cleanup steps from spec:
  - **End concentration:** no call to a concentration-end function inside `EndCombat`. (Concentration logic lives in `internal/combat/concentration.go` but isn't invoked.)
  - **Ammunition recovery:** `RecoverAmmunition` (`attack.go:210-220`) is implemented and unit-tested (`attack_test.go:2239-2270`) but never invoked from `EndCombat`.
  - **Timer cancellation:** `PauseCombatTimers` (`timer_overrides.go:44`) is not called from `EndCombat`.
  - **#combat-log bot announcement:** no Discord post on encounter completion (only the dashboard publish at `service.go:730`).
- Casualty / round-elapsed summary is populated (`service.go:701-727`).

### Phase 27: Concurrency — Advisory Locks & Turn Validation ⚠️
- `AcquireTurnLock` (`internal/combat/turnlock.go:32-53`) — 5s `SET LOCAL lock_timeout`, `pg_advisory_xact_lock`, returns `ErrLockTimeout` on SQLSTATE 55P03.
- `AcquireTurnLockWithValidation` (`turnvalidation.go:129-158`) — TOCTOU re-check after lock.
- `ValidateTurnOwnership` (`turnvalidation.go:51-118`) — DM bypass, NPC=DM only, otherwise verifies `player_characters.discord_user_id == invoker`.
- Exemptions: `IsExemptCommand("reaction"|"check"|"save"|"rest")` (`turnvalidation.go:162-169`).
- Excellent integration tests in `turnlock_integration_test.go` (lines 118, 157, 193, 222, 242, 261, 277, 343, 426).
- ❌ **Not wired into Discord handlers.** `grep -n 'AcquireTurnLock\|ValidateTurnOwnership' internal/discord` returns nothing — no handler in `internal/discord/` calls either function. Explicit gap: `move_handler.go:155 // TODO: turn ownership validation will be wired when full turn lock is available`. `fly_handler.go` (no lock, no ownership check) and `distance_handler.go` likewise rely on the encounter's current turn without validating that the invoker owns it. The `action_handler.go:165-168` does a manual `combatantBelongsToUser` ownership check but no advisory lock.
- ❌ **`IsExemptCommand` only referenced from tests** (`turnlock_integration_test.go:409-419`); no real router branches on it.

### Phase 28: Turn Resource Tracking ✅
- Resource enum + Validate / Use / Refund: `turnresources.go:23-114`.
- Movement/attack helpers: `UseMovement` (`turnresources.go:119-131`), `UseAttack` (`turnresources.go:158-164`).
- `AttacksPerActionForLevel` (`turnresources.go:137-154`) + `resolveAttacksPerAction` multiclass-best (`turnresources.go:236-259`).
- `ResolveTurnResources` resolves speed and attacks, applies condition speed effects (`turnresources.go:214-232`).
- Display: `buildResourceList`, `FormatTurnStartPrompt`, `FormatRemainingResources` (`turnresources.go:186-207,261-313`) — spent resources omitted as required.
- Tests in `turnresources_test.go` (33+ cases).

### Phase 29: Pathfinding (A*) ✅
- `pathfinding.FindPath` (`internal/pathfinding/pathfinding.go:179-270`).
- Difficult terrain ×2 + prone ×2 → `tileCost` (lines 283-293) yields 5/10/15ft (normal / difficult / prone-on-difficult), matching spec lines 1397-1407.
- Walls via `buildBlockedEdges` (lines 95-135) cardinal-only blocking; diagonal corner-cut allowed at line 242.
- Diagonal at 5ft via Chebyshev heuristic (lines 164-171) and tile-cost = 5 (no alt-counting).
- Occupancy: `canPassThrough` (lines 276-281) → ally always passes, enemies only with `|size diff| >= 2`.
- Flying occupants don't block ground (lines 142-147) — spec line 1406.
- Tests: 35+ cases in `pathfinding_test.go` covering every requirement (lines 16-535).

### Phase 30: Movement (`/move`) ✅ / ⚠️
- `ValidateMove` (`movement.go:48-144`), prone variants `ValidateProneMoveStandAndMove` / `ValidateProneMoveCrawl` (`movement.go:149-188`).
- A1-AA99 parsing: `renderer.ParseCoordinate` invoked at `movement.go:50` and `move_handler.go:107`.
- Cannot end on occupied tile: `movement.go:84-93` plus `ValidateEndTurnPosition` (`movement.go:211-221`).
- Discord wiring: `move_handler.go:97-299` with confirm/cancel buttons (lines 270-298).
- Split movement implicit (each `/move` deducts only its cost; remaining shown in `FormatMoveConfirmation`).
- Ally pass-through tested: `movement_test.go:179` ("can path through ally but cannot end on ally tile").
- ⚠️ `move_handler.go:155 // TODO: turn ownership validation` — anyone in the encounter can move whoever's turn is active.
- ⚠️ `move_handler.go:193 // TODO: look up creature size` (defaults to Medium) and `:209 // We use maxSpeed=30 as default` are still hard-coded.

### Phase 31: Altitude & Flying (`/fly`) ✅ / ⚠️
- `Distance3D` rounded to nearest 5ft (`altitude.go:13-19,22-24`).
- `ValidateFly` (`altitude.go:43-72`) — 1ft of altitude = 1ft of movement, rejects negative / equal / over-budget.
- `FallDamage` 1d6/10ft (`altitude.go:92-114`).
- Token altitude rendering (`AR↑30`-style suffix) lives outside this chunk's scope but Phase 31 dependency is on Phase 22 — verified in token-renderer tests via `pathfinding.go:142-147` ignoring flying ground occupants.
- Discord: `fly_handler.go:37-130` with confirm/cancel.
- Tests: `altitude_test.go` covers distance, fly cost, fall damage.
- ⚠️ Same advisory-lock / ownership gap as `/move` — `fly_handler.go` does not validate that the invoker owns the active turn.

### Phase 32: Distance Awareness (`/distance`) ✅
- `Distance3D` (Phase 31) + `FormatDistance` / `FormatRangeRejection` / `ResolveTarget` (`distance.go:15-56`).
- Discord handler: `distance_handler.go:24-138` (handles 1- and 2-arg forms via `combat.ResolveTarget`).
- Passive distance in attack feedback: `FormatAttackLog` (`attack.go:674-676`) prints distance for ranged attacks or melee >5ft.
- Range-rejection messages with both distances: `FormatRangeRejection` (`distance.go:23-25`).

### Phase 33: Cover Calculation ✅
- `CalculateCover` (`cover.go:74-102`): for each attacker corner draw 4 lines to target's corners, pick least-cover corner.
- DMG mapping `blockedToCover` (lines 154-166): 0→none, 1-2→half, 3→three-quarters, 4→full — matches spec line 1381.
- AC bonus / DEX-save bonus / EffectiveAC: `cover.go:32-52`.
- Creature-granted half cover: `creatureCover` (lines 170-186).
- Origin-based cover for AoE (closest origin corner, single line set): `CalculateCoverFromOrigin` (lines 106-136).
- `cover_test.go` covers walls, occupants, half/three-quarters/full, creature cover, AoE.

### Phase 34: Basic Attack Resolution (`/attack`) ⚠️
- Service: `Service.Attack` (`attack.go:773-869`), pure `ResolveAttack` (`attack.go:400-556`).
- Finesse auto-select STR/DEX: `abilityModForWeapon` (`attack.go:77-98`) — `max(strMod, dexMod)` for finesse weapons; same for monk weapons when monkLevel>0.
- Crits: nat-20 `CriticalHit`, nat-1 auto-miss (`attack.go:535-543`); doubled dice in `resolveWeaponDamage`.
- Auto-crit: `CheckAutoCrit` (`attack.go:645-666`) detects paralyzed / unconscious target within 5ft, fed into `buildAttackInput` at line 1049.
- Range validation + cover AC integration (`attack.go:417-422`).
- Distance shown in combat log (Phase 32).
- ❌ **No Discord handler.** `commands.go:42-66` registers `/attack` with target/weapon/gwm/twohanded options, but `router.go:198-204,228-230` routes the `gameCommands` slice (which contains `attack`) to either `StatusAwareStubHandler` (when reg deps present) or plain `stubHandler` (`router.go:495-497`) — both reply "/attack is not yet implemented." There is no `SetAttackHandler` in `router.go` and `grep -rn 'AttackHandler' internal/discord` returns nothing. Service-level `Service.Attack` callers: only test files (verified via `grep` — 30+ test references, zero non-test).
- Phase 34 done-when ("Integration tests verify attack flow") is satisfied at the service level (`attack_test.go`, `advantage_test.go`, `obscurement_integration_test.go` — `svc.Attack` is invoked dozens of times) but not exercisable through Discord.

### Phase 35: Advantage / Disadvantage ✅
- `DetectAdvantage` (`advantage.go:29-120`) covers attacker conditions (blinded/invisible/poisoned/prone/restrained), target conditions (blinded/invisible/restrained/stunned/paralyzed/unconscious/petrified/prone with the 5ft prone-distance flip at lines 110-116), context (HostileNearAttacker for ranged, long range, heavy + Small/Tiny size), DM overrides, hidden/obscurement.
- Cancellation per 5e: `resolveMode` (`advantage.go:123-136`) — any adv + any disadv → `AdvantageAndDisadvantage` (treated as Normal in `dice` package). Combat log surfaces both reason lists (`attack.go:693-701`).
- Tests: `advantage_test.go` lines 16-487 (28+ named cases including cancellation, hidden, DM overrides).

### Phase 36: Extra Attack & Two-Weapon Fighting ✅
- Extra Attack: `AttacksPerActionForLevel` (`turnresources.go:137-154`) picks highest threshold ≤ level; `resolveAttacksPerAction` (`turnresources.go:236-259`) iterates all multiclass entries and picks max — matches spec "multiclass highest wins".
- Per-attack deduction: `UseAttack` (`turnresources.go:158-164`) decrements; spent attacks remaining published in `FormatRemainingResources`.
- Unused attacks forfeited on `/done`: see `internal/combat/unused_resources.go` — out of this chunk's scope but the resource accounting on `Turn` row supports it.
- TWF: `OffhandAttack` (`attack.go:921-991`) validates main hand light + off-hand light, sets damage modifier=0 unless `HasFightingStyle(features, "two_weapon_fighting")` (lines 964-968) — matches spec.
- Tests: `attack_test.go` includes off-hand cases; `modifierflags_test.go:557` `TestServiceAttack_Reckless_BarbarianClass_OK`.

### Phase 37: Weapon Properties ⚠️
- Versatile two-handed: `--twohanded` propagates from `AttackCommand.TwoHanded` to `AttackInput.TwoHanded`; off-hand-occupied check at `attack.go:404-406`; `VersatileDamageExpression` (`attack.go:243-248`).
- Reach: `MaxRange` returns 10 for melee `reach` (`attack.go:113-119`).
- Heavy + Small/Tiny disadvantage: `advantage.go:88-91`.
- Loading: `ApplyLoadingLimit` (`attack.go:263-268`) caps to 1, Crossbow Expert override applied in `attack.go:809-811`.
- Thrown: `ThrownMaxRange` / `ThrownNormalRange` / `IsThrownInLongRange` (`attack.go:270-295`); used in `ResolveAttack` (lines 408-415, 506-510).
- Ammunition: `DeductAmmunition` / `RecoverAmmunition` / `GetAmmunitionName` (`attack.go:195-229`); auto-deducted in `Service.Attack` lines 819-836.
- Improvised: `ImprovisedWeapon` (1d4 bludgeoning, `attack.go:251-259`); profBonus zeroed unless Tavern Brawler (`attack.go:425-428`); thrown improvised range 20/60 (lines 413-415).
- ⚠️ **Missing: thrown weapon hand management.** Spec calls for the weapon to be removed from the hand after a thrown attack. `grep -rn 'thrown' internal/combat | grep -i 'unequip\|EquippedMainHand'` returns no logic that nils `EquippedMainHand` post-throw. The flag is passed through, the range math is right, but the hand is not emptied.
- ⚠️ Half-recovery is a one-shot helper; not auto-invoked from `EndCombat` (see Phase 26b).

### Phase 38: GWM / Sharpshooter / Reckless ✅
- Service-level prerequisite checks (`attack.go:788-797`):
  - GWM requires `HasFeat(features, "great-weapon-master")`.
  - Sharpshooter requires `HasFeat(features, "sharpshooter")`.
  - Reckless requires `HasBarbarianClass(classes)`.
- Pure-input weapon-type prerequisites (`attack.go:430-447`):
  - GWM rejects ranged or non-heavy weapons.
  - Sharpshooter rejects melee weapons.
  - Reckless rejects ranged weapons and finesse-using-DEX (line 444-447).
- Math: `-5 atk +10 dmg` for GWM/Sharpshooter at lines 467-472. Reckless feeds advantage via `AdvantageInput.Reckless` (`advantage.go:34-36`).
- Tests: `modifierflags_test.go` lines 32-617 covers each rejection path and the happy path with feats / Barbarian.
- ⚠️ "First attack only" qualifier on Reckless from spec — implementation does not track whether this is the attacker's first attack of the turn. The flag is allowed on every `/attack` invocation. Minor scoping gap.

## Cross-cutting risks

1. **Advisory-lock leak risk: low.** `AcquireTurnLock` always rolls back on error paths (`turnlock.go:39,45`) and `pg_advisory_xact_lock` is auto-released on tx commit/rollback. The risk is at the *caller* — handlers that don't currently hold a tx might forget to commit/rollback, but since the Discord handlers don't take the lock at all (Phase 27 wiring gap), there is no leak surface there yet.
2. **Race condition: out-of-turn writes possible from Discord today.** Because `move_handler.go`, `fly_handler.go`, and the various other player commands skip ownership validation and skip the advisory lock, two concurrent `/move` calls from different players targeting the same active turn could both succeed and both `UpdateTurnActions`. The integration tests in `turnlock_integration_test.go` cover this at the service surface but the production code path doesn't enter that surface.
3. **Initiative tracker drift.** Round changes, surprise removals, and HP changes all update combatant rows in the DB but no Discord message gets edited. Players will not see the auto-updated tracker the spec requires (line 1696).
4. **End-combat cleanup partial.** Concentration on lingering spells, ammunition recovery prompt, and timer cancellation are all missing from `EndCombat` (see Phase 26b). A boss fight that ends with a Concentration spell active will leave the spell concentration tracked indefinitely.
5. **Untested branches in attack pipeline.** `Service.Attack` branches for `IsImprovised`, `Thrown`, `HasCrossbowExpert`, `HasTavernBrawler` are exercised by unit tests with stubs but not by an end-to-end Discord-driver test, since no Discord handler reaches them.
6. **Hand-state desync after thrown attack.** The thrown property is computed but never updates `EquippedMainHand`. A javelin thrown at long range will continue to satisfy main-hand-equipped checks for the next attack/bonus action.
7. **Reckless first-attack-only constraint** is documented in spec but not enforced; flag can be set every attack.
8. **`maxSpeed` and creature-size hardcoding** in `move_handler.go:193,210` (Medium / 30ft) will give wrong path costs for Halflings (25ft), Tabaxi (40ft via Feline Agility), Large creatures, etc. once non-default species/sizes hit the table.

## Recommended follow-ups

1. **Wire `AttackHandler`, `CastHandler`, `BonusHandler`, `ShoveHandler`, `InteractHandler`** in `internal/discord/` and add the corresponding `SetAttackHandler` etc. on `CommandRouter`. Each handler should call `combat.AcquireTurnLockWithValidation` at the top, route exempt commands via `combat.IsExemptCommand`, and dispatch to the existing `combat.Service` methods.
2. **Replace the `move_handler.go:155` TODO with a real ownership + lock guard.** Apply the same fix to `fly_handler.go`. After this, the integration tests in `turnlock_integration_test.go` should be re-run end-to-end against a real Discord interaction.
3. **Auto-post + auto-update the initiative tracker message** in `#initiative-tracker` after `RollInitiative`, every `AdvanceTurn`, and on `EndCombat`. Persist `tracker_message_id` on `encounters` (new column) so subsequent updates use `ChannelMessageEdit`.
4. **Complete `EndCombat` cleanup**: invoke the concentration-end path, run `RecoverAmmunition` for each PC's spent arrows/bolts (probably tracked on `turns` or a new per-encounter counter), call `PauseCombatTimers`, and post a "combat ended" line to `#combat-log`.
5. **Remove main-hand on thrown attack** when the weapon is the only copy in inventory, or surface a reminder if a stack remains (`Service.Attack` after the ammunition block).
6. **Track first-attack-of-turn for Reckless** (e.g., a `reckless_used_this_turn bool` column on `turns`) and reject the flag on subsequent attacks.
7. **Look up `speed_ft` and creature size in `move_handler.go`** rather than the hardcoded 30ft / Medium defaults.
8. **First-combatant ping on `StartCombat`.** Either have the dashboard call a follow-up endpoint, or have `Service.StartCombat` enqueue the same `dmqueue` event used after `/done`.
9. **Add a unit test labeling diagonal-at-5ft** (the behavior is correct via `TestFindPath_Diagonal` at `pathfinding_test.go:32`, but a named test makes the spec-trace explicit).
10. **Add a service-level test for `ApplyLoadingLimit`** combined with Crossbow Expert + Extra Attack — verifies that a level-5 Fighter with Crossbow Expert and a heavy crossbow gets 2 attacks per action.
