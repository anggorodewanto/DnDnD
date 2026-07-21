package combat

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// RageDamageBonus returns the rage damage bonus for a given barbarian level.
func RageDamageBonus(barbarianLevel int) int {
	if barbarianLevel >= 16 {
		return 4
	}
	if barbarianLevel >= 9 {
		return 3
	}
	return 2
}

// RageUsesPerDay returns the number of rage uses per day for a given barbarian level.
// Returns -1 for unlimited (level 20).
func RageUsesPerDay(barbarianLevel int) int {
	if barbarianLevel >= 20 {
		return -1 // unlimited
	}
	if barbarianLevel >= 17 {
		return 6
	}
	if barbarianLevel >= 12 {
		return 5
	}
	if barbarianLevel >= 6 {
		return 4
	}
	if barbarianLevel >= 3 {
		return 3
	}
	return 2
}

// RageFeature returns the FeatureDefinition for Rage at the given barbarian level.
func RageFeature(barbarianLevel int) FeatureDefinition {
	dmgBonus := RageDamageBonus(barbarianLevel)

	return FeatureDefinition{
		Name:   "Rage",
		Source: "barbarian",
		Effects: []Effect{
			{
				Type:     EffectModifyDamageRoll,
				Trigger:  TriggerOnDamageRoll,
				Modifier: dmgBonus,
				Conditions: EffectConditions{
					WhenRaging:  true,
					AttackType:  "melee",
					AbilityUsed: "str",
				},
			},
			{
				Type:        EffectGrantResistance,
				Trigger:     TriggerOnTakeDamage,
				DamageTypes: []string{"bludgeoning", "piercing", "slashing"},
				Conditions: EffectConditions{
					WhenRaging: true,
				},
			},
			{
				Type:    EffectConditionalAdvantage,
				Trigger: TriggerOnCheck,
				On:      "advantage",
				Conditions: EffectConditions{
					WhenRaging:  true,
					AbilityUsed: "str",
				},
			},
			{
				Type:    EffectConditionalAdvantage,
				Trigger: TriggerOnSave,
				On:      "advantage",
				Conditions: EffectConditions{
					WhenRaging:  true,
					AbilityUsed: "str",
				},
			},
		},
	}
}

// FastMovementFeature returns the FeatureDefinition for Fast Movement, the 2024
// Barbarian L5 feature: "Your speed increases by 10 feet while you aren't wearing
// Heavy armor." Unlike Monk Unarmored Movement (gated on wearing NO armor + NO
// shield), this only cares about Heavy armor — medium, light, and no armor all
// qualify, and a shield never blocks it. The flat +10 does not scale with level.
// Folded into turn-start speed via turnStartSpeedBonus at TriggerOnTurnStart.
func FastMovementFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Fast Movement",
		Source: "barbarian",
		Effects: []Effect{
			{
				Type:     EffectModifySpeed,
				Trigger:  TriggerOnTurnStart,
				Modifier: 10,
				Conditions: EffectConditions{
					NotWearingHeavyArmor: true,
				},
			},
		},
	}
}

// RageRounds is the hard duration cap on a Rage: 2024 PHB says 10 minutes,
// which is 100 rounds. The rage almost always lapses long before this — the
// turn-by-turn sustain check (ShouldRageEndOnTurnEnd) is the real clock — but
// the cap is enforced at turn start via ShouldRageEndOnTurnStart.
const RageRounds = 100

// ValidateRageActivation checks all preconditions for STARTING a rage.
// Returns an error if any precondition fails. An already-raging barbarian is
// routed to the bonus-action extension path before this is reached, so
// isRaging remains a genuine error here (a second fresh rage would burn a
// second daily use).
func ValidateRageActivation(isRaging bool, armorType string) error {
	if isRaging {
		return fmt.Errorf("already raging")
	}
	if armorType == "heavy" {
		return fmt.Errorf("cannot rage while wearing heavy armor")
	}
	return nil
}

// FormatRageActivation returns the combat log string for rage activation.
func FormatRageActivation(name string, ragesRemaining int) string {
	return fmt.Sprintf("\U0001f525  %s enters a Rage! (%d rages remaining today)", name, ragesRemaining)
}

// FormatRageEnd returns the combat log string for rage ending.
func FormatRageEnd(name string, reason string) string {
	return fmt.Sprintf("\U0001f525  %s's Rage ends \u2014 %s", name, reason)
}

// FormatRageEndVoluntary returns the combat log string for voluntarily ending rage.
func FormatRageEndVoluntary(name string) string {
	return fmt.Sprintf("\U0001f525  %s ends their Rage", name)
}

// FormatRageExtend returns the combat log string for spending a Bonus Action to
// extend an active Rage (2024 PHB sustain option).
func FormatRageExtend(name string) string {
	return fmt.Sprintf("\U0001f525  %s roars and extends their Rage (Bonus Action)", name)
}

// ApplyRageToCombatant starts a rage in the given encounter round.
func ApplyRageToCombatant(c refdata.Combatant, roundNumber int32) refdata.Combatant {
	c.IsRaging = true
	c.RageRoundsRemaining = sql.NullInt32{Int32: RageRounds, Valid: true}
	c.RageAttackedThisRound = false
	c.RageTookDamageThisRound = false
	c.RageStartedRound = sql.NullInt32{Int32: roundNumber, Valid: true}
	return c
}

// ExtendRageOnCombatant refreshes an active rage's "until the end of your next
// turn" horizon to the given round — the Bonus Action sustain option. It leaves
// the daily-use count and the RageRounds cap untouched: extending costs an
// action economy slot, not a rage.
func ExtendRageOnCombatant(c refdata.Combatant, roundNumber int32) refdata.Combatant {
	c.RageStartedRound = sql.NullInt32{Int32: roundNumber, Valid: true}
	return c
}

// ClearRageFromCombatant removes rage state from a combatant.
func ClearRageFromCombatant(c refdata.Combatant) refdata.Combatant {
	c.IsRaging = false
	c.RageRoundsRemaining = sql.NullInt32{Valid: false}
	c.RageAttackedThisRound = false
	c.RageTookDamageThisRound = false
	c.RageStartedRound = sql.NullInt32{Valid: false}
	return c
}

// RageHasGraceWindow reports whether the rage was started (or Bonus-Action
// extended) in the round whose turn is now ending. 2024 PHB: a Rage "lasts
// until the end of your NEXT turn", so the turn that started or extended it can
// never be the turn that ends it — regardless of activity. This is the fix for
// the live regression where a barbarian raged, swung once, and lost the rage at
// the end of that very turn.
func RageHasGraceWindow(c refdata.Combatant, roundNumber int32) bool {
	return c.RageStartedRound.Valid && c.RageStartedRound.Int32 == roundNumber
}

// ShouldRageEndOnTurnEnd checks if rage lapses at the end of the barbarian's
// turn in roundNumber. Under 2024 rules the rage is sustained by ANY of:
//   - the grace window (started or Bonus-Action extended this very turn)
//   - an attack roll made this round, hit or miss (RageAttackedThisRound, which
//     markRageForcedSave also sets for "you forced a save")
//   - damage taken since the barbarian's last turn (RageTookDamageThisRound)
//
// Nothing at all → the rage ends.
func ShouldRageEndOnTurnEnd(c refdata.Combatant, roundNumber int32) bool {
	if !c.IsRaging {
		return false
	}
	if RageHasGraceWindow(c, roundNumber) {
		return false
	}
	return !c.RageAttackedThisRound && !c.RageTookDamageThisRound
}

// ShouldRageEndOnTurnStart checks if rage should end because rounds have expired.
func ShouldRageEndOnTurnStart(c refdata.Combatant) bool {
	if !c.IsRaging {
		return false
	}
	return c.RageRoundsRemaining.Valid && c.RageRoundsRemaining.Int32 <= 0
}

// DecrementRageRound decrements the rage round counter and resets per-round tracking.
func DecrementRageRound(c refdata.Combatant) refdata.Combatant {
	if c.IsRaging && c.RageRoundsRemaining.Valid && c.RageRoundsRemaining.Int32 > 0 {
		c.RageRoundsRemaining.Int32--
	}
	c.RageAttackedThisRound = false
	c.RageTookDamageThisRound = false
	return c
}

// ShouldRageEndOnUnconscious checks if rage should end because the barbarian
// has fallen unconscious (HP = 0).
func ShouldRageEndOnUnconscious(c refdata.Combatant) bool {
	if !c.IsRaging {
		return false
	}
	return c.HpCurrent <= 0
}

// IsHeavyArmor checks if the given armor is heavy armor.
func IsHeavyArmor(armor refdata.Armor) bool {
	return armor.ArmorType == "heavy"
}

// RageCommand holds the service-level inputs for activating rage.
type RageCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
}

// RageResult holds the result of activating rage.
type RageResult struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	CombatLog string
	Remaining string
	RagesLeft int
}

// ActivateRage handles the /bonus rage command. An already-raging barbarian
// does not get an error: 2024 PHB lets you spend a Bonus Action to extend an
// active Rage, so /bonus rage doubles as the extend command (/bonus end-rage
// is still the way to stop raging).
func (s *Service) ActivateRage(ctx context.Context, cmd RageCommand) (RageResult, error) {
	if cmd.Combatant.IsRaging {
		return s.extendRage(ctx, cmd)
	}

	// Validate bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return RageResult{}, err
	}

	// Must be a PC
	if !cmd.Combatant.CharacterID.Valid {
		return RageResult{}, fmt.Errorf("rage requires a character (not NPC)")
	}

	// Get character
	char, err := s.store.GetCharacter(ctx, cmd.Combatant.CharacterID.UUID)
	if err != nil {
		return RageResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Must be barbarian
	if !HasBarbarianClass(char.Classes) {
		return RageResult{}, fmt.Errorf("Rage requires Barbarian class")
	}

	// Check armor — heavy armor blocks rage
	armorType := ""
	if char.EquippedArmor.Valid && char.EquippedArmor.String != "" {
		armor, err := s.store.GetArmor(ctx, char.EquippedArmor.String)
		if err == nil {
			armorType = armor.ArmorType
		}
	}

	// Validate rage preconditions
	if err := ValidateRageActivation(cmd.Combatant.IsRaging, armorType); err != nil {
		return RageResult{}, err
	}

	// Parse feature_uses and check rage remaining
	featureUses, ragesRemaining, err := ParseFeatureUses(char, FeatureKeyRage)
	if err != nil {
		return RageResult{}, err
	}

	barbLevel := ClassLevelFromJSON(char.Classes, "Barbarian")
	maxRages := RageUsesPerDay(barbLevel)

	// Unlimited rages at level 20
	if maxRages != -1 && ragesRemaining <= 0 {
		return RageResult{}, fmt.Errorf("no rage uses remaining (0/%d)", maxRages)
	}

	// Deduct rage use (unless unlimited)
	newRagesRemaining := ragesRemaining
	if maxRages != -1 {
		var deductErr error
		newRagesRemaining, deductErr = s.DeductFeatureUse(ctx, char, FeatureKeyRage, featureUses, ragesRemaining)
		if deductErr != nil {
			return RageResult{}, deductErr
		}
	}

	// Use bonus action
	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return RageResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	// Persist turn
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return RageResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Apply rage to combatant and persist
	ragedCombatant := ApplyRageToCombatant(cmd.Combatant, cmd.Turn.RoundNumber)
	ragedCombatant, err = s.persistRageState(ctx, ragedCombatant)
	if err != nil {
		return RageResult{}, fmt.Errorf("updating combatant rage: %w", err)
	}

	// D-46-rage-spellcasting-block — a barbarian who enters rage drops any
	// active concentration spell. BreakConcentrationFully clears the columns,
	// strips spell-sourced conditions, dismisses summons, and removes
	// concentration-tagged zones. Best-effort: errors here do NOT abort rage
	// activation (rage already persisted above).
	_, _ = s.breakStoredConcentration(ctx, ragedCombatant, "raging")

	combatLog := FormatRageActivation(cmd.Combatant.DisplayName, newRagesRemaining)
	remaining := FormatRemainingResources(updatedTurn, nil)

	return RageResult{
		Combatant: ragedCombatant,
		Turn:      updatedTurn,
		CombatLog: combatLog,
		Remaining: remaining,
		RagesLeft: newRagesRemaining,
	}, nil
}

// extendRage spends a Bonus Action to push an active Rage's horizon out to the
// end of the barbarian's next turn (2024 PHB sustain option c). It costs no
// daily rage use and does not touch the RageRounds cap — refreshing
// RageStartedRound to the current round re-arms the same grace window that
// activation uses, which is exactly "lasts until the end of your next turn".
func (s *Service) extendRage(ctx context.Context, cmd RageCommand) (RageResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return RageResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return RageResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return RageResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	extended, err := s.persistRageState(ctx, ExtendRageOnCombatant(cmd.Combatant, cmd.Turn.RoundNumber))
	if err != nil {
		return RageResult{}, fmt.Errorf("updating combatant rage: %w", err)
	}

	return RageResult{
		Combatant: extended,
		Turn:      updatedTurn,
		CombatLog: FormatRageExtend(cmd.Combatant.DisplayName),
		Remaining: FormatRemainingResources(updatedTurn, nil),
	}, nil
}

// markRageAttacked persists `RageAttackedThisRound = true` for a raging
// attacker so the sustain check at end-of-turn (see ShouldRageEndOnTurnEnd)
// does not fire prematurely. 2024 rules: an attack ROLL sustains the rage
// whether it hits or misses, which is why this is called from the attack
// pipeline unconditionally rather than on a hit.
//
// The live rage state is re-read rather than trusted from `attacker`: callers
// pass the combatant snapshot loaded at attack time, so a barbarian who attacks
// FIRST and rages afterwards on the same turn carries IsRaging=false in that
// snapshot and used to get no credit. Re-reading also avoids writing stale
// rage_rounds_remaining / rage_started_round values back over fresher ones.
//
// Best-effort: skip non-raging attackers and swallow errors so the attack
// pipeline is never blocked by rage state. (D-46)
func (s *Service) markRageAttacked(ctx context.Context, attacker refdata.Combatant) {
	live, err := s.store.GetCombatant(ctx, attacker.ID)
	if err != nil {
		return
	}
	if !live.IsRaging {
		return
	}
	live.RageAttackedThisRound = true
	_, _ = s.persistRageState(ctx, live)
}

// markRageForcedSave credits the 2024 sustain condition "you forced an enemy to
// make a saving throw". It shares RageAttackedThisRound with markRageAttacked:
// the column is really "did a rage-sustaining offensive action happen this
// round", and both answers are indistinguishable to ShouldRageEndOnTurnEnd, so
// a second column would buy nothing.
func (s *Service) markRageForcedSave(ctx context.Context, source refdata.Combatant) {
	s.markRageAttacked(ctx, source)
}

// markRageTookDamage persists `RageTookDamageThisRound = true` for a raging
// target that received positive damage this round. Mirrors markRageAttacked.
// (D-46)
func (s *Service) markRageTookDamage(ctx context.Context, target refdata.Combatant) {
	if !target.IsRaging {
		return
	}
	target.RageTookDamageThisRound = true
	_, _ = s.persistRageState(ctx, target)
}

// maybeEndRageOnUnconscious drops rage state when a raging combatant has
// fallen to 0 HP. Called by the damage pipeline after applying the dying
// condition bundle. Best-effort: lookup / persistence errors are swallowed
// because rage state is non-critical to the death-save flow. (D-46)
func (s *Service) maybeEndRageOnUnconscious(ctx context.Context, c refdata.Combatant) {
	if !ShouldRageEndOnUnconscious(c) {
		return
	}
	cleared := ClearRageFromCombatant(c)
	_, _ = s.persistRageState(ctx, cleared)
}

// maybeEndRageOnTurnEnd is the AdvanceTurn-end hook implementing the 2024 Rage
// clock for the turn that just ended in roundNumber. Either the rage lapses
// (nothing sustained it — see ShouldRageEndOnTurnEnd) or it rolls over: one
// round is burned off the RageRounds cap and the per-round activity flags are
// cleared, which opens the "since your last turn" damage window for the next
// round.
//
// The flags MUST be reset here and not at turn start, or damage the barbarian
// takes during the enemies' turns would be wiped before it could sustain the
// rage.
//
// Best-effort: lookup or persistence errors are swallowed so AdvanceTurn flow
// is never blocked by rage state. med-43 / Phase 46.
func (s *Service) maybeEndRageOnTurnEnd(ctx context.Context, combatantID uuid.UUID, roundNumber int32) {
	c, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil || !c.IsRaging {
		return
	}
	if ShouldRageEndOnTurnEnd(c, roundNumber) {
		cleared := ClearRageFromCombatant(c)
		_, _ = s.persistRageState(ctx, cleared)
		s.notifyRageEnded(ctx, c, rageEndReasonNoActivity)
		return
	}
	_, _ = s.persistRageState(ctx, DecrementRageRound(c))
}

// maybeEndRageOnRoundCap is the turn-start hook enforcing the 10-minute
// (RageRounds) hard cap via ShouldRageEndOnTurnStart. Best-effort, and gated on
// IsRaging so a non-raging combatant costs nothing.
func (s *Service) maybeEndRageOnRoundCap(ctx context.Context, c refdata.Combatant) {
	if !ShouldRageEndOnTurnStart(c) {
		return
	}
	cleared := ClearRageFromCombatant(c)
	_, _ = s.persistRageState(ctx, cleared)
	s.notifyRageEnded(ctx, c, rageEndReasonDurationExpired)
}

// maybeEndRageOnIncapacitated drops rage when the barbarian's turn is skipped
// for incapacitation (2024: a Rage ends early if you are Incapacitated).
// Best-effort, mirroring the other rage hooks.
func (s *Service) maybeEndRageOnIncapacitated(ctx context.Context, c refdata.Combatant) {
	if !c.IsRaging {
		return
	}
	cleared := ClearRageFromCombatant(c)
	_, _ = s.persistRageState(ctx, cleared)
	s.notifyRageEnded(ctx, c, rageEndReasonIncapacitated)
}

const (
	// rageEndReasonNoActivity is the reason shown when a rage lapses at end of
	// turn because nothing sustained it: no attack roll, no forced save, no
	// damage taken, and no Bonus Action extension.
	rageEndReasonNoActivity = "no attack, forced save, or damage since last turn"
	// rageEndReasonDurationExpired is the reason shown when the 10-minute
	// (RageRounds) hard cap runs out.
	rageEndReasonDurationExpired = "the Rage has burned out (10 minutes)"
	// rageEndReasonIncapacitated is the reason shown when the barbarian is
	// Incapacitated.
	rageEndReasonIncapacitated = "incapacitated"
)

// notifyRageEnded best-effort records a rage lapse (ISSUE-041) to
// both the encounter's #combat-log channel and the DM Console timeline
// (action_log), so a silently dropped rage is visible in lockstep with the turn
// that ended it — players were never told their rage fell off. Pure
// observability, mirroring notifyDroppedToZero: the action_log parent is the
// encounter's active turn (the rager's own turn, still current at the
// AdvanceTurn end-hook), and every error is swallowed so a logging miss never
// blocks turn flow.
func (s *Service) notifyRageEnded(ctx context.Context, c refdata.Combatant, reason string) {
	msg := FormatRageEnd(c.DisplayName, reason)
	s.postCombatLog(ctx, c.EncounterID, msg)

	enc, err := s.store.GetEncounter(ctx, c.EncounterID)
	if err != nil || !enc.CurrentTurnID.Valid {
		return
	}
	s.recordCombatAction(ctx, enc.CurrentTurnID.UUID, c.EncounterID, c.ID, uuid.NullUUID{}, actionTypeRageExpired, msg)
}

// EndRage handles the /bonus end-rage command.
func (s *Service) EndRage(ctx context.Context, cmd RageCommand) (RageResult, error) {
	if !cmd.Combatant.IsRaging {
		return RageResult{}, fmt.Errorf("not currently raging")
	}

	cleared := ClearRageFromCombatant(cmd.Combatant)
	cleared, err := s.persistRageState(ctx, cleared)
	if err != nil {
		return RageResult{}, fmt.Errorf("updating combatant rage: %w", err)
	}

	return RageResult{
		Combatant: cleared,
		CombatLog: FormatRageEndVoluntary(cmd.Combatant.DisplayName),
	}, nil
}

// persistRageState saves the combatant's rage fields to the database.
func (s *Service) persistRageState(ctx context.Context, c refdata.Combatant) (refdata.Combatant, error) {
	return s.store.UpdateCombatantRage(ctx, refdata.UpdateCombatantRageParams{
		ID:                      c.ID,
		IsRaging:                c.IsRaging,
		RageRoundsRemaining:     c.RageRoundsRemaining,
		RageAttackedThisRound:   c.RageAttackedThisRound,
		RageTookDamageThisRound: c.RageTookDamageThisRound,
		RageStartedRound:        c.RageStartedRound,
	})
}
