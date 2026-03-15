package check

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
)

// ConditionInfo carries condition data from a combatant for check processing.
type ConditionInfo struct {
	Conditions      json.RawMessage
	ExhaustionLevel int
}

// ParseConditions parses combat conditions from raw JSON.
func ParseConditions(raw json.RawMessage) ([]combat.CombatCondition, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var conds []combat.CombatCondition
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil, err
	}
	return conds, nil
}

// validAbilities are the six core abilities that can be used for raw ability checks.
var validAbilities = map[string]bool{
	"str": true, "dex": true, "con": true,
	"int": true, "wis": true, "cha": true,
}

// Service handles ability and skill check logic.
type Service struct {
	roller *dice.Roller
}

// NewService creates a new check Service.
func NewService(roller *dice.Roller) *Service {
	return &Service{roller: roller}
}

// SingleCheckInput holds parameters for a single ability/skill check.
type SingleCheckInput struct {
	Scores           character.AbilityScores
	Skill            string // skill name or ability abbreviation (e.g. "perception", "str")
	ProficientSkills []string
	ExpertiseSkills  []string
	JackOfAllTrades  bool
	ProfBonus        int
	RollMode         dice.RollMode
	Conditions       []combat.CombatCondition
	ConditionCtx     combat.AbilityCheckContext
	ExhaustionLevel  int
}

// SingleCheckResult holds the result of a single ability/skill check.
type SingleCheckResult struct {
	Skill            string
	Modifier         int
	Total            int
	AutoFail         bool
	ConditionReasons []string
	D20Result        dice.D20Result
}

// SingleCheck performs a single ability or skill check.
func (s *Service) SingleCheck(input SingleCheckInput) (SingleCheckResult, error) {
	skill := strings.ToLower(input.Skill)

	modifier, err := calculateModifier(input, skill)
	if err != nil {
		return SingleCheckResult{}, err
	}

	// Apply condition effects (including exhaustion)
	autoFail, condMode, reasons := combat.CheckAbilityCheckWithExhaustion(
		input.Conditions, input.ConditionCtx, input.ExhaustionLevel,
	)

	result := SingleCheckResult{
		Skill:            skill,
		Modifier:         modifier,
		AutoFail:         autoFail,
		ConditionReasons: reasons,
	}

	if autoFail {
		result.Total = 0
		return result, nil
	}

	// Combine requested roll mode with condition-imposed mode
	finalMode := dice.CombineRollModes(input.RollMode, condMode)

	d20, err := s.roller.RollD20(modifier, finalMode)
	if err != nil {
		return SingleCheckResult{}, fmt.Errorf("rolling d20: %w", err)
	}

	result.D20Result = d20
	result.Total = d20.Total
	return result, nil
}

// calculateModifier computes the total modifier for a check.
func calculateModifier(input SingleCheckInput, skill string) (int, error) {
	// Raw ability check
	if validAbilities[skill] {
		return character.AbilityModifier(input.Scores.Get(skill)), nil
	}

	// Skill check - must be in SkillAbilityMap
	if _, ok := character.SkillAbilityMap[skill]; !ok {
		return 0, fmt.Errorf("unknown skill or ability: %q", skill)
	}

	return character.SkillModifier(
		input.Scores, skill,
		input.ProficientSkills, input.ExpertiseSkills,
		input.JackOfAllTrades, input.ProfBonus,
	), nil
}

// PassiveCheckInput holds parameters for a passive check.
type PassiveCheckInput struct {
	Scores           character.AbilityScores
	Skill            string
	ProficientSkills []string
	ExpertiseSkills  []string
	JackOfAllTrades  bool
	ProfBonus        int
}

// PassiveCheckResult holds the result of a passive check.
type PassiveCheckResult struct {
	Skill    string
	Modifier int
	Total    int
}

// PassiveCheck calculates a passive check: 10 + modifier.
func (s *Service) PassiveCheck(input PassiveCheckInput) PassiveCheckResult {
	skill := strings.ToLower(input.Skill)
	modifier, err := calculateModifier(SingleCheckInput{
		Scores:           input.Scores,
		Skill:            skill,
		ProficientSkills: input.ProficientSkills,
		ExpertiseSkills:  input.ExpertiseSkills,
		JackOfAllTrades:  input.JackOfAllTrades,
		ProfBonus:        input.ProfBonus,
	}, skill)
	if err != nil {
		modifier = 0
	}
	return PassiveCheckResult{
		Skill:    skill,
		Modifier: modifier,
		Total:    10 + modifier,
	}
}

// GroupParticipant holds a participant in a group check.
type GroupParticipant struct {
	Name     string
	Modifier int
}

// GroupCheckInput holds parameters for a group check.
type GroupCheckInput struct {
	DC           int
	Participants []GroupParticipant
}

// GroupParticipantResult holds the result for one participant in a group check.
type GroupParticipantResult struct {
	Name    string
	D20     dice.D20Result
	Passed  bool
}

// GroupCheckResult holds the result of a group check.
type GroupCheckResult struct {
	DC           int
	Results      []GroupParticipantResult
	Passed       int
	Failed       int
	Success      bool
}

// GroupCheck performs a group check. Succeeds if at least half the participants pass.
func (s *Service) GroupCheck(input GroupCheckInput) GroupCheckResult {
	result := GroupCheckResult{DC: input.DC}

	if len(input.Participants) == 0 {
		return result
	}

	for _, p := range input.Participants {
		d20, _ := s.roller.RollD20(p.Modifier, dice.Normal)
		passed := d20.Total >= input.DC
		if passed {
			result.Passed++
		} else {
			result.Failed++
		}
		result.Results = append(result.Results, GroupParticipantResult{
			Name:   p.Name,
			D20:    d20,
			Passed: passed,
		})
	}

	result.Success = result.Passed*2 >= len(input.Participants)
	return result
}

// ContestedParticipant holds a participant in a contested check.
type ContestedParticipant struct {
	Name     string
	Modifier int
	RollMode dice.RollMode
}

// ContestedCheckInput holds parameters for a contested check.
type ContestedCheckInput struct {
	Initiator ContestedParticipant
	Opponent  ContestedParticipant
}

// ContestedCheckResult holds the result of a contested check.
type ContestedCheckResult struct {
	InitiatorD20   dice.D20Result
	OpponentD20    dice.D20Result
	InitiatorTotal int
	OpponentTotal  int
	Winner         string // name of winner, empty on tie
	Tie            bool
}

// ContestedCheck performs a contested check between two participants.
func (s *Service) ContestedCheck(input ContestedCheckInput) ContestedCheckResult {
	initD20, _ := s.roller.RollD20(input.Initiator.Modifier, input.Initiator.RollMode)
	oppD20, _ := s.roller.RollD20(input.Opponent.Modifier, input.Opponent.RollMode)

	result := ContestedCheckResult{
		InitiatorD20:   initD20,
		OpponentD20:    oppD20,
		InitiatorTotal: initD20.Total,
		OpponentTotal:  oppD20.Total,
	}

	if initD20.Total > oppD20.Total {
		result.Winner = input.Initiator.Name
	} else if oppD20.Total > initD20.Total {
		result.Winner = input.Opponent.Name
	} else {
		result.Tie = true
	}

	return result
}

