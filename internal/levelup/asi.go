package levelup

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// ASIType represents the kind of ASI choice the player makes.
type ASIType string

const (
	ASIPlus2     ASIType = "plus2"
	ASIPlus1Plus1 ASIType = "plus1plus1"
	ASIFeat      ASIType = "feat"
)

// ASIChoice represents a player's ASI/Feat selection.
type ASIChoice struct {
	Type     ASIType `json:"type"`
	Ability  string  `json:"ability,omitempty"`
	Ability2 string  `json:"ability2,omitempty"`
	FeatID   string  `json:"feat_id,omitempty"`
}

const maxAbilityScore = 20

var validAbilities = map[string]bool{
	"str": true, "dex": true, "con": true,
	"int": true, "wis": true, "cha": true,
}

// ApplyASI applies an ASI choice to ability scores, capping at 20.
// Returns the new scores or an error if the choice is invalid.
func ApplyASI(scores character.AbilityScores, choice ASIChoice) (character.AbilityScores, error) {
	switch choice.Type {
	case ASIPlus2:
		return applyPlus2(scores, choice.Ability)
	case ASIPlus1Plus1:
		return applyPlus1Plus1(scores, choice.Ability, choice.Ability2)
	default:
		return scores, fmt.Errorf("unsupported ASI type: %s", choice.Type)
	}
}

func getScore(scores character.AbilityScores, ability string) (int, error) {
	if !validAbilities[ability] {
		return 0, fmt.Errorf("invalid ability: %s", ability)
	}
	return scores.Get(ability), nil
}

func setScore(scores *character.AbilityScores, ability string, value int) {
	if value > maxAbilityScore {
		value = maxAbilityScore
	}
	switch ability {
	case "str":
		scores.STR = value
	case "dex":
		scores.DEX = value
	case "con":
		scores.CON = value
	case "int":
		scores.INT = value
	case "wis":
		scores.WIS = value
	case "cha":
		scores.CHA = value
	}
}

func applyPlus2(scores character.AbilityScores, ability string) (character.AbilityScores, error) {
	current, err := getScore(scores, ability)
	if err != nil {
		return scores, err
	}
	if current >= maxAbilityScore {
		return scores, fmt.Errorf("ability %s is already at %d", ability, maxAbilityScore)
	}
	setScore(&scores, ability, current+2)
	return scores, nil
}

func applyPlus1Plus1(scores character.AbilityScores, ability1, ability2 string) (character.AbilityScores, error) {
	if ability1 == ability2 {
		return scores, fmt.Errorf("must choose two different abilities")
	}
	current1, err := getScore(scores, ability1)
	if err != nil {
		return scores, err
	}
	current2, err := getScore(scores, ability2)
	if err != nil {
		return scores, err
	}
	if current1 >= maxAbilityScore {
		return scores, fmt.Errorf("ability %s is already at %d", ability1, maxAbilityScore)
	}
	if current2 >= maxAbilityScore {
		return scores, fmt.Errorf("ability %s is already at %d", ability2, maxAbilityScore)
	}
	setScore(&scores, ability1, current1+1)
	setScore(&scores, ability2, current2+1)
	return scores, nil
}
