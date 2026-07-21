package levelup

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ab/dndnd/internal/character"
)

var ErrInvalidFeatChoices = errors.New("invalid feat choices")

// ErrFeatNotPresent is returned by RemoveFeat / RetrainFeat when the character
// does not carry the feat being removed — the caller (DM) started from a state
// that no longer matches, so the swap is refused rather than silently ignored.
var ErrFeatNotPresent = errors.New("feat not present on character")

// ErrFeatRetrainUnsupported is returned when a feat's reversal can't be applied
// cleanly. Resilient and Skilled grant saving-throw / skill proficiencies via
// applyFeatProficiencyChoices, and there is no record of whether the character
// already had those proficiencies before the feat was taken — so un-granting
// them could strip a proficiency the character legitimately owns. Retraining
// out of these feats is refused.
var ErrFeatRetrainUnsupported = errors.New("feat retrain not supported")

// ErrFeatRetrainSame is returned when a retrain names the same feat for both
// sides of the swap, which would needlessly strip then re-add it.
var ErrFeatRetrainSame = errors.New("cannot retrain a feat into itself")

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
	Choices          FeatChoices
}

// FeatChoices carries feat-internal selections made before DM approval.
type FeatChoices struct {
	Ability    string   `json:"ability,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	DamageType string   `json:"damage_type,omitempty"`
}

func validateRequiredFeatChoices(feat FeatInfo) error {
	switch feat.ID {
	case "resilient":
		if !validAbilities[feat.Choices.Ability] {
			return fmt.Errorf("%w: resilient requires a valid ability choice", ErrInvalidFeatChoices)
		}
	case "skilled":
		if len(feat.Choices.Skills) != 3 {
			return fmt.Errorf("%w: skilled requires exactly three skill choices", ErrInvalidFeatChoices)
		}
		seen := make(map[string]bool, len(feat.Choices.Skills))
		for _, skill := range feat.Choices.Skills {
			if !validSkill(skill) {
				return fmt.Errorf("%w: skilled has invalid skill choice %q", ErrInvalidFeatChoices, skill)
			}
			if seen[skill] {
				return fmt.Errorf("%w: skilled requires three distinct skill choices", ErrInvalidFeatChoices)
			}
			seen[skill] = true
		}
	case "elemental-adept":
		if !validElementalAdeptDamageType(feat.Choices.DamageType) {
			return fmt.Errorf("%w: elemental adept requires a valid damage type choice", ErrInvalidFeatChoices)
		}
	}
	return nil
}

func validElementalAdeptDamageType(damageType string) bool {
	switch damageType {
	case "acid", "cold", "fire", "lightning", "thunder":
		return true
	default:
		return false
	}
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
