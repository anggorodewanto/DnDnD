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
}

// Save performs a saving throw.
func (s *Service) Save(input SaveInput) (SaveResult, error) {
	ability := strings.ToLower(input.Ability)

	if !validAbilities[ability] {
		return SaveResult{}, fmt.Errorf("unknown ability: %q", ability)
	}

	modifier := character.SavingThrowModifier(input.Scores, ability, input.ProficientSaves, input.ProfBonus)

	// Check condition effects (including exhaustion)
	autoFail, condMode, reasons := combat.CheckSaveWithExhaustion(input.Conditions, ability, input.ExhaustionLevel)

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
			result.FeatureReasons = append(result.FeatureReasons, fmt.Sprintf("%s: +%d", ae.FeatureName, ae.Effect.Modifier))
		}
	}

	// Combine all roll modes
	finalMode := dice.CombineRollModes(input.RollMode, condMode)
	finalMode = dice.CombineRollModes(finalMode, featureMode)

	totalModifier := modifier + result.FeatureBonus

	d20, err := s.roller.RollD20(totalModifier, finalMode)
	if err != nil {
		return SaveResult{}, fmt.Errorf("rolling d20: %w", err)
	}

	result.D20Result = d20
	result.Total = d20.Total
	return result, nil
}
