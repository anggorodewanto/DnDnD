package save

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
)

// validAbilities are the six core abilities for saving throws.
var validAbilities = map[string]bool{
	"str": true, "dex": true, "con": true,
	"int": true, "wis": true, "cha": true,
}

// Service handles saving throw logic.
type Service struct {
	roller *dice.Roller
}

// NewService creates a new save Service.
func NewService(roller *dice.Roller) *Service {
	return &Service{roller: roller}
}

// SaveInput holds parameters for a saving throw.
type SaveInput struct {
	Scores          character.AbilityScores
	Ability         string
	ProficientSaves []string
	ProfBonus       int
	RollMode        dice.RollMode
	Conditions      []combat.CombatCondition
	ExhaustionLevel int
	FeatureEffects  []combat.FeatureDefinition
	EffectCtx       combat.EffectContext
	// BonusDice, when non-empty, is a player-declared effect-die expression
	// (e.g. "1d4" Bless, "1d8" Bardic Inspiration) added to the save total.
	// Save validates + rolls it; an invalid expression returns
	// dice.ErrInvalidBonus. It is not rolled when the save auto-fails.
	BonusDice string
}

// SaveResult holds the result of a saving throw.
type SaveResult struct {
	Ability          string
	Modifier         int
	FeatureBonus     int
	Total            int
	AutoFail         bool
	ConditionReasons []string
	FeatureReasons   []string
	D20Result        dice.D20Result
	// Effect-dice bonus (Bless, Bardic Inspiration, ...). Zero-valued when no
	// BonusDice supplied. BonusTotal is folded into Total; D20Result stays
	// bonus-free. BonusRolls carries the per-die results for roll logging.
	BonusExpression string
	BonusTotal      int
	BonusRolls      []dice.GroupResult
}

// Save performs a saving throw.
func (s *Service) Save(input SaveInput) (SaveResult, error) {
	ability := strings.ToLower(input.Ability)

	if !validAbilities[ability] {
		return SaveResult{}, fmt.Errorf("unknown ability: %q", ability)
	}

	// Validate the optional effect-die expression up front so a typo is
	// reported even when the save would auto-fail (it isn't rolled below).
	if input.BonusDice != "" {
		if err := dice.ValidateBonusExpression(input.BonusDice); err != nil {
			return SaveResult{}, err
		}
	}

	modifier := character.SavingThrowModifier(input.Scores, ability, input.ProficientSaves, input.ProfBonus)

	// Check condition effects (including exhaustion)
	autoFail, condMode, exhaustionPenalty, reasons := combat.CheckSaveWithExhaustion(input.Conditions, ability, input.ExhaustionLevel)

	result := SaveResult{
		Ability:          ability,
		Modifier:         modifier,
		AutoFail:         autoFail,
		ConditionReasons: reasons,
	}

	if autoFail {
		result.Total = 0
		return result, nil
	}

	// Process feature effects
	var featureMode dice.RollMode
	if len(input.FeatureEffects) > 0 {
		pr := combat.ProcessEffects(input.FeatureEffects, combat.TriggerOnSave, input.EffectCtx)
		result.FeatureBonus = pr.FlatModifier
		featureMode = pr.RollMode
		for _, ae := range pr.AppliedEffects {
			// A zero flat-modifier save effect contributes nothing to the roll —
			// e.g. the Evasion marker (EffectModifySave{Modifier:0}), whose real
			// mechanic is the post-save damage upgrade applied in
			// combat.ResolveAoESaves (COV-3), not a d20 bonus. Emitting
			// "Evasion: +0" here is noise, so skip it. Advantage-granting effects
			// are EffectConditionalAdvantage (not EffectModifySave) and are
			// unaffected.
			if ae.Effect.Type == combat.EffectModifySave && ae.Effect.Modifier == 0 {
				continue
			}
			result.FeatureReasons = append(result.FeatureReasons, fmt.Sprintf("%s: +%d", ae.FeatureName, ae.Effect.Modifier))
		}
	}

	// Combine all roll modes
	finalMode := dice.CombineRollModes(input.RollMode, condMode)
	finalMode = dice.CombineRollModes(finalMode, featureMode)

	// 2024 exhaustion applies a flat -2/level penalty to the saving throw.
	totalModifier := modifier + result.FeatureBonus + exhaustionPenalty

	d20, err := s.roller.RollD20(totalModifier, finalMode)
	if err != nil {
		return SaveResult{}, fmt.Errorf("rolling d20: %w", err)
	}

	result.D20Result = d20
	result.Total = d20.Total

	// Add player-declared effect dice (already validated above). The d20
	// breakdown stays bonus-free; only the grand Total folds the bonus in.
	if input.BonusDice != "" {
		bonus, berr := s.roller.Roll(input.BonusDice)
		if berr != nil {
			return SaveResult{}, fmt.Errorf("%w: %v", dice.ErrInvalidBonus, berr)
		}
		result.BonusExpression = input.BonusDice
		result.BonusTotal = bonus.Total
		result.BonusRolls = bonus.Groups
		result.Total += bonus.Total
	}

	return result, nil
}
