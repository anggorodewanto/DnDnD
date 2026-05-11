package combat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ApplyDamageResistances applies resistance, immunity, and vulnerability rules
// to raw damage. Returns the final damage amount and a reason string.
// Rules:
//   - Immunity trumps all (damage = 0)
//   - Resistance + Vulnerability cancel out (normal damage)
//   - Resistance alone halves damage (rounded down)
//   - Vulnerability alone doubles damage
//   - Petrified condition grants resistance to all damage types
func ApplyDamageResistances(rawDamage int, damageType string, resistances, immunities, vulnerabilities []string, conditions []CombatCondition) (int, string) {
	dt := strings.ToLower(damageType)

	isImmune := containsCI(immunities, dt)
	if isImmune {
		return 0, "immune to " + dt
	}

	isResistant := containsCI(resistances, dt) || hasCondition(conditions, "petrified")
	isVulnerable := containsCI(vulnerabilities, dt)

	if isResistant && isVulnerable {
		return rawDamage, "resistance and vulnerability cancel"
	}
	if isResistant {
		return rawDamage / 2, "resistance to " + dt
	}
	if isVulnerable {
		return rawDamage * 2, "vulnerable to " + dt
	}

	return rawDamage, ""
}

// containsCI checks if the slice contains the target string (case-insensitive).
func containsCI(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// AbsorbTempHP applies damage to temporary HP first, returning remaining damage
// and new temp HP value.
func AbsorbTempHP(damage int, tempHP int) (remainingDamage int, newTempHP int) {
	if damage <= tempHP {
		return 0, tempHP - damage
	}
	return damage - tempHP, 0
}

// GrantTempHP returns the higher of current temp HP or new temp HP (no stacking).
func GrantTempHP(currentTempHP int, newTempHP int) int {
	if newTempHP > currentTempHP {
		return newTempHP
	}
	return currentTempHP
}

// ExhaustionEffectiveSpeed returns effective speed after exhaustion effects.
// Level 2: speed halved. Level 5+: speed reduced to 0.
func ExhaustionEffectiveSpeed(baseSpeed int, exhaustionLevel int) int {
	if exhaustionLevel >= 5 {
		return 0
	}
	if exhaustionLevel >= 2 {
		return baseSpeed / 2
	}
	return baseSpeed
}

// ExhaustionEffectiveMaxHP returns effective max HP after exhaustion effects.
// Level 4+: max HP halved.
func ExhaustionEffectiveMaxHP(baseMaxHP int, exhaustionLevel int) int {
	if exhaustionLevel >= 4 {
		return baseMaxHP / 2
	}
	return baseMaxHP
}

// ExhaustionRollEffect returns the roll effects from exhaustion.
// Level 1+: disadvantage on ability checks.
// Level 3+: disadvantage on attack rolls and saving throws.
func ExhaustionRollEffect(exhaustionLevel int) (checkDisadv bool, attackDisadv bool, saveDisadv bool) {
	if exhaustionLevel >= 3 {
		return true, true, true
	}
	if exhaustionLevel >= 1 {
		return true, false, false
	}
	return false, false, false
}

// IsExhaustedToDeath returns true if exhaustion level is 6 (death).
func IsExhaustedToDeath(exhaustionLevel int) bool {
	return exhaustionLevel >= 6
}

// CheckConditionImmunity returns true if the creature is immune to the given condition.
func CheckConditionImmunity(conditionName string, immunities []string) bool {
	return containsCI(immunities, conditionName)
}

// ApplyDamageInput holds inputs for the centralized damage pipeline. Every
// production damage call site funnels through Service.ApplyDamage with this
// struct so that Phase 42 (resistance / immunity / vulnerability, temp HP,
// exhaustion HP-halving and exhaustion-6 death) is uniformly applied before
// the underlying applyDamageHP write.
type ApplyDamageInput struct {
	EncounterID uuid.UUID
	Target      refdata.Combatant
	RawDamage   int
	DamageType  string
	// Override skips R/I/V + temp HP absorption. Used for DM dashboard
	// overrides and undo paths where the caller is asserting an exact HP
	// outcome (or restoring a previous state) and per-target damage typing
	// is not meaningful.
	Override bool
	// IsCritical indicates the incoming hit is a critical. Forwarded to the
	// Phase 43 damage-at-0-HP rule (ApplyDamageAtZeroHP) so a crit on a dying
	// combatant adds two death-save failures instead of one. Damage rolls
	// that are not attack-based (AoE saves, ongoing damage) pass false.
	IsCritical bool
}

// ApplyDamageResult holds outputs of Service.ApplyDamage.
type ApplyDamageResult struct {
	Updated      refdata.Combatant
	FinalDamage  int
	AbsorbedTemp int
	NewTempHP    int32
	NewHP        int32
	IsAlive      bool
	Reason       string
	// Killed is true when the damage event triggered exhaustion-6 death,
	// instant-death overflow (Phase 43), or accumulated 3 death-save
	// failures via damage-at-0-HP.
	Killed bool
	// InstantDeath is true when overflow damage >= max HP killed the target
	// outright (PHB instant-death rule). Implies Killed and !IsAlive.
	InstantDeath bool
	// DeathSaveOutcome carries the optional outcome from a Phase 43
	// drop-to-0 or damage-at-0-HP routing. Nil when no death-save event
	// fired (e.g. target survived above 0 HP, NPC corpse path).
	DeathSaveOutcome *DeathSaveOutcome
}

// ApplyDamage is the single seam for every production damage write. It
// resolves the target's resistances / immunities / vulnerabilities, runs
// temp-HP absorption, applies exhaustion HP-halving (level 4+) and
// exhaustion-6 death, then funnels the resulting HP delta through
// applyDamageHP so concentration saves and unconscious-at-0-HP still fire.
//
// For PCs, R/I/V default to empty (the PC schema does not yet carry
// per-character damage modifiers — race/feat-driven resistances will plumb
// through here once Phase 45 wires them). For NPCs, R/I/V come from the
// referenced creature row.
//
// Override=true short-circuits R/I/V + temp HP and treats RawDamage as the
// exact HP delta. This is used by DM dashboard overrides and undo paths.
func (s *Service) ApplyDamage(ctx context.Context, input ApplyDamageInput) (ApplyDamageResult, error) {
	if input.RawDamage < 0 {
		return ApplyDamageResult{}, errors.New("ApplyDamage: RawDamage must be non-negative")
	}

	target := input.Target

	// Exhaustion 6 = instant death regardless of incoming damage.
	if IsExhaustedToDeath(int(target.ExhaustionLevel)) {
		updated, err := s.applyDamageHP(ctx, input.EncounterID, target.ID, target.HpCurrent, 0, 0, false)
		if err != nil {
			return ApplyDamageResult{}, fmt.Errorf("ApplyDamage exhaustion-6 kill: %w", err)
		}
		// Phase 17: refresh the card on the exhaustion-6 kill path too so
		// the HP-to-zero transition is visible immediately.
		s.notifyCardUpdate(ctx, updated)
		return ApplyDamageResult{
			Updated:     updated,
			FinalDamage: int(target.HpCurrent),
			NewHP:       0,
			NewTempHP:   0,
			IsAlive:     false,
			Reason:      "exhaustion 6: dead",
			Killed:      true,
		}, nil
	}

	// Cap currentHP to the exhaustion-effective max (level 4+ halves max HP).
	// The cap only applies when exhaustion is in the halving band; we never
	// rewrite HP based on HpMax outside that case (test fixtures may leave
	// HpMax=0 without expecting a cap).
	currentHP := int(target.HpCurrent)
	if int(target.ExhaustionLevel) >= 4 {
		effectiveMaxHP := ExhaustionEffectiveMaxHP(int(target.HpMax), int(target.ExhaustionLevel))
		if currentHP > effectiveMaxHP {
			currentHP = effectiveMaxHP
		}
	}

	adjusted := input.RawDamage
	reason := ""
	tempHP := int(target.TempHp)
	absorbed := 0

	if !input.Override {
		resistances, immunities, vulnerabilities, conditions, err := s.resolveDamageProfile(ctx, target)
		if err != nil {
			return ApplyDamageResult{}, fmt.Errorf("ApplyDamage resolving profile: %w", err)
		}
		adjusted, reason = ApplyDamageResistances(input.RawDamage, input.DamageType, resistances, immunities, vulnerabilities, conditions)
		var remaining int
		remaining, tempHP = AbsorbTempHP(adjusted, tempHP)
		absorbed = adjusted - remaining
		adjusted = remaining
	}

	rawNewHP := currentHP - adjusted
	newHP := rawNewHP
	// med-43 / Phase 47: preserve a negative newHP for wild-shaped targets
	// so applyDamageHP can derive the overflow used to apply excess damage
	// to the Druid's original HP pool. For all other targets we keep the
	// historical "clamp to 0" semantics by leaving the clamp logic intact.
	isWildShaped := target.IsWildShaped && target.WildShapeOriginal.Valid
	if newHP < 0 && !isWildShaped {
		newHP = 0
	}

	// Phase 43: PC death-save routing. NPCs and wild-shaped targets are
	// unaffected — wild-shape uses its own auto-revert path inside
	// applyDamageHP, and NPCs die outright at 0 HP. adjusted == 0 (immunity,
	// full temp-HP absorption) is a no-op for the death-save state machine.
	dsRouting, err := s.routePhase43DeathSave(ctx, target, adjusted, rawNewHP, newHP, isWildShaped, input.IsCritical)
	if err != nil {
		return ApplyDamageResult{}, err
	}

	// PCs at 0 HP go unconscious + dying (still alive); NPCs die outright.
	// Instant death and 3-failure death from the death-save routing above
	// override the default "still alive" outcome for PCs.
	isAlive := newHP > 0 || !target.IsNpc
	if dsRouting.outcome != nil && !dsRouting.outcome.IsAlive && !dsRouting.outcome.IsStable {
		isAlive = false
	}

	updated, err := s.applyDamageHP(ctx, input.EncounterID, target.ID, target.HpCurrent, int32(newHP), int32(tempHP), isAlive)
	if err != nil {
		return ApplyDamageResult{}, fmt.Errorf("ApplyDamage persisting HP: %w", err)
	}

	// D-46-rage-auto-end-quiet — mark the raging target so the
	// no-attack-no-damage auto-end check at end-of-turn doesn't fire while
	// the barbarian is actively taking hits. Only fires when adjusted damage
	// actually reached HP (immunity / temp-HP absorption don't count).
	// We carry IsRaging + RageRoundsRemaining over from the input target
	// because UpdateCombatantHP only rewrites HP / temp_hp / is_alive, so
	// the persisted rage columns are unchanged. (D-46)
	postHPCombatant := updated
	postHPCombatant.IsRaging = target.IsRaging
	postHPCombatant.RageRoundsRemaining = target.RageRoundsRemaining
	postHPCombatant.RageAttackedThisRound = target.RageAttackedThisRound
	postHPCombatant.RageTookDamageThisRound = target.RageTookDamageThisRound
	if adjusted > 0 {
		s.markRageTookDamage(ctx, postHPCombatant)
	}

	// D-46-rage-end-on-unconscious — a raging combatant who hit 0 HP drops
	// rage along with the dying-condition bundle. applyDamageHP already
	// applied unconscious+prone via ConditionsForDying; here we just clear
	// the rage columns.
	s.maybeEndRageOnUnconscious(ctx, postHPCombatant)

	// Phase 17: refresh the persistent character card so HP / temp-HP /
	// alive flag stay in sync with combat. Silent no-op for NPCs and when
	// no card updater is wired (e.g. tests, bot offline).
	s.notifyCardUpdate(ctx, updated)

	return ApplyDamageResult{
		Updated:          updated,
		FinalDamage:      adjusted,
		AbsorbedTemp:     absorbed,
		NewTempHP:        int32(tempHP),
		NewHP:            int32(newHP),
		IsAlive:          isAlive,
		Reason:           reason,
		Killed:           !isAlive,
		InstantDeath:     dsRouting.instantDeath,
		DeathSaveOutcome: dsRouting.outcome,
	}, nil
}

// phase43DeathSaveResult is the internal return of routePhase43DeathSave.
type phase43DeathSaveResult struct {
	outcome      *DeathSaveOutcome
	instantDeath bool
}

// routePhase43DeathSave evaluates the Phase 43 death-save state machine for a
// PC damage event. There are exactly three outcomes:
//
//   - drop-to-0 (prevHP > 0, newHP <= 0): ProcessDropToZeroHP decides between
//     instant death (overflow >= maxHP) and the dying state.
//   - damage-at-0 (prevHP <= 0): if overflow (= adjusted) >= maxHP, instant
//     death; else ApplyDamageAtZeroHP increments death save failures (+1 or
//     +2 on crit) and persists them.
//   - any other shape (heal, NPC, wild-shape, immune): no-op (nil outcome).
//
// The helper persists death save tallies on the damage-at-0 path; the caller
// is responsible for applying the unconscious / prone bundle via
// applyDamageHP after the HP write.
func (s *Service) routePhase43DeathSave(ctx context.Context, target refdata.Combatant, adjusted, rawNewHP, newHP int, isWildShaped, isCrit bool) (phase43DeathSaveResult, error) {
	if target.IsNpc || isWildShaped || adjusted <= 0 {
		return phase43DeathSaveResult{}, nil
	}
	maxHP := int(target.HpMax)

	// Drop-to-0: overflow is the pre-clamp HP deficit.
	if target.HpCurrent > 0 && newHP <= 0 {
		overflow := 0
		if rawNewHP < 0 {
			overflow = -rawNewHP
		}
		outcome := ProcessDropToZeroHP(target.DisplayName, overflow, maxHP)
		return phase43DeathSaveResult{
			outcome:      &outcome,
			instantDeath: outcome.TokenState == TokenDead,
		}, nil
	}

	// Damage at 0 HP path. Only fires for PCs that were already at or below
	// 0 HP — i.e. dying combatants receiving another tick of damage.
	if target.HpCurrent > 0 {
		return phase43DeathSaveResult{}, nil
	}

	// Instant-death overflow rule still wins before tallying failures.
	if CheckInstantDeath(adjusted, maxHP) {
		outcome := ProcessDropToZeroHP(target.DisplayName, adjusted, maxHP)
		return phase43DeathSaveResult{outcome: &outcome, instantDeath: true}, nil
	}

	ds, err := ParseDeathSaves(target.DeathSaves.RawMessage)
	if err != nil {
		return phase43DeathSaveResult{}, fmt.Errorf("ApplyDamage parsing death saves: %w", err)
	}
	outcome := ApplyDamageAtZeroHP(target.DisplayName, ds, isCrit)
	if _, err := s.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         target.ID,
		DeathSaves: MarshalDeathSaves(outcome.DeathSaves),
	}); err != nil {
		return phase43DeathSaveResult{}, fmt.Errorf("ApplyDamage persisting death saves: %w", err)
	}
	return phase43DeathSaveResult{outcome: &outcome}, nil
}

// resolveDamageProfile returns the target's R/I/V slices and parsed conditions.
// PCs default to empty R/I/V (not yet schema-tracked); NPCs pull from the
// referenced creature row, falling back to empty on missing creature.
func (s *Service) resolveDamageProfile(ctx context.Context, target refdata.Combatant) ([]string, []string, []string, []CombatCondition, error) {
	conditions, err := ListConditions(target.Conditions)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing conditions: %w", err)
	}
	if !target.IsNpc || !target.CreatureRefID.Valid {
		return nil, nil, nil, conditions, nil
	}
	creature, err := s.store.GetCreature(ctx, target.CreatureRefID.String)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, conditions, nil
		}
		return nil, nil, nil, nil, fmt.Errorf("looking up creature %s: %w", target.CreatureRefID.String, err)
	}
	return creature.DamageResistances, creature.DamageImmunities, creature.DamageVulnerabilities, conditions, nil
}
