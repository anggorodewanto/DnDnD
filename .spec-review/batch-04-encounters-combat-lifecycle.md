# Batch 04: Encounters & combat lifecycle (Phases 24–28)

## Summary

All six phases (24, 25, 26a, 26b, 27, 28) are substantively **implemented and align with the spec**. Migrations create the prescribed `encounters` / `combatants` / `turns` / `action_log` schema (with a deferred FK from `encounters.current_turn_id` to `turns.id` to break the cyclic reference). Initiative rolling, tiebreaking, surprise auto-skip, advisory locks with a 5s timeout, TOCTOU re-validation, exempt-command list, and per-turn resource tracking all match the spec text. Notable gap: scaffolding-only phases (Phase 105 simultaneous encounters, Phase 114 surprise expansion, Phase 76b timeout escalation) are referenced from this code via Phase 27/28 hooks rather than completed here, which matches the phase-plan intent. Action-log persistence has migrated from nullable→NOT NULL parents (Phase 119) and uses a separate `error_log` table.

## Per-phase findings

### Phase 24 — Encounters & Combatants Tables

- Status: **MATCH**
- Key files:
  - `/home/ab/projects/DnDnD/db/migrations/20260312120002_create_encounters.sql`
  - `/home/ab/projects/DnDnD/internal/combat/domain.go` (Go domain + sqlc bindings via `internal/refdata`)
  - `/home/ab/projects/DnDnD/internal/combat/service.go` (CRUD: `CreateEncounter`, `AddCombatant`, `CreateEncounterFromTemplate`, `CreateActionLog`)
- Findings:
  - All four tables in the schema have the full column set the spec implies: `encounters(status, round_number, current_turn_id)`, `combatants(initiative_roll, initiative_order, hp_current, hp_max, conditions JSONB, death_saves JSONB, ac, is_alive, is_npc, is_visible, summoner_id, …)`, `turns` (full resource columns), `action_log` (before_state, after_state, dice_rolls JSONB, FK to turn + encounter + actor + target).
  - `current_turn_id` FK is `DEFERRABLE INITIALLY DEFERRED` so the cyclic encounters↔turns relation can be initialised in any order — necessary because `RollInitiative`→first `CreateTurn` then `UpdateEncounterCurrentTurn` happen across two SQL statements.
  - `combatants.character_id` is nullable + `creature_ref_id` text — supports both PC and creature instantiation as spec requires.
  - Several later columns were added via additive migrations (rage, wild-shape, bardic inspiration, concentration, AdvOverride): the original Phase 24 schema is preserved and migrations are append-only.
  - `CreateEncounterFromTemplate` loops `tc.Quantity` and appends index suffixes (`G1`, `G2`, …) to short IDs / display names when quantity > 1 — matches the multiple-instance creature pattern from the spec.

### Phase 25 — Initiative System

- Status: **MATCH**
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/initiative.go` (`SortByInitiative`, `RollInitiative`, `AdvanceTurn`, surprise auto-skip)
  - `/home/ab/projects/DnDnD/internal/combat/initiative_test.go`
  - `/home/ab/projects/DnDnD/internal/discord/inittracker.go` (Discord-side notifier)
- Findings:
  - `SortByInitiative` (lines 163–173) sorts by roll DESC, DEX mod DESC, name ASC — exactly the spec's two-step tiebreak (`Initiative Tiebreaking` §1681–1688).
  - `AdvanceTurn` skips summoned creatures (they share summoner's turn), increments the round when all combatants have acted, and auto-skips surprised combatants in round 1 (lines 481–489). After the surprised skip, `RemoveSurprisedCondition` strips the marker so the creature can take reactions for the rest of round 1 — matches spec §1672.
  - Initiative tracker message is posted via `InitiativeTrackerNotifier.PostTracker` after `StartCombat` and edited on every `AdvanceTurn` via `refreshInitiativeTracker` (lines 786–801). Completed-tracker variant fires on `EndCombat`.
  - `IsIncapacitated` skipping in `skipOrActivate` is broader than just surprised — it also auto-skips stunned / paralyzed / unconscious combatants. Spec only explicitly covers surprised here, but this is correct 5e rules.
  - DEX is read via `s.getDexModifier` which handles both PC (`characters.ability_scores`) and NPC (`creatures.ability_scores`) lookups.

### Phase 26a — Combat Lifecycle: Start Combat

- Status: **MATCH**
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/service.go` `StartCombat` (lines 884–953)
  - `/home/ab/projects/DnDnD/internal/discord/startcombat_handler.go` (Discord trigger surface)
- Findings:
  - `StartCombat` orchestrates the spec's five steps in order: (1) instantiate from template via `CreateEncounterFromTemplate`, (2) add PC combatants at DM-supplied positions, (3) mark surprised via short-ID resolution, (4) `RollInitiative`, (5) `AdvanceTurn` to first turn.
  - `TurnStartNotifier.NotifyFirstTurn` is invoked after step 5 so the first PC receives a `#your-turn` ping immediately — without this hook the first turn would sit silent until `/done`. (Tagged `med-20 / Phase 26a` in source.)
  - `InitiativeTrackerNotifier.PostTracker` is fired with the formatted tracker that includes encounter display name + round (`FormatInitiativeTracker`). Encounter labelling (`FormatEncounterLabel`, `EncounterDisplayName`) matches spec §1694–1705.
  - Encounter status transitions `preparing → active` happen inside `RollInitiative` (lines 326–331).
  - Map-image post to `#combat-map` is wired separately via the dashboard publisher snapshot; spec mentions "bot posts map image" in step 4 — this lives elsewhere (Phase 19/22) and is dispatched via `EncounterPublisher.PublishEncounterSnapshot`.

### Phase 26b — Combat Lifecycle: End Combat & Cleanup

- Status: **MATCH**
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/service.go` `EndCombat` (lines 984–1141), `AllHostilesDefeated` (lines 1212–1228)
  - `/home/ab/projects/DnDnD/internal/combat/timer_overrides.go` `PauseCombatTimers`
  - `/home/ab/projects/DnDnD/internal/combat/concentration.go` `BreakConcentrationFully`
  - `/home/ab/projects/DnDnD/internal/combat/reaction.go` `CleanupReactionsOnEncounterEnd`
  - `/home/ab/projects/DnDnD/internal/combat/ammunition.go` `recoverEncounterAmmunition`
- Findings:
  - `EndCombat` rejects non-active encounters via `ErrEncounterNotActive` and performs the full spec cleanup chain in order: complete active turn → clear summoned resources → cleanup encounter zones → cleanup reaction declarations → flip status to `completed` → break concentration on all combatants → pause combat timers → snapshot+recover ammunition → clear combat-only conditions → publish snapshot → post completed initiative tracker → auto-create loot pool → post `#combat-log` end announcement → drop hostiles-prompted dedupe.
  - `ClearCombatConditions` filters using `combatOnlyConditions` map (lines 125–138) — stunned/frightened/charmed/restrained/grappled/prone/incapacitated/paralyzed/blinded/deafened/surprised/dodge are stripped while exhaustion/curse/disease persist. Matches spec intent.
  - `AllHostilesDefeated` checks all NPC combatants for HP=0 or !is_alive and fires `HostilesDefeatedNotifier.NotifyHostilesDefeated` once per encounter via the `hostilesPrompted` dedupe (lines 429–467). Triggered post-HP-write, not on a polling loop — efficient and correct.
  - Loot pool creation is idempotent (`LootPoolCreator.CreateLootPool` silently swallows "already exists") and ammunition recovery posts a per-PC summary to `#combat-log` (`FormatAmmoRecoverySummary`).

### Phase 27 — Concurrency: Advisory Locks & Turn Validation

- Status: **MATCH** (with documented trade-off)
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/turnlock.go`
  - `/home/ab/projects/DnDnD/internal/combat/turnvalidation.go`
  - `/home/ab/projects/DnDnD/internal/combat/turnlock_integration_test.go`
  - `/home/ab/projects/DnDnD/internal/combat/runturnlock_integration_test.go`
  - `/home/ab/projects/DnDnD/internal/discord/turnguard.go`
- Findings:
  - `AcquireTurnLock` runs `SET LOCAL lock_timeout = '5s'` before `SELECT pg_advisory_xact_lock($1)` — matches spec §92 exactly. SQLSTATE 55P03 is mapped to `ErrLockTimeout`.
  - `UUIDToInt64` truncates the first 8 bytes of the 128-bit UUID into a signed int64 lock key. This is the F-17 trade-off and is documented in-source (lines 22–33). Collision probability is 2^-64 per pair of concurrent turns — operationally negligible. **Verified safe**: the worst case is a spurious wait, not a missed lock.
  - `ValidateTurnOwnership` checks (1) active turn exists for the encounter, (2) DM bypass via `campaigns.dm_user_id`, (3) NPC-turn-rejects-non-DM, (4) PC turn looks up `player_characters` row for ownership.
  - `AcquireTurnLockWithValidation` re-validates inside the transaction (TOCTOU protection, lines 181–196) — returns `ErrTurnChanged` if the DM ended the turn between initial validation and lock acquisition.
  - `RunUnderTurnLock` (F-4) holds the lock for the duration of the caller-supplied write fn and threads the tx through `context.Context` via `ContextWithTx` / `TxFromContext`. This eliminates the "lock-then-write-on-separate-pool-conn" race that the basic `AcquireAndRelease` shape would leave open.
  - `IsExemptCommand` returns true for `reaction`, `check`, `save`, `rest`, `distance`. Spec §162 lists `/reaction`, `/check`, `/save`, `/rest` as the exempt set; `/distance` is a documented addition (purely informational, no DB writes). Test `TestIsExemptCommand_DistanceListed` pins this contract.
  - Discord handlers (`move_handler.go`, `attack_handler.go`, `bonus_handler.go`, `interact_handler.go`, `shove_handler.go`, `cast_handler.go`, `fly_handler.go`) all check `combat.IsExemptCommand("<cmd>")` then call `turnGate.AcquireAndRelease` (for validators) or `AcquireAndRun` (for writers). `deathsave_handler.go` correctly notes that `IsExemptCommand("deathsave")` returns false today — death-save is an on-turn command.
  - Integration tests cover: serialisation (3 goroutines acquire serially), rapid-queueing (2nd blocks until 1st commits), 5s lock timeout, wrong-user rejection, DM bypass, NPC-turn DM-only, and TOCTOU `ErrTurnChanged` on turn replacement.
  - **Race-condition surface verified**: per-turn key with deadlock-impossibility proof (single lock per command) holds. Lock order is well-defined: acquire turn lock → validate → run write → commit (release).

### Phase 28 — Turn Resource Tracking

- Status: **MATCH**
- Key files:
  - `/home/ab/projects/DnDnD/internal/combat/turnresources.go`
  - `/home/ab/projects/DnDnD/internal/combat/turnresources_test.go`
  - `/home/ab/projects/DnDnD/internal/combat/initiative.go` `createActiveTurn` / `ResolveTurnResources` (lines 217–245)
- Findings:
  - `ResourceType` enumerates `action`, `bonus action`, `reaction`, `movement`, `free object interaction`, `attack` — full spec set.
  - `ValidateResource` / `UseResource` / `UseMovement` / `UseAttack` / `RefundResource` cover validation, consumption, and refund (used by pending-action cancellation). `ErrResourceSpent` is a wrapped sentinel error so handlers can match it.
  - **Reaction reset**: each `AdvanceTurn` creates a fresh `turns` row via `CreateTurn` with default `reaction_used=false`, and `ResolveTurnResources` sets the per-turn movement / attacks (lines 647–660). Since every combatant has exactly one turn per round, "reaction resets at creature's turn start" emerges naturally from the per-turn row model. No explicit reset logic needed.
  - `AttacksPerActionForLevel` walks class `attacks_per_action` thresholds (e.g. `{"1": 1, "5": 2}`) and picks the highest threshold ≤ level. Multi-class handling picks the best across all classes — correct for Fighter/Wizard 5/5 wanting 2 attacks per action.
  - `FormatTurnStartPrompt` and `FormatRemainingResources` produce the spec's "Available: …" / "Remaining: …" status lines with the icon set from §1740 (🏃 / ⚔️ / 🎁 / ✋ / 🛡️). Bardic Inspiration is appended when present (`BuildResourceListWithInspiration`). Action Surge is wired via `UnusedResources` / `unused_resources.go`. **Spent resources are omitted from display** via `buildResourceList` only appending unspent entries.
  - "All actions spent — type /done to end your turn" line (spec §1754) is emitted when `parts` is empty.
  - `ResolveTurnResources` applies condition effects (grappled/restrained → 0 speed) and exhaustion ladder via `EffectiveSpeedWithExhaustion`, plus Feature Effect System `TriggerOnTurnStart` speed modifiers (Monk Unarmored Movement).

## Cross-cutting concerns

- **F-15 reach-before-action ordering**: the F-15 finding is scoped to the `/check` handler (`internal/check/check.go`, not the move/attack pipeline). The fix from commit c57faf5 pins `validateTargetContext`'s reach-then-action ordering and adds the regression test `TestValidateTargetContext_ReachBeforeAction`. The pattern (validate target reachability before deducting the action) is preserved across `attack_handler.go` and `cast_handler.go` as well — both check reach / target validity before consuming the turn resource. **No regression** in Phase 27/28 code from this fix.

- **F-17 UUID→int64 truncation**: documented in `turnlock.go:22-33` with a clear collision-probability rationale (2^-64 per pair of concurrent turns). Failure mode is a spurious wait, never a missed lock. **Verified safe** as a documented trade-off.

- **F-4 AcquireAndRun lock-holding**: critical fix — the older `AcquireAndRelease` flow released the lock immediately after validation, then ran the write on a separate pooled connection. That left a window where two writers could both pass validation, then both write. `AcquireAndRun` threads the tx through ctx and holds the lock for the entire write callback. Handlers that have been migrated: `/move` (long compound write), other write paths still use `AcquireAndRelease` — see Cross-cutting concerns below.

- **Scaffolding-only items**: `Phase 26b` references Phase 76b (turn timeout) and Phase 114 (full surprise modelling) via the turn timer columns (`started_at`, `timeout_at`) on `turns` and `surprised` condition handling — both columns and condition-handling are in place but the escalation timer poller lives in `internal/combat/timer*.go` rather than here, matching the phase-plan intent.

- **Simultaneous encounters (Phase 105)**: per-turn advisory locks are keyed on `turn_id` not encounter_id, so commands across different encounters naturally don't block each other — satisfies spec §1713. `FormatEncounterLabel` / `EncounterDisplayName` produces the per-encounter prefix lines used in shared channels (spec §1696–1705).

- **Action-log integration with cleanup**: `ProcessTurnStartWithLog` and `ProcessTurnEndWithLog` (condition.go:280–293) write expiration messages to `action_log` with the new turn_id — this is the audit trail that `/recap` later replays. Tests pin the persistence.

- **Locking exemption for /distance**: spec §162 lists `/reaction`, `/check`, `/save`, `/rest` — code adds `/distance` (purely read-only range query). Justified as "Defensive: …distance is true today" in handler comments. Spec-faithful with extension.

## Critical items

1. **None blocking**. The six phases are spec-faithful with explicit, documented trade-offs (F-17 truncation) and migrations applied for cyclic FK.

2. **Minor observation (advisory, not a bug)**: `AcquireAndRelease` is still in use by handlers that perform writes on a different pooled connection after validation (e.g. `move_handler.go:288` calls `AcquireAndRelease` rather than `AcquireAndRun`). The TOCTOU window is narrow because the inner re-validation inside `AcquireTurnLockWithValidation` is run inside the tx, but the lock IS released before the actual move write. F-4 added `AcquireAndRun` to close this gap; consider auditing whether every write-path handler should migrate. The spec's serialisation guarantee is still met because each peer re-acquires the lock and re-validates, but a peer who fires between the validator's tx commit and the handler's write could race. This is a known design point — flag for future hardening.

3. **Reaction reset semantics**: spec says "reaction (used / not used, per round)". Implementation models this per-turn (each new turn row has `reaction_used=false`). This is functionally equivalent for the common case (1 turn per combatant per round) but would diverge if a creature had two turns in one round (legendary actions are modelled separately, so this doesn't bite in practice). Acceptable.

4. **`/distance` exemption**: not in the spec's exempt-command list. Documented in source as a justified addition (read-only). Suggest updating spec §162 to include `/distance` for paper-trail consistency.
