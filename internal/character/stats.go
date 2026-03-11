package character

import (
	"strconv"
	"strings"
)

// TotalLevel returns the sum of all class levels.
func TotalLevel(classes []ClassEntry) int {
	total := 0
	for _, c := range classes {
		total += c.Level
	}
	return total
}

// CalculateHP computes max HP from class hit dice and CON modifier.
// First class gets max hit die at level 1, all subsequent levels use average+1.
// hitDice maps class name to hit die string (e.g. "fighter" -> "d10").
// Result is at least 1.
func CalculateHP(classes []ClassEntry, hitDice map[string]string, scores AbilityScores) int {
	if len(classes) == 0 {
		return 0
	}

	conMod := AbilityModifier(scores.CON)
	totalLevel := TotalLevel(classes)
	hp := 0

	for i, c := range classes {
		die := HitDieValue(hitDice[c.Class])
		avg := die/2 + 1

		if i == 0 {
			// First class: level 1 gets max die
			hp += die
			hp += (c.Level - 1) * avg
		} else {
			// Subsequent classes: all levels use average
			hp += c.Level * avg
		}
	}

	hp += conMod * totalLevel

	return max(hp, 1)
}

// CalculateAC computes armor class.
// If armor is nil and no formula, uses 10 + DEX.
// If formula is set and no armor, evaluates formula (Unarmored Defense).
// If armor is equipped, formula is ignored (Unarmored Defense doesn't apply).
// Shield adds +2 in all cases.
func CalculateAC(scores AbilityScores, armor *ArmorInfo, hasShield bool, acFormula string) int {
	ac := calculateArmorAC(scores, armor)

	if armor == nil && acFormula != "" {
		ac = max(ac, evaluateACFormula(scores, acFormula))
	}

	if hasShield {
		ac += 2
	}
	return ac
}

func calculateArmorAC(scores AbilityScores, armor *ArmorInfo) int {
	dexMod := AbilityModifier(scores.DEX)
	if armor == nil {
		return 10 + dexMod
	}

	ac := armor.ACBase
	if armor.DexBonus {
		if armor.DexMax > 0 {
			ac += min(dexMod, armor.DexMax)
		} else {
			ac += dexMod
		}
	}
	return ac
}

// evaluateACFormula parses formulas like "10 + DEX + WIS".
func evaluateACFormula(scores AbilityScores, formula string) int {
	result := 0
	parts := splitFormula(formula)
	for _, part := range parts {
		switch part {
		case "STR", "str":
			result += AbilityModifier(scores.STR)
		case "DEX", "dex":
			result += AbilityModifier(scores.DEX)
		case "CON", "con":
			result += AbilityModifier(scores.CON)
		case "INT", "int":
			result += AbilityModifier(scores.INT)
		case "WIS", "wis":
			result += AbilityModifier(scores.WIS)
		case "CHA", "cha":
			result += AbilityModifier(scores.CHA)
		default:
			n, _ := strconv.Atoi(part)
			result += n
		}
	}
	return result
}

// splitFormula splits "10 + DEX + WIS" into ["10", "DEX", "WIS"].
func splitFormula(formula string) []string {
	return strings.Fields(strings.ReplaceAll(formula, "+", " "))
}

// AbilityModifier returns the modifier for a given ability score.
// Formula: floor((score - 10) / 2)
func AbilityModifier(score int) int {
	diff := score - 10
	if diff < 0 {
		return (diff - 1) / 2
	}
	return diff / 2
}

// ProficiencyBonus returns the proficiency bonus for a given total character level.
// Returns 0 for invalid levels (outside 1-20).
func ProficiencyBonus(level int) int {
	if level < 1 || level > 20 {
		return 0
	}
	return (level-1)/4 + 2
}
