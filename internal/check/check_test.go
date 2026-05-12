package check

import (
	"errors"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
)

func fixedRoller(val int) *dice.Roller {
	return dice.NewRoller(func(max int) int { return val })
}

func TestSingleCheck_ProficientSkill(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:          character.AbilityScores{WIS: 16}, // +3 mod
		Skill:           "perception",
		ProficientSkills: []string{"perception"},
		ProfBonus:       2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, WIS mod=3, prof=2 => total=15
	if result.Total != 15 {
		t.Errorf("expected total 15, got %d", result.Total)
	}
	if result.Modifier != 5 {
		t.Errorf("expected modifier 5, got %d", result.Modifier)
	}
	if result.Skill != "perception" {
		t.Errorf("expected skill perception, got %s", result.Skill)
	}
}

func TestSingleCheck_NonProficientSkill(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 16}, // +3 mod
		Skill:  "perception",
		// no proficiency
		ProfBonus: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, WIS mod=3, no prof => total=13
	if result.Total != 13 {
		t.Errorf("expected total 13, got %d", result.Total)
	}
}

func TestSingleCheck_Expertise(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:           character.AbilityScores{DEX: 14}, // +2 mod
		Skill:            "stealth",
		ProficientSkills: []string{"stealth"},
		ExpertiseSkills:  []string{"stealth"},
		ProfBonus:        3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, DEX mod=2, expertise=3*2=6 => total=18
	if result.Total != 18 {
		t.Errorf("expected total 18, got %d", result.Total)
	}
}

func TestSingleCheck_JackOfAllTrades(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:          character.AbilityScores{CHA: 12}, // +1 mod
		Skill:           "persuasion",
		JackOfAllTrades: true,
		ProfBonus:       2,
		// not proficient in persuasion
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, CHA mod=1, JoAT=2/2=1 => total=12
	if result.Total != 12 {
		t.Errorf("expected total 12, got %d", result.Total)
	}
}

func TestSingleCheck_RawAbilityCheck(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:    character.AbilityScores{STR: 18}, // +4 mod
		Skill:     "str",
		ProfBonus: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, STR mod=4 => total=14 (raw ability, no proficiency applies)
	if result.Total != 14 {
		t.Errorf("expected total 14, got %d", result.Total)
	}
	if result.Skill != "str" {
		t.Errorf("expected skill str, got %s", result.Skill)
	}
}

func TestSingleCheck_Advantage(t *testing.T) {
	// With advantage, roller returns 15 both times, takes higher
	svc := NewService(fixedRoller(15))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:   character.AbilityScores{WIS: 10},
		Skill:    "perception",
		RollMode: dice.Advantage,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Advantage {
		t.Errorf("expected advantage mode, got %v", result.D20Result.Mode)
	}
}

func TestSingleCheck_ConditionDisadvantage(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "perception",
		Conditions: []combat.CombatCondition{
			{Condition: "poisoned"},
		},
		ConditionCtx: combat.AbilityCheckContext{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Disadvantage {
		t.Errorf("expected disadvantage mode, got %v", result.D20Result.Mode)
	}
	if len(result.ConditionReasons) == 0 {
		t.Error("expected condition reasons")
	}
}

func TestSingleCheck_ConditionAutoFail(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "perception",
		Conditions: []combat.CombatCondition{
			{Condition: "blinded"},
		},
		ConditionCtx: combat.AbilityCheckContext{RequiresSight: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for blinded perception requiring sight")
	}
}

func TestSingleCheck_ExhaustionDisadvantage(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:          character.AbilityScores{STR: 10},
		Skill:           "athletics",
		ExhaustionLevel: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Disadvantage {
		t.Errorf("expected disadvantage from exhaustion, got %v", result.D20Result.Mode)
	}
}

func TestPassiveCheck_Proficient(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.PassiveCheck(PassiveCheckInput{
		Scores:           character.AbilityScores{WIS: 16}, // +3
		Skill:            "perception",
		ProficientSkills: []string{"perception"},
		ProfBonus:        2,
	})
	// 10 + 3 + 2 = 15
	if result.Total != 15 {
		t.Errorf("expected passive 15, got %d", result.Total)
	}
}

func TestPassiveCheck_NonProficient(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.PassiveCheck(PassiveCheckInput{
		Scores: character.AbilityScores{WIS: 10}, // +0
		Skill:  "perception",
	})
	// 10 + 0 = 10
	if result.Total != 10 {
		t.Errorf("expected passive 10, got %d", result.Total)
	}
}

func TestPassiveCheck_WithExpertise(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.PassiveCheck(PassiveCheckInput{
		Scores:           character.AbilityScores{WIS: 14}, // +2
		Skill:            "perception",
		ProficientSkills: []string{"perception"},
		ExpertiseSkills:  []string{"perception"},
		ProfBonus:        3,
	})
	// 10 + 2 + 6 = 18
	if result.Total != 18 {
		t.Errorf("expected passive 18, got %d", result.Total)
	}
}

func TestPassiveCheck_RawAbility(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.PassiveCheck(PassiveCheckInput{
		Scores: character.AbilityScores{STR: 18}, // +4
		Skill:  "str",
	})
	// 10 + 4 = 14
	if result.Total != 14 {
		t.Errorf("expected passive 14, got %d", result.Total)
	}
}

func TestGroupCheck_AllPass(t *testing.T) {
	svc := NewService(fixedRoller(15))

	result := svc.GroupCheck(GroupCheckInput{
		DC: 10,
		Participants: []GroupParticipant{
			{Name: "Aria", Modifier: 3},
			{Name: "Bob", Modifier: 1},
		},
	})
	if !result.Success {
		t.Error("expected group check to succeed when all pass")
	}
	if result.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", result.Passed)
	}
}

func TestGroupCheck_HalfPass(t *testing.T) {
	// Two participants, one passes one fails with DC 20
	roller := dice.NewRoller(func() func(int) int {
		calls := 0
		return func(max int) int {
			calls++
			if calls == 1 {
				return 18 // first participant: 18+3=21 >= 20
			}
			return 2 // second participant: 2+1=3 < 20
		}
	}())

	svc := NewService(roller)
	result := svc.GroupCheck(GroupCheckInput{
		DC: 20,
		Participants: []GroupParticipant{
			{Name: "Aria", Modifier: 3},
			{Name: "Bob", Modifier: 1},
		},
	})
	// 1 out of 2 passes = half, which counts as success
	if !result.Success {
		t.Error("expected group check to succeed when half pass")
	}
}

func TestGroupCheck_MajorityFail(t *testing.T) {
	roller := dice.NewRoller(func() func(int) int {
		calls := 0
		return func(max int) int {
			calls++
			if calls == 1 {
				return 18 // passes
			}
			return 2 // fails
		}
	}())

	svc := NewService(roller)
	result := svc.GroupCheck(GroupCheckInput{
		DC: 20,
		Participants: []GroupParticipant{
			{Name: "Aria", Modifier: 3},
			{Name: "Bob", Modifier: 1},
			{Name: "Cal", Modifier: 0},
		},
	})
	// 1 out of 3 passes < half, fails
	if result.Success {
		t.Error("expected group check to fail when majority fail")
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", result.Passed)
	}
}

func TestGroupCheck_Empty(t *testing.T) {
	svc := NewService(fixedRoller(10))
	result := svc.GroupCheck(GroupCheckInput{DC: 10})
	if result.Success {
		t.Error("expected empty group check to fail")
	}
}

func TestContestedCheck_InitiatorWins(t *testing.T) {
	svc := NewService(fixedRoller(15))

	result := svc.ContestedCheck(ContestedCheckInput{
		Initiator: ContestedParticipant{Name: "Aria", Modifier: 5},
		Opponent:  ContestedParticipant{Name: "Goblin", Modifier: 2},
	})
	// Both roll 15. Aria: 15+5=20, Goblin: 15+2=17. Aria wins.
	if result.Winner != "Aria" {
		t.Errorf("expected Aria to win, got %s", result.Winner)
	}
	if result.InitiatorTotal != 20 {
		t.Errorf("expected initiator total 20, got %d", result.InitiatorTotal)
	}
	if result.OpponentTotal != 17 {
		t.Errorf("expected opponent total 17, got %d", result.OpponentTotal)
	}
}

func TestContestedCheck_OpponentWins(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.ContestedCheck(ContestedCheckInput{
		Initiator: ContestedParticipant{Name: "Aria", Modifier: 1},
		Opponent:  ContestedParticipant{Name: "Goblin", Modifier: 5},
	})
	// Both roll 10. Aria: 11, Goblin: 15. Goblin wins.
	if result.Winner != "Goblin" {
		t.Errorf("expected Goblin to win, got %s", result.Winner)
	}
}

func TestContestedCheck_Tie(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.ContestedCheck(ContestedCheckInput{
		Initiator: ContestedParticipant{Name: "Aria", Modifier: 3},
		Opponent:  ContestedParticipant{Name: "Goblin", Modifier: 3},
	})
	// Tied: both 13. Tie goes to no winner (status quo / initiator fails).
	if result.Winner != "" {
		t.Errorf("expected tie (empty winner), got %s", result.Winner)
	}
	if !result.Tie {
		t.Error("expected Tie to be true")
	}
}

func TestFormatSingleCheckResult(t *testing.T) {
	result := SingleCheckResult{
		Skill:    "perception",
		Modifier: 5,
		Total:    15,
		D20Result: dice.D20Result{
			Rolls:   []int{10},
			Chosen:  10,
			Total:   15,
			Mode:    dice.Normal,
		},
	}
	msg := FormatSingleCheckResult("Aria", result)
	if !strings.Contains(msg, "Perception") {
		t.Errorf("expected Perception in message, got: %s", msg)
	}
	if !strings.Contains(msg, "15") {
		t.Errorf("expected total 15 in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Aria") {
		t.Errorf("expected Aria in message, got: %s", msg)
	}
}

func TestFormatSingleCheckResult_AutoFail(t *testing.T) {
	result := SingleCheckResult{
		Skill:            "perception",
		AutoFail:         true,
		ConditionReasons: []string{"blinded: auto-fail (requires sight)"},
	}
	msg := FormatSingleCheckResult("Aria", result)
	if !strings.Contains(msg, "Auto-fail") {
		t.Errorf("expected Auto-fail in message, got: %s", msg)
	}
	if !strings.Contains(msg, "blinded") {
		t.Errorf("expected blinded reason in message, got: %s", msg)
	}
}

func TestFormatSingleCheckResult_WithConditionReasons(t *testing.T) {
	result := SingleCheckResult{
		Skill:    "athletics",
		Modifier: 3,
		Total:    13,
		D20Result: dice.D20Result{
			Rolls:  []int{10, 8},
			Chosen: 8,
			Total:  13,
			Mode:   dice.Disadvantage,
		},
		ConditionReasons: []string{"poisoned: disadvantage on ability checks"},
	}
	msg := FormatSingleCheckResult("Aria", result)
	if !strings.Contains(msg, "poisoned") {
		t.Errorf("expected poisoned reason in message, got: %s", msg)
	}
	if !strings.Contains(msg, "disadvantage") {
		t.Errorf("expected disadvantage in message, got: %s", msg)
	}
}

func TestFormatGroupCheckResult(t *testing.T) {
	result := GroupCheckResult{
		DC:      15,
		Passed:  2,
		Failed:  1,
		Success: true,
		Results: []GroupParticipantResult{
			{Name: "Aria", D20: dice.D20Result{Total: 18}, Passed: true},
			{Name: "Bob", D20: dice.D20Result{Total: 16}, Passed: true},
			{Name: "Cal", D20: dice.D20Result{Total: 10}, Passed: false},
		},
	}
	msg := FormatGroupCheckResult("stealth", result)
	if !strings.Contains(msg, "SUCCESS") {
		t.Errorf("expected SUCCESS in message, got: %s", msg)
	}
	if !strings.Contains(msg, "2/3") {
		t.Errorf("expected 2/3 in message, got: %s", msg)
	}
}

func TestFormatContestedCheckResult(t *testing.T) {
	result := ContestedCheckResult{
		InitiatorTotal: 20,
		OpponentTotal:  17,
		Winner:         "Aria",
	}
	msg := FormatContestedCheckResult("athletics", "Aria", "Goblin", result)
	if !strings.Contains(msg, "Aria wins") {
		t.Errorf("expected 'Aria wins' in message, got: %s", msg)
	}
}

func TestFormatContestedCheckResult_Tie(t *testing.T) {
	result := ContestedCheckResult{
		InitiatorTotal: 15,
		OpponentTotal:  15,
		Tie:            true,
	}
	msg := FormatContestedCheckResult("athletics", "Aria", "Goblin", result)
	if !strings.Contains(msg, "Tie") {
		t.Errorf("expected 'Tie' in message, got: %s", msg)
	}
}

func TestSingleCheck_InvalidSkill(t *testing.T) {
	svc := NewService(fixedRoller(10))

	_, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{},
		Skill:  "invalid-skill",
	})
	if err == nil {
		t.Error("expected error for invalid skill")
	}
}

func TestSingleCheck_AdvPlusConditionDisadv_Cancel(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:   character.AbilityScores{WIS: 10},
		Skill:    "perception",
		RollMode: dice.Advantage,
		Conditions: []combat.CombatCondition{
			{Condition: "poisoned"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Advantage (player) + Disadvantage (condition) = cancel out
	if result.D20Result.Mode != dice.Normal {
		t.Errorf("expected normal (cancelled), got %v", result.D20Result.Mode)
	}
}

func TestSingleCheck_DeafenedAutoFail(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "perception",
		Conditions: []combat.CombatCondition{
			{Condition: "deafened"},
		},
		ConditionCtx: combat.AbilityCheckContext{RequiresHearing: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for deafened check requiring hearing")
	}
}

func TestSingleCheck_FrightenedWithFearSource(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{STR: 10},
		Skill:  "athletics",
		Conditions: []combat.CombatCondition{
			{Condition: "frightened"},
		},
		ConditionCtx: combat.AbilityCheckContext{FearSourceVisible: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Disadvantage {
		t.Errorf("expected disadvantage from frightened, got %v", result.D20Result.Mode)
	}
}

func TestSingleCheck_FrightenedWithoutFearSource(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{STR: 10},
		Skill:  "athletics",
		Conditions: []combat.CombatCondition{
			{Condition: "frightened"},
		},
		ConditionCtx: combat.AbilityCheckContext{FearSourceVisible: false},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fear source not visible => no disadvantage
	if result.D20Result.Mode != dice.Normal {
		t.Errorf("expected normal (fear source not visible), got %v", result.D20Result.Mode)
	}
}

func TestCombineRollModes(t *testing.T) {
	tests := []struct {
		name      string
		requested dice.RollMode
		condition dice.RollMode
		expected  dice.RollMode
	}{
		{"normal+normal", dice.Normal, dice.Normal, dice.Normal},
		{"adv+normal", dice.Advantage, dice.Normal, dice.Advantage},
		{"normal+disadv", dice.Normal, dice.Disadvantage, dice.Disadvantage},
		{"adv+disadv", dice.Advantage, dice.Disadvantage, dice.AdvantageAndDisadvantage},
		{"disadv+disadv", dice.Disadvantage, dice.Disadvantage, dice.Disadvantage},
		{"adv+adv", dice.Advantage, dice.Advantage, dice.Advantage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dice.CombineRollModes(tc.requested, tc.condition)
			if got != tc.expected {
				t.Errorf("CombineRollModes(%v, %v) = %v, want %v", tc.requested, tc.condition, got, tc.expected)
			}
		})
	}
}

func TestPassiveCheck_InvalidSkill(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result := svc.PassiveCheck(PassiveCheckInput{
		Scores: character.AbilityScores{},
		Skill:  "bogus",
	})
	// Should default to 0 modifier
	if result.Total != 10 {
		t.Errorf("expected passive 10 for invalid skill, got %d", result.Total)
	}
}

func TestParseConditions(t *testing.T) {
	raw := []byte(`[{"condition":"poisoned"},{"condition":"blinded"}]`)
	conds, err := ParseConditions(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conds) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(conds))
	}
}

func TestParseConditions_Empty(t *testing.T) {
	conds, err := ParseConditions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conds != nil {
		t.Errorf("expected nil for empty, got %v", conds)
	}
}

func TestParseConditions_Invalid(t *testing.T) {
	_, err := ParseConditions([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFormatSingleCheckResult_RawAbility(t *testing.T) {
	result := SingleCheckResult{
		Skill:    "dex",
		Modifier: 2,
		Total:    14,
		D20Result: dice.D20Result{
			Rolls:     []int{12},
			Chosen:    12,
			Total:     14,
			Breakdown: "12 + 2 = 14",
		},
	}
	msg := FormatSingleCheckResult("Aria", result)
	if !strings.Contains(msg, "Dex") {
		t.Errorf("expected Dex in message, got: %s", msg)
	}
}

// --- F-15: TargetContext enforcement at the service layer ---

func TestSingleCheck_TargetContextNil_LegacyBehaviorUnchanged(t *testing.T) {
	svc := NewService(fixedRoller(10))
	result, err := svc.SingleCheck(SingleCheckInput{
		Scores:    character.AbilityScores{WIS: 16},
		Skill:     "perception",
		ProfBonus: 2,
		Target:    nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 13 {
		t.Errorf("expected total 13, got %d", result.Total)
	}
}

func TestSingleCheck_TargetContext_Adjacent_Passes(t *testing.T) {
	svc := NewService(fixedRoller(10))
	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "medicine",
		Target: &TargetContext{
			AttackerPosition: [3]int{1, 1, 0},
			TargetPosition:   [3]int{1, 2, 0},
			InCombat:         true,
			ActionAvailable:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error for adjacent target: %v", err)
	}
	if result.Total != 10 {
		t.Errorf("expected total 10, got %d", result.Total)
	}
}

func TestSingleCheck_TargetContext_OutOfReach_ReturnsErrTargetNotInReach(t *testing.T) {
	svc := NewService(fixedRoller(10))
	_, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "medicine",
		Target: &TargetContext{
			AttackerPosition: [3]int{0, 0, 0},
			TargetPosition:   [3]int{5, 5, 0},
			InCombat:         true,
			ActionAvailable:  true,
		},
	})
	if !errors.Is(err, ErrTargetNotInReach) {
		t.Errorf("expected ErrTargetNotInReach, got %v", err)
	}
}

func TestSingleCheck_TargetContext_NoActionAvailable_ReturnsErr(t *testing.T) {
	svc := NewService(fixedRoller(10))
	_, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "medicine",
		Target: &TargetContext{
			AttackerPosition: [3]int{2, 2, 0},
			TargetPosition:   [3]int{2, 3, 0},
			InCombat:         true,
			ActionAvailable:  false,
		},
	})
	if !errors.Is(err, ErrNoActionAvailable) {
		t.Errorf("expected ErrNoActionAvailable, got %v", err)
	}
}

func TestSingleCheck_TargetContext_OutOfCombat_IgnoresActionFlag(t *testing.T) {
	svc := NewService(fixedRoller(10))
	// Out of combat: ActionAvailable=false must not block the check.
	result, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{WIS: 10},
		Skill:  "medicine",
		Target: &TargetContext{
			AttackerPosition: [3]int{0, 0, 0},
			TargetPosition:   [3]int{1, 0, 0},
			InCombat:         false,
			ActionAvailable:  false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error out of combat: %v", err)
	}
	if result.Total != 10 {
		t.Errorf("expected total 10, got %d", result.Total)
	}
}

func TestSingleCheck_TargetContext_ReachVariants(t *testing.T) {
	svc := NewService(fixedRoller(10))
	// ProficientReach=0 should default to 1; (0,0)->(1,1) is Chebyshev 1.
	if _, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{}, Skill: "str",
		Target: &TargetContext{AttackerPosition: [3]int{0, 0, 0}, TargetPosition: [3]int{1, 1, 0}, ActionAvailable: true},
	}); err != nil {
		t.Errorf("default reach 1 should allow diagonal adjacency, got %v", err)
	}
	// Reach 2 lets the attacker hit a target 2 tiles away (e.g. polearm).
	if _, err := svc.SingleCheck(SingleCheckInput{
		Scores: character.AbilityScores{}, Skill: "str",
		Target: &TargetContext{AttackerPosition: [3]int{0, 0, 0}, TargetPosition: [3]int{2, 0, 0}, ProficientReach: 2, ActionAvailable: true},
	}); err != nil {
		t.Errorf("reach=2 should allow distance 2, got %v", err)
	}
}

func TestFormatGroupCheckResult_Failure(t *testing.T) {
	result := GroupCheckResult{
		DC:      20,
		Passed:  0,
		Failed:  2,
		Success: false,
		Results: []GroupParticipantResult{
			{Name: "A", D20: dice.D20Result{Total: 5}, Passed: false},
			{Name: "B", D20: dice.D20Result{Total: 8}, Passed: false},
		},
	}
	msg := FormatGroupCheckResult("stealth", result)
	if !strings.Contains(msg, "FAILURE") {
		t.Errorf("expected FAILURE, got: %s", msg)
	}
}
