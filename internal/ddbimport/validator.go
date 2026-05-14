package ddbimport

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// Warning represents a non-blocking advisory validation issue.
type Warning struct {
	Message string
}

func (w Warning) String() string { return w.Message }

// multiclassPrereqs maps class names to their multiclass ability score prerequisites.
var multiclassPrereqs = map[string][]string{
	"Barbarian": {"STR"},
	"Bard":      {"CHA"},
	"Cleric":    {"WIS"},
	"Druid":     {"WIS"},
	"Fighter":   {"STR,DEX"}, // STR or DEX
	"Monk":      {"DEX", "WIS"},
	"Paladin":   {"STR", "CHA"},
	"Ranger":    {"DEX", "WIS"},
	"Rogue":     {"DEX"},
	"Sorcerer":  {"CHA"},
	"Warlock":   {"CHA"},
	"Wizard":    {"INT"},
}

// Validate performs structural validation (returns error) and advisory warnings.
func Validate(pc *ParsedCharacter) ([]Warning, error) {
	if pc == nil {
		return nil, fmt.Errorf("character is nil")
	}

	// Structural validation
	if pc.Name == "" {
		return nil, fmt.Errorf("character name is required")
	}

	if pc.Level < 1 || pc.Level > 20 {
		return nil, fmt.Errorf("level %d is out of range (1-20)", pc.Level)
	}

	if len(pc.Classes) == 0 {
		return nil, fmt.Errorf("at least one class is required")
	}

	if pc.HPMax <= 0 {
		return nil, fmt.Errorf("HP max must be greater than 0")
	}

	// Validate ability scores in range 1-30
	scores := map[string]int{
		"STR": pc.AbilityScores.STR,
		"DEX": pc.AbilityScores.DEX,
		"CON": pc.AbilityScores.CON,
		"INT": pc.AbilityScores.INT,
		"WIS": pc.AbilityScores.WIS,
		"CHA": pc.AbilityScores.CHA,
	}
	for name, val := range scores {
		if val < 1 || val > 30 {
			return nil, fmt.Errorf("%s %d is out of range (1-30)", name, val)
		}
	}

	// Advisory warnings
	var warnings []Warning

	// High ability score warning
	for name, val := range scores {
		if val > 20 {
			warnings = append(warnings, Warning{
				Message: fmt.Sprintf("%s %d — exceeds 20 without a detected magic item source", name, val),
			})
		}
	}

	// Multiclass prerequisite check
	if len(pc.Classes) > 1 {
		for _, cls := range pc.Classes {
			prereqs, ok := multiclassPrereqs[cls.Class]
			if !ok {
				continue
			}
			for _, prereq := range prereqs {
				if meetsPrereq(scores, prereq) {
					continue
				}
				warnings = append(warnings, Warning{
					Message: fmt.Sprintf("Multiclass %s — does not meet 13 %s minimum", cls.Class, prereq),
				})
			}
		}
	}

	// Attunement limit check
	attunedCount := 0
	for _, item := range pc.Inventory {
		if item.Equipped && item.RequiresAttunement {
			attunedCount++
		}
	}
	if attunedCount > 3 {
		warnings = append(warnings, Warning{
			Message: fmt.Sprintf("%d attuned items — exceeds default attunement limit of 3", attunedCount),
		})
	}

	for _, spell := range pc.Spells {
		if !spell.OffList {
			continue
		}
		className := primaryClassName(pc.Classes)
		if className == "" {
			continue
		}
		warnings = append(warnings, Warning{
			Message: fmt.Sprintf("%s spell list includes %s (not on %s spell list)", className, spell.Name, className),
		})
	}

	return warnings, nil
}

func primaryClassName(classes []character.ClassEntry) string {
	if len(classes) == 0 {
		return ""
	}
	return classes[0].Class
}

// meetsPrereq checks if the given ability scores meet a prerequisite string.
// A prereq like "STR,DEX" means either STR or DEX must be >= 13.
func meetsPrereq(scores map[string]int, prereq string) bool {
	if !strings.Contains(prereq, ",") {
		return scores[prereq] >= 13
	}
	for _, p := range strings.Split(prereq, ",") {
		if scores[p] >= 13 {
			return true
		}
	}
	return false
}
