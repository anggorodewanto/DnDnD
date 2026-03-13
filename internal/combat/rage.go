package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/sqlc-dev/pqtype"

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

// RageRounds is the number of rounds rage lasts (1 minute = 10 rounds).
const RageRounds = 10

// ValidateRageActivation checks all preconditions for rage activation.
// Returns an error if any precondition fails.
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

// ApplyRageToCombatant sets rage state on a combatant.
func ApplyRageToCombatant(c refdata.Combatant) refdata.Combatant {
	c.IsRaging = true
	c.RageRoundsRemaining = sql.NullInt32{Int32: RageRounds, Valid: true}
	c.RageAttackedThisRound = false
	c.RageTookDamageThisRound = false
	return c
}

// ClearRageFromCombatant removes rage state from a combatant.
func ClearRageFromCombatant(c refdata.Combatant) refdata.Combatant {
	c.IsRaging = false
	c.RageRoundsRemaining = sql.NullInt32{Valid: false}
	c.RageAttackedThisRound = false
	c.RageTookDamageThisRound = false
	return c
}

// ShouldRageEndOnTurnEnd checks if rage should auto-end because the barbarian
// neither attacked nor took damage this round.
func ShouldRageEndOnTurnEnd(c refdata.Combatant) bool {
	if !c.IsRaging {
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
	if c.IsRaging && c.RageRoundsRemaining.Valid {
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

// ActivateRage handles the /bonus rage command.
func (s *Service) ActivateRage(ctx context.Context, cmd RageCommand) (RageResult, error) {
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
	featureUses, ragesRemaining, err := parseRageUses(char)
	if err != nil {
		return RageResult{}, err
	}

	barbLevel := barbarianLevel(char.Classes)
	maxRages := RageUsesPerDay(barbLevel)

	// Unlimited rages at level 20
	if maxRages != -1 && ragesRemaining <= 0 {
		return RageResult{}, fmt.Errorf("no rage uses remaining (0/%d)", maxRages)
	}

	// Deduct rage use (unless unlimited)
	newRagesRemaining := ragesRemaining
	if maxRages != -1 {
		newRagesRemaining = ragesRemaining - 1
		featureUses["rage"] = newRagesRemaining
		featureUsesJSON, err := json.Marshal(featureUses)
		if err != nil {
			return RageResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
		}
		if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
			ID:          char.ID,
			FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		}); err != nil {
			return RageResult{}, fmt.Errorf("updating feature_uses: %w", err)
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
	ragedCombatant := ApplyRageToCombatant(cmd.Combatant)
	ragedCombatant, err = s.persistRageState(ctx, ragedCombatant)
	if err != nil {
		return RageResult{}, fmt.Errorf("updating combatant rage: %w", err)
	}

	combatLog := FormatRageActivation(cmd.Combatant.DisplayName, newRagesRemaining)
	remaining := FormatRemainingResources(updatedTurn)

	return RageResult{
		Combatant: ragedCombatant,
		Turn:      updatedTurn,
		CombatLog: combatLog,
		Remaining: remaining,
		RagesLeft: newRagesRemaining,
	}, nil
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
	})
}

// parseRageUses extracts rage uses from character feature_uses JSON.
func parseRageUses(char refdata.Character) (map[string]int, int, error) {
	featureUses := make(map[string]int)
	if char.FeatureUses.Valid && len(char.FeatureUses.RawMessage) > 0 {
		if err := json.Unmarshal(char.FeatureUses.RawMessage, &featureUses); err != nil {
			return nil, 0, fmt.Errorf("parsing feature_uses: %w", err)
		}
	}
	ragesRemaining, _ := featureUses["rage"]
	return featureUses, ragesRemaining, nil
}

// barbarianLevel returns the barbarian level from character classes JSON.
func barbarianLevel(classesJSON json.RawMessage) int {
	if len(classesJSON) == 0 {
		return 0
	}
	var classes []CharacterClass
	if err := json.Unmarshal(classesJSON, &classes); err != nil {
		return 0
	}
	return classLevel(classes, "Barbarian")
}
