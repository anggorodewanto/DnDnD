package levelup

import (
	"fmt"
	"slices"

	"github.com/ab/dndnd/internal/character"
)

// FeatPrerequisites represents the prerequisites for a feat.
type FeatPrerequisites struct {
	Ability      map[string]int `json:"ability,omitempty"`
	AbilityOr    map[string]int `json:"ability_or,omitempty"`
	Spellcasting bool           `json:"spellcasting,omitempty"`
	Proficiency  string         `json:"proficiency,omitempty"`
}

// FeatInfo holds the data needed to validate and apply a feat selection.
type FeatInfo struct {
	ID               string
	Name             string
	Prerequisites    FeatPrerequisites
	ASIBonus         map[string]any
	MechanicalEffect []map[string]string
}

// CheckFeatPrerequisites validates whether a character meets a feat's prerequisites.
// Returns (true, "") if met, or (false, reason) if not met.
func CheckFeatPrerequisites(
	prereqs FeatPrerequisites,
	scores character.AbilityScores,
	armorProficiencies []string,
	isSpellcaster bool,
) (bool, string) {
	// Check ability prerequisites
	for ability, minScore := range prereqs.Ability {
		if scores.Get(ability) < minScore {
			return false, fmt.Sprintf("requires %s %d (have %d)", ability, minScore, scores.Get(ability))
		}
	}

	// Check ability_or prerequisites (any one must be met)
	if len(prereqs.AbilityOr) > 0 {
		met := false
		for ability, minScore := range prereqs.AbilityOr {
			if scores.Get(ability) >= minScore {
				met = true
				break
			}
		}
		if !met {
			return false, "does not meet any of the required ability scores"
		}
	}

	// Check spellcasting prerequisite
	if prereqs.Spellcasting && !isSpellcaster {
		return false, "requires spellcasting ability"
	}

	// Check proficiency prerequisite
	if prereqs.Proficiency != "" {
		if !slices.Contains(armorProficiencies, prereqs.Proficiency) {
			return false, fmt.Sprintf("requires %s proficiency", prereqs.Proficiency)
		}
	}

	return true, ""
}
