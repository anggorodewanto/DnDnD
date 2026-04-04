package levelup

import (
	"github.com/ab/dndnd/internal/character"
)

// LevelUpResult holds the computed stats after a level-up.
type LevelUpResult struct {
	NewLevel            int
	OldLevel            int
	NewHPMax            int
	HPGained            int
	NewProficiencyBonus int
	NewSpellSlots       map[int]int
	NewPactSlots        character.PactMagicSlots
	NewAttacksPerAction int
	LeveledClass        string
	LeveledClassLevel   int
}

// CalculateLevelUp computes the new stats resulting from a level change.
// oldClasses/newClasses are the class entries before/after the level-up.
// hitDice maps class name -> hit die string (e.g. "fighter" -> "d10").
// spellcasting maps class name -> ClassSpellcasting.
// attacksMap maps class name -> {threshold_level: attacks_count}.
func CalculateLevelUp(
	oldClasses, newClasses []character.ClassEntry,
	hitDice map[string]string,
	spellcasting map[string]character.ClassSpellcasting,
	attacksMap map[string]map[int]int,
	scores character.AbilityScores,
) LevelUpResult {
	oldLevel := character.TotalLevel(oldClasses)
	newLevel := character.TotalLevel(newClasses)
	oldHP := character.CalculateHP(oldClasses, hitDice, scores)
	newHP := character.CalculateHP(newClasses, hitDice, scores)

	// Determine which class was leveled
	leveledClass, leveledClassLevel := findLeveledClass(oldClasses, newClasses)

	// Spell slots
	newSpellSlots := character.CalculateSpellSlots(newClasses, spellcasting)

	// Pact magic
	var newPactSlots character.PactMagicSlots
	for _, c := range newClasses {
		sc, ok := spellcasting[c.Class]
		if ok && sc.SlotProgression == "pact" {
			newPactSlots = character.PactMagicSlotsForLevel(c.Level)
			break
		}
	}

	// Attacks per action: highest across all classes
	newAttacks := maxAttacksPerAction(newClasses, attacksMap)

	return LevelUpResult{
		NewLevel:            newLevel,
		OldLevel:            oldLevel,
		NewHPMax:            newHP,
		HPGained:            newHP - oldHP,
		NewProficiencyBonus: character.ProficiencyBonus(newLevel),
		NewSpellSlots:       newSpellSlots,
		NewPactSlots:        newPactSlots,
		NewAttacksPerAction: newAttacks,
		LeveledClass:        leveledClass,
		LeveledClassLevel:   leveledClassLevel,
	}
}

// asiLevels are the standard class levels that grant an ASI.
var asiLevels = map[int]bool{4: true, 8: true, 12: true, 16: true, 19: true}

// extraASILevels maps class IDs to their additional ASI levels beyond the standard ones.
// Fighter gets extra ASI at 6 and 14; Rogue gets extra ASI at 10.
var extraASILevels = map[string]map[int]bool{
	"fighter": {6: true, 14: true},
	"rogue":   {10: true},
}

// IsASILevel returns true if the given class level grants an ASI/Feat choice.
// classID is used to check for class-specific extra ASI levels (e.g. Fighter 6/14, Rogue 10).
func IsASILevel(classID string, classLevel int) bool {
	if asiLevels[classLevel] {
		return true
	}
	if extras, ok := extraASILevels[classID]; ok {
		return extras[classLevel]
	}
	return false
}

// NeedsSubclassSelection returns true if the class level has reached or passed
// the subclass threshold and no subclass has been chosen yet.
func NeedsSubclassSelection(classLevel, subclassLevel int, hasSubclass bool) bool {
	if hasSubclass {
		return false
	}
	return classLevel >= subclassLevel
}

// findLeveledClass compares old and new class entries to find which class changed.
func findLeveledClass(oldClasses, newClasses []character.ClassEntry) (string, int) {
	oldMap := make(map[string]int, len(oldClasses))
	for _, c := range oldClasses {
		oldMap[c.Class] = c.Level
	}
	for _, c := range newClasses {
		oldLevel, existed := oldMap[c.Class]
		if !existed || c.Level > oldLevel {
			return c.Class, c.Level
		}
	}
	if len(newClasses) > 0 {
		return newClasses[0].Class, newClasses[0].Level
	}
	return "", 0
}

// maxAttacksPerAction returns the highest attacks per action across all class entries,
// using the attacks_per_action threshold table for each class.
func maxAttacksPerAction(classes []character.ClassEntry, attacksMap map[string]map[int]int) int {
	best := 1
	for _, c := range classes {
		table, ok := attacksMap[c.Class]
		if !ok {
			continue
		}
		attacks := attacksForLevel(table, c.Level)
		if attacks > best {
			best = attacks
		}
	}
	return best
}

// attacksForLevel looks up the attacks per action for a given class level
// using a threshold table (e.g. {1: 1, 5: 2, 11: 3, 20: 4}).
func attacksForLevel(table map[int]int, level int) int {
	best := 1
	for threshold, attacks := range table {
		if level >= threshold && attacks > best {
			best = attacks
		}
	}
	return best
}
