package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
)

// F-15: errors surfaced when the optional TargetContext on SingleCheckInput
// fails preconditions. Callers (Discord handler today, dashboard-prompted
// path tomorrow) should map these to player-facing messages. Keeping the
// errors typed lets non-Discord entry paths enforce the same rules without
// reimplementing the validation logic.
var (
	// ErrTargetNotInReach is returned when the attacker/target Chebyshev
	// distance exceeds TargetContext.ProficientReach.
	ErrTargetNotInReach = errors.New("target not in reach")
	// ErrNoActionAvailable is returned when TargetContext.InCombat is true
	// but ActionAvailable is false (the caster has already used their
	// Action this round).
	ErrNoActionAvailable = errors.New("no action available")
)

// TargetContext, when attached to SingleCheckInput, enables service-layer
// adjacency + action-cost enforcement so non-Discord entry paths can't
// bypass the rules. (F-15 / Phase 81 finding)
//
// AttackerPosition / TargetPosition are caller-defined coordinates in tiles
// (typically [col, row, 0]). Chebyshev distance is used so a 5ft tile maps
// to ProficientReach=1. When ProficientReach is 0 it defaults to 1.
//
// When InCombat is true the caster must have ActionAvailable=true or
// ErrNoActionAvailable is returned. When InCombat is false the
// ActionAvailable flag is ignored (out-of-combat checks are free).
type TargetContext struct {
	AttackerPosition [3]int
	TargetPosition   [3]int
	InCombat         bool
	ActionAvailable  bool
	ProficientReach  int
}

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
	// Target is optional: when nil, SingleCheck preserves legacy behavior.
	// When set, SingleCheck enforces adjacency and (when InCombat)
	// action-availability before rolling. (F-15)
	Target *TargetContext
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
	if err := validateTargetContext(input.Target); err != nil {
		return SingleCheckResult{}, err
	}

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

// validateTargetContext enforces adjacency + action-cost preconditions when
// a TargetContext is attached to the input. Returns nil when ctx is nil so
// legacy callers are unaffected. (F-15)
func validateTargetContext(ctx *TargetContext) error {
	if ctx == nil {
		return nil
	}
	reach := ctx.ProficientReach
	if reach <= 0 {
		reach = 1
	}
	if chebyshev3(ctx.AttackerPosition, ctx.TargetPosition) > reach {
		return ErrTargetNotInReach
	}
	if ctx.InCombat && !ctx.ActionAvailable {
		return ErrNoActionAvailable
	}
	return nil
}

// chebyshev3 returns the Chebyshev (chessboard) distance between two 3D
// tile positions: the max axis delta. For a flat grid pass z=0 on both.
func chebyshev3(a, b [3]int) int {
	d := absInt(a[0] - b[0])
	if v := absInt(a[1] - b[1]); v > d {
		d = v
	}
	if v := absInt(a[2] - b[2]); v > d {
		d = v
	}
	return d
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
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

