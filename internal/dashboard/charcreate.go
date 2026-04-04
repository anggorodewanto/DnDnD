package dashboard

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
)

// DMCharacterSubmission is the payload for DM-created characters.
type DMCharacterSubmission struct {
	Name          string                 `json:"name"`
	Race          string                 `json:"race"`
	Background    string                 `json:"background"`
	Classes       []character.ClassEntry `json:"classes"`
	AbilityScores character.AbilityScores `json:"ability_scores"`
}

// ValidateDMSubmission returns a list of validation error messages.
func ValidateDMSubmission(s DMCharacterSubmission) []string {
	var errs []string

	if s.Name == "" {
		errs = append(errs, "name is required")
	}
	if s.Race == "" {
		errs = append(errs, "race is required")
	}
	if len(s.Classes) == 0 {
		errs = append(errs, "at least one class is required")
	}

	totalLevel := 0
	for i, c := range s.Classes {
		if c.Class == "" {
			errs = append(errs, fmt.Sprintf("class entry %d: class name is required", i+1))
		}
		if c.Level < 1 {
			errs = append(errs, fmt.Sprintf("class entry %d: level must be at least 1", i+1))
		}
		totalLevel += c.Level
	}

	if totalLevel > 20 {
		errs = append(errs, "total level cannot exceed 20")
	}

	abilityNames := []struct {
		name  string
		value int
	}{
		{"STR", s.AbilityScores.STR},
		{"DEX", s.AbilityScores.DEX},
		{"CON", s.AbilityScores.CON},
		{"INT", s.AbilityScores.INT},
		{"WIS", s.AbilityScores.WIS},
		{"CHA", s.AbilityScores.CHA},
	}
	for _, a := range abilityNames {
		if a.value < 1 || a.value > 30 {
			errs = append(errs, fmt.Sprintf("%s must be between 1 and 30", a.name))
		}
	}

	return errs
}

// DMDerivedStats holds the derived statistics for a DM-created character.
type DMDerivedStats struct {
	HPMax             int            `json:"hp_max"`
	AC                int            `json:"ac"`
	SpeedFt           int            `json:"speed_ft"`
	TotalLevel        int            `json:"total_level"`
	ProficiencyBonus  int            `json:"proficiency_bonus"`
	SaveProficiencies []string       `json:"save_proficiencies"`
	Saves             map[string]int `json:"saves"`
	Skills            map[string]int `json:"skills"`
	HitDiceRemaining  map[string]int `json:"hit_dice_remaining"`
}

// DeriveDMStats calculates derived stats from a DM character submission.
func DeriveDMStats(sub DMCharacterSubmission) DMDerivedStats {
	totalLevel := character.TotalLevel(sub.Classes)
	profBonus := character.ProficiencyBonus(totalLevel)

	// Build hit dice map for HP calculation
	hitDice := make(map[string]string, len(sub.Classes))
	for _, c := range sub.Classes {
		hitDice[c.Class] = portal.ClassHitDie(c.Class)
	}

	hp := character.CalculateHP(sub.Classes, hitDice, sub.AbilityScores)
	ac := character.CalculateAC(sub.AbilityScores, nil, false, "")

	// Save proficiencies from primary class
	saveProficiencies := classSaveProficiencies(sub.Classes)

	// Calculate saving throw values
	saves := make(map[string]int, 6)
	abilities := []string{"str", "dex", "con", "int", "wis", "cha"}
	for _, ab := range abilities {
		saves[ab] = character.SavingThrowModifier(sub.AbilityScores, ab, saveProficiencies, profBonus)
	}

	// Skill proficiencies from primary class
	skillProfs := classSkillProficiencies(sub.Classes)

	// Calculate skill modifiers for all 18 skills
	skills := make(map[string]int, len(character.SkillAbilityMap))
	for skill := range character.SkillAbilityMap {
		skills[skill] = character.SkillModifier(sub.AbilityScores, skill, skillProfs, nil, false, profBonus)
	}

	// Hit dice remaining (starts full)
	hitDiceRemaining := make(map[string]int, len(sub.Classes))
	for _, c := range sub.Classes {
		hitDiceRemaining[c.Class] = c.Level
	}

	return DMDerivedStats{
		HPMax:             hp,
		AC:                ac,
		SpeedFt:           raceSpeed(sub.Race),
		TotalLevel:        totalLevel,
		ProficiencyBonus:  profBonus,
		SaveProficiencies: saveProficiencies,
		Saves:             saves,
		Skills:            skills,
		HitDiceRemaining:  hitDiceRemaining,
	}
}

// raceSpeed returns the base walking speed in feet for a race name.
// Defaults to 30 ft for unknown races.
func raceSpeed(race string) int {
	switch strings.ToLower(race) {
	case "dwarf", "hill dwarf", "mountain dwarf":
		return 25
	case "halfling", "lightfoot halfling", "stout halfling":
		return 25
	case "gnome", "forest gnome", "rock gnome":
		return 25
	case "wood elf":
		return 35
	default:
		return 30
	}
}

// classSkillProficiencies returns default skill proficiencies for the primary (first) class.
// These are the SRD default skill proficiencies granted by each class.
func classSkillProficiencies(classes []character.ClassEntry) []string {
	if len(classes) == 0 {
		return nil
	}
	switch strings.ToLower(classes[0].Class) {
	case "barbarian":
		return []string{"athletics", "perception"}
	case "bard":
		return []string{"performance", "persuasion", "deception"}
	case "cleric":
		return []string{"insight", "medicine"}
	case "druid":
		return []string{"nature", "perception"}
	case "fighter":
		return []string{"athletics", "perception"}
	case "monk":
		return []string{"acrobatics", "insight"}
	case "paladin":
		return []string{"athletics", "persuasion"}
	case "ranger":
		return []string{"perception", "survival", "stealth"}
	case "rogue":
		return []string{"acrobatics", "stealth", "perception", "deception"}
	case "sorcerer":
		return []string{"arcana", "persuasion"}
	case "warlock":
		return []string{"arcana", "deception"}
	case "wizard":
		return []string{"arcana", "investigation"}
	default:
		return nil
	}
}

// classSaveProficiencies returns save proficiencies for the primary (first) class.
func classSaveProficiencies(classes []character.ClassEntry) []string {
	if len(classes) == 0 {
		return nil
	}
	switch strings.ToLower(classes[0].Class) {
	case "barbarian":
		return []string{"str", "con"}
	case "bard":
		return []string{"dex", "cha"}
	case "cleric":
		return []string{"wis", "cha"}
	case "druid":
		return []string{"int", "wis"}
	case "fighter":
		return []string{"str", "con"}
	case "monk":
		return []string{"str", "dex"}
	case "paladin":
		return []string{"wis", "cha"}
	case "ranger":
		return []string{"str", "dex"}
	case "rogue":
		return []string{"dex", "int"}
	case "sorcerer":
		return []string{"con", "cha"}
	case "warlock":
		return []string{"wis", "cha"}
	case "wizard":
		return []string{"int", "wis"}
	default:
		return nil
	}
}
