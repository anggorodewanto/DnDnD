package character

import "slices"

// SavingThrowModifier returns the saving throw modifier for a given ability.
// If proficient in that save, adds proficiency bonus.
func SavingThrowModifier(scores AbilityScores, ability string, profSaves []string, profBonus int) int {
	mod := AbilityModifier(scores.Get(ability))
	if slices.Contains(profSaves, ability) {
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

	if slices.Contains(expertiseSkills, skill) && slices.Contains(profSkills, skill) {
		return mod + profBonus*2
	}
	if slices.Contains(profSkills, skill) {
		return mod + profBonus
	}
	if jackOfAllTrades {
		return mod + profBonus/2
	}
	return mod
}
