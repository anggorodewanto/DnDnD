package character

// SavingThrowModifier returns the saving throw modifier for a given ability.
// If proficient in that save, adds proficiency bonus.
func SavingThrowModifier(scores AbilityScores, ability string, profSaves []string, profBonus int) int {
	mod := AbilityModifier(scores.Get(ability))
	if contains(profSaves, ability) {
		mod += profBonus
	}
	return mod
}

// SkillModifier returns the skill check modifier.
// Expertise doubles proficiency bonus. Jack of All Trades adds half proficiency
// (rounded down) to non-proficient skill checks.
func SkillModifier(scores AbilityScores, skill string, profSkills []string, expertiseSkills []string, jackOfAllTrades bool, profBonus int) int {
	ability, ok := SkillAbilityMap[skill]
	if !ok {
		return 0
	}
	mod := AbilityModifier(scores.Get(ability))

	if contains(expertiseSkills, skill) {
		return mod + profBonus*2
	}
	if contains(profSkills, skill) {
		return mod + profBonus
	}
	if jackOfAllTrades {
		return mod + profBonus/2
	}
	return mod
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
