package character

import "testing"

func TestSavingThrowModifier_Proficient(t *testing.T) {
	scores := AbilityScores{STR: 16} // +3
	profSaves := []string{"str", "con"}
	got := SavingThrowModifier(scores, "str", profSaves, 2)
	if got != 5 { // 3 + 2
		t.Errorf("SavingThrowModifier proficient = %d, want 5", got)
	}
}

func TestSavingThrowModifier_NotProficient(t *testing.T) {
	scores := AbilityScores{DEX: 14} // +2
	profSaves := []string{"str", "con"}
	got := SavingThrowModifier(scores, "dex", profSaves, 3)
	if got != 2 { // just ability mod
		t.Errorf("SavingThrowModifier not proficient = %d, want 2", got)
	}
}

func TestSkillModifier_NotProficient(t *testing.T) {
	scores := AbilityScores{STR: 14}
	got := SkillModifier(scores, "athletics", nil, nil, false, 2)
	if got != 2 { // STR mod only
		t.Errorf("SkillModifier not proficient = %d, want 2", got)
	}
}

func TestSkillModifier_Proficient(t *testing.T) {
	scores := AbilityScores{STR: 14}
	profSkills := []string{"athletics"}
	got := SkillModifier(scores, "athletics", profSkills, nil, false, 3)
	if got != 5 { // 2 + 3
		t.Errorf("SkillModifier proficient = %d, want 5", got)
	}
}

func TestSkillModifier_Expertise(t *testing.T) {
	scores := AbilityScores{DEX: 16}
	profSkills := []string{"stealth"}
	expertiseSkills := []string{"stealth"}
	got := SkillModifier(scores, "stealth", profSkills, expertiseSkills, false, 2)
	if got != 7 { // 3 + 2*2
		t.Errorf("SkillModifier expertise = %d, want 7", got)
	}
}

func TestSkillModifier_JackOfAllTrades_NotProficient(t *testing.T) {
	scores := AbilityScores{INT: 10}
	got := SkillModifier(scores, "arcana", nil, nil, true, 3)
	// 0 + floor(3/2) = 1
	if got != 1 {
		t.Errorf("SkillModifier jack of all trades = %d, want 1", got)
	}
}

func TestSkillModifier_JackOfAllTrades_AlreadyProficient(t *testing.T) {
	// Jack of All Trades doesn't stack with proficiency
	scores := AbilityScores{CHA: 16}
	profSkills := []string{"persuasion"}
	got := SkillModifier(scores, "persuasion", profSkills, nil, true, 2)
	if got != 5 { // 3 + 2 (normal proficiency, not half)
		t.Errorf("SkillModifier jack+proficient = %d, want 5", got)
	}
}

func TestSkillModifier_JackOfAllTrades_OddProfBonus(t *testing.T) {
	scores := AbilityScores{WIS: 10}
	got := SkillModifier(scores, "perception", nil, nil, true, 5)
	// 0 + floor(5/2) = 2
	if got != 2 {
		t.Errorf("SkillModifier jack odd bonus = %d, want 2", got)
	}
}

func TestSkillModifier_UnknownSkill(t *testing.T) {
	scores := AbilityScores{}
	got := SkillModifier(scores, "nonexistent", nil, nil, false, 2)
	if got != 0 {
		t.Errorf("SkillModifier unknown = %d, want 0", got)
	}
}
