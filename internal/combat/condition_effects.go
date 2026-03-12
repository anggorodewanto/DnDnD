package combat

import (
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
)

// autoFailSTRDEXConditions are conditions that cause auto-fail on STR and DEX saves.
var autoFailSTRDEXConditions = map[string]bool{
	"paralyzed":   true,
	"stunned":     true,
	"unconscious": true,
	"petrified":   true,
}

// CheckSaveConditionEffects checks conditions for effects on saving throws.
// Returns autoFail (the save automatically fails), rollMode (advantage/disadvantage),
// and a list of reason strings.
func CheckSaveConditionEffects(conditions []CombatCondition, ability string) (bool, dice.RollMode, []string) {
	var reasons []string
	var hasAdv, hasDisadv bool

	for _, c := range conditions {
		// Auto-fail STR/DEX saves
		if autoFailSTRDEXConditions[c.Condition] && (ability == "str" || ability == "dex") {
			reasons = append(reasons, fmt.Sprintf("%s: auto-fail %s save", c.Condition, abilityLabel(ability)))
			return true, dice.Normal, reasons
		}

		// Restrained: disadvantage on DEX saves
		if c.Condition == "restrained" && ability == "dex" {
			hasDisadv = true
			reasons = append(reasons, "restrained: disadvantage on DEX save")
		}

		// Dodge: advantage on DEX saves
		if c.Condition == "dodge" && ability == "dex" {
			hasAdv = true
			reasons = append(reasons, "dodge: advantage on DEX save")
		}
	}

	mode := resolveSaveMode(hasAdv, hasDisadv)
	return false, mode, reasons
}

// resolveSaveMode applies 5e cancellation for saves.
func resolveSaveMode(hasAdv, hasDisadv bool) dice.RollMode {
	if hasAdv && hasDisadv {
		return dice.AdvantageAndDisadvantage
	}
	if hasAdv {
		return dice.Advantage
	}
	if hasDisadv {
		return dice.Disadvantage
	}
	return dice.Normal
}

// abilityLabel returns the uppercase short label for an ability.
func abilityLabel(ability string) string {
	switch ability {
	case "str":
		return "STR"
	case "dex":
		return "DEX"
	case "con":
		return "CON"
	case "int":
		return "INT"
	case "wis":
		return "WIS"
	case "cha":
		return "CHA"
	default:
		return ability
	}
}

// CheckAbilityCheckEffects checks conditions for effects on ability checks.
// Returns autoFail, rollMode, and reasons.
func CheckAbilityCheckEffects(conditions []CombatCondition, requiresSight bool, requiresHearing bool) (bool, dice.RollMode, []string) {
	var reasons []string
	var hasDisadv bool

	for _, c := range conditions {
		switch c.Condition {
		case "blinded":
			if requiresSight {
				reasons = append(reasons, "blinded: auto-fail (requires sight)")
				return true, dice.Normal, reasons
			}
		case "deafened":
			if requiresHearing {
				reasons = append(reasons, "deafened: auto-fail (requires hearing)")
				return true, dice.Normal, reasons
			}
		case "frightened":
			hasDisadv = true
			reasons = append(reasons, "frightened: disadvantage on ability checks")
		case "poisoned":
			hasDisadv = true
			reasons = append(reasons, "poisoned: disadvantage on ability checks")
		}
	}

	if hasDisadv {
		return false, dice.Disadvantage, reasons
	}
	return false, dice.Normal, reasons
}

// EffectiveSpeed returns the effective speed after condition effects.
// Grappled and restrained reduce speed to 0.
func EffectiveSpeed(baseSpeed int, conditions []CombatCondition) int {
	for _, c := range conditions {
		if c.Condition == "grappled" || c.Condition == "restrained" {
			return 0
		}
	}
	return baseSpeed
}

// incapacitatingConditions are conditions that block actions and reactions.
var incapacitatingConditions = map[string]bool{
	"incapacitated": true,
	"stunned":       true,
	"paralyzed":     true,
	"unconscious":   true,
	"petrified":     true,
}

// IsIncapacitated returns true if any condition blocks actions/reactions.
func IsIncapacitated(conditions []CombatCondition) bool {
	for _, c := range conditions {
		if incapacitatingConditions[c.Condition] {
			return true
		}
	}
	return false
}

// CanAct returns whether a combatant can take actions. Returns false and a
// reason string if incapacitated.
func CanAct(conditions []CombatCondition) (bool, string) {
	for _, c := range conditions {
		if incapacitatingConditions[c.Condition] {
			return false, fmt.Sprintf("cannot act: %s", c.Condition)
		}
	}
	return true, ""
}

// IsCharmedBy returns true if conditions contain a "charmed" condition whose
// source_combatant_id matches the given target combatant ID.
func IsCharmedBy(conditions []CombatCondition, targetCombatantID string) bool {
	for _, c := range conditions {
		if c.Condition == "charmed" && c.SourceCombatantID == targetCombatantID {
			return true
		}
	}
	return false
}

// ValidateFrightenedMovement checks whether a frightened creature's move would
// bring it closer to the source of its fear. Returns an error if the move is
// invalid, nil if allowed.
func ValidateFrightenedMovement(conditions []CombatCondition, currentCol, currentRow, targetCol, targetRow int, fearSources map[string][2]int) error {
	for _, c := range conditions {
		if c.Condition != "frightened" || c.SourceCombatantID == "" {
			continue
		}
		pos, ok := fearSources[c.SourceCombatantID]
		if !ok {
			continue
		}
		srcCol, srcRow := pos[0], pos[1]
		currentDist := gridDistance(currentCol, currentRow, srcCol, srcRow)
		targetDist := gridDistance(targetCol, targetRow, srcCol, srcRow)
		if targetDist < currentDist {
			return fmt.Errorf("cannot move closer to source of fear")
		}
	}
	return nil
}

// gridDistance returns the Chebyshev distance (5ft grid) between two grid positions.
func gridDistance(col1, row1, col2, row2 int) int {
	dc := col1 - col2
	if dc < 0 {
		dc = -dc
	}
	dr := row1 - row2
	if dr < 0 {
		dr = -dr
	}
	if dc > dr {
		return dc
	}
	return dr
}

// IsIncapacitatedRaw checks conditions from raw JSON.
func IsIncapacitatedRaw(conditions json.RawMessage) bool {
	conds, err := parseConditions(conditions)
	if err != nil {
		return false
	}
	return IsIncapacitated(conds)
}

// CanActRaw checks conditions from raw JSON.
func CanActRaw(conditions json.RawMessage) (bool, string) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return true, ""
	}
	return CanAct(conds)
}
