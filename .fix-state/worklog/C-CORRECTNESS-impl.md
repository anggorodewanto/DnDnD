# C-CORRECTNESS bundle 3 worklog

Bundle: C-31, C-35-attacker-size, C-35-hostile-near, C-37, C-38,
C-40-charmed-attack, C-42. All seven landed in a single TDD pass.

## Per-task status

### C-31-fall-damage-unwired — CLOSED

- **Files changed**: `internal/combat/altitude.go`,
  `internal/combat/condition.go`, `internal/combat/service.go`,
  `internal/combat/bundle3_test.go`.
- **Hook**: `Service.ApplyCondition` calls the new
  `Service.applyFallDamageOnProne` whenever the applied condition is
  `prone` AND the combatant's `AltitudeFt > 0`. The helper:
    1. Rolls `FallDamage(altitude, s.roller)`.
    2. Resets the combatant's altitude to 0 via
       `UpdateCombatantPosition`.
    3. Funnels the damage through `Service.ApplyDamage` so
       resistances / vulnerabilities / concentration saves all fire.
    4. Returns a combat-log line that ApplyCondition appends to the
       caller-facing msgs slice.
- **Roller**: a new `Service.roller` field defaults to
  `dice.NewRoller(nil)` (crypto/rand); `Service.SetRoller` lets tests
  inject a deterministic roller without touching the existing per-call
  roller parameters used by `Attack`, `RollInitiative`, etc.
- **Tests added**:
  `TestApplyCondition_AirborneProne_AppliesFallDamage`,
  `TestApplyCondition_GroundedProne_NoFallDamage`.

### C-35-attacker-size — CLOSED

- **Files changed**: `internal/combat/attack.go`,
  `internal/combat/bundle3_test.go`.
- **Hook**: new `Service.populateAttackContext` runs after
  `buildAttackInput` inside `Service.Attack`, `Service.attackImprovised`,
  and `Service.OffhandAttack`. It populates `AttackInput.AttackerSize`
  from `Service.resolveAttackerSize` (creature_ref → creature.Size for
  NPCs; PCs default to Medium). Command-supplied non-empty values still
  win, so the Discord layer can override.
- **Tests added**:
  `TestPopulateAttackContext_ResolvesNPCCreatureSize`,
  `TestPopulateAttackContext_DefaultsPCSizeToMedium`,
  `TestPopulateAttackContext_CommandOverrideWins`.

### C-35-hostile-near — CLOSED

- **Files changed**: `internal/combat/attack.go` (same
  `populateAttackContext` helper), `internal/combat/bundle3_test.go`.
- **Hook**: `Service.detectHostileNear` lists encounter combatants and
  returns true for any living, non-incapacitated, opposite-faction
  combatant within 5ft (Chebyshev grid distance) of the attacker. The
  result populates `AttackInput.HostileNearAttacker` so
  `DetectAdvantage`'s ranged-with-hostile-adjacent path fires
  end-to-end. Command-supplied `true` still wins (so DM-override flows
  remain authoritative).
- **Test added**:
  `TestServiceAttack_AutoPopulatesHostileNear_RangedWithAdjacentHostile`.

### C-37-ammo-recovery — CLOSED (in-memory tracker)

- **Files changed**: `internal/combat/ammunition.go` (new),
  `internal/combat/service.go`, `internal/combat/attack.go`,
  `internal/combat/bundle3_test.go`.
- **Tracker**: new `AmmoSpentTracker` (thread-safe in-memory map keyed
  by `(encounter, combatant, ammoName)`) lives on `Service.ammoTracker`.
- **Spend hook**: `Service.Attack` calls `recordAmmoForAttack` after the
  weapon-ammunition deduction so every shot fires the tracker.
- **Recovery hook**: `EndCombat` now calls
  `Service.recoverEncounterAmmunition` immediately before the
  combat-condition cleanup. It iterates the snapshot, runs
  `RecoverAmmunition(spent)` on each PC's inventory items, persists via
  `UpdateCharacterInventory`, and clears the tracker for the encounter.
- **Public hook**: `Service.RecordAmmoSpent` exists so future test /
  Discord paths can record without going through the attack pipeline
  (e.g. ranged AoE / sprays).
- **Schema follow-up**: the `service.go:885` comment was replaced —
  the in-memory tracker resolves the recovery within a single-process
  encounter. Promoting to a persistent column is still a separate
  schema migration but is not blocking playtest.
- **Tests added**: `TestEndCombat_RecoversHalfSpentAmmunition`,
  `TestAmmoSpentTracker_NegativeAndEmptyAreNoop`,
  `TestAmmoSpentTracker_ClearEncounterIsolatesOtherEncounters`,
  `TestRecordAmmoSpent_NilTrackerIsSafe`.

### C-38-reckless-target-side — CLOSED

- **Files changed**: `internal/combat/advantage.go`,
  `internal/combat/attack.go`, `internal/combat/bundle3_test.go`.
- **Reader half**: `DetectAdvantage`'s target-condition loop now adds
  `"target reckless"` to `advReasons` when the target carries a
  `reckless` condition.
- **Writer half**: `Service.Attack` calls
  `Service.applyRecklessMarker` after `populatePostHitPrompts` when
  `cmd.Reckless` is set. The helper applies a transient `reckless`
  condition with `DurationRounds=1`, `ExpiresOn="start_of_turn"`, and
  `SourceCombatantID=attacker.ID`, so the existing condition-expiry
  pipeline clears it at the attacker's next start-of-turn.
  Idempotent — `HasCondition("reckless")` short-circuits.
- **Tests added**: `TestDetectAdvantage_TargetReckless_GrantsAdvantage`,
  `TestServiceAttack_Reckless_AppliesTargetSideMarker`.

### C-40-charmed-attack — CLOSED

- **Files changed**: `internal/combat/attack.go`,
  `internal/combat/bundle3_test.go`.
- **Hook**: new `validateCharmedAttack` (pure function over
  `attacker.Conditions` + `target.ID`) is called at the top of
  `Service.Attack` (before resource validation / weapon resolution) and
  the top of `Service.OffhandAttack`. `attackImprovised` is reached via
  `Service.Attack` so the same gate covers improvised attacks. Returns
  `fmt.Errorf("%s is charmed by %s and cannot attack them", ...)`.
- **Tests added**: `TestServiceAttack_Charmed_BlocksAttackOnCharmer`,
  `TestServiceAttack_Charmed_AllowsAttackOnNonCharmer`.

### C-42-exhaustion-speed — CLOSED

- **Files changed**: `internal/combat/turnresources.go`,
  `internal/combat/bundle3_test.go` (existing `turnresources_test.go`
  also extended).
- **Hook**: `Service.ResolveTurnResources` now routes through
  `EffectiveSpeedWithExhaustion(speed, conds, exhaustion)` on both the
  NPC default-30 branch and the PC race-aware branch. Conditions
  (grappled / restrained → 0) still take precedence; otherwise
  exhaustion level 2+ halves and level 5+ zeroes.
- **Tests added**:
  `TestResolveTurnResources_ExhaustionLevel2HalvesSpeed`,
  `TestResolveTurnResources_ExhaustionLevel5ZeroesSpeed`,
  `TestResolveTurnResources_ExhaustionLevel1Unchanged`,
  `TestResolveTurnResources_NPCExhaustionLevel2HalvesSpeed`.

## Validation

- `go test ./internal/combat/` — PASS (16.6s).
- `make test` — PASS.
- `make cover-check` — PASS (combat at 93.0%, well above the 85% gate).
- `make build` — PASS.

## Out-of-scope follow-ups (file these)

- Race-aware PC size lookup: currently PCs default to Medium for
  AttackerSize. A follow-up can plumb Race → Size through the combat
  Store interface to drop the default for non-Medium races (Halfling,
  Gnome, Goliath, ...).
- Discord-side surfacing: `RecoverAmmunition` recovery is silent on the
  ephemeral combat log. A `B-26b-ammo-recovery-prompt` task may want to
  post a per-PC summary line during the end-of-combat announcement.
- Persistent ammo-spent column: the in-memory tracker is correct for a
  single-process server within an encounter's lifetime, but a
  pod-restart mid-encounter would lose state. A small schema migration
  can promote this when the playtest reveals churn.
