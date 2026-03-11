package character

// multiclassSpellSlotTable is the standard 5e multiclass spellcasting table.
// Index 0 is unused; index 1-20 correspond to caster levels 1-20.
// Each entry maps spell level to number of slots.
var multiclassSpellSlotTable = [21]map[int]int{
	0:  nil,
	1:  {1: 2},
	2:  {1: 3},
	3:  {1: 4, 2: 2},
	4:  {1: 4, 2: 3},
	5:  {1: 4, 2: 3, 3: 2},
	6:  {1: 4, 2: 3, 3: 3},
	7:  {1: 4, 2: 3, 3: 3, 4: 1},
	8:  {1: 4, 2: 3, 3: 3, 4: 2},
	9:  {1: 4, 2: 3, 3: 3, 4: 3, 5: 1},
	10: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2},
	11: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1},
	12: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1},
	13: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1},
	14: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1},
	15: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1},
	16: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1},
	17: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1, 9: 1},
	18: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 1, 7: 1, 8: 1, 9: 1},
	19: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 1, 8: 1, 9: 1},
	20: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 2, 8: 1, 9: 1},
}

// MulticastSpellSlots returns the spell slot allocation for a given caster level
// using the multiclass spellcasting table. Returns nil for caster level 0.
func MulticastSpellSlots(casterLevel int) map[int]int {
	if casterLevel < 1 || casterLevel > 20 {
		return nil
	}
	// Return a copy to prevent mutation
	src := multiclassSpellSlotTable[casterLevel]
	result := make(map[int]int, len(src))
	for k, v := range src {
		result[k] = v
	}
	return result
}

// CalculateCasterLevel computes the effective caster level for multiclass spell slots.
// Full casters contribute level*1, half casters level/2, third casters level/3.
// Warlock (pact) and non-casters contribute 0.
func CalculateCasterLevel(classes []ClassEntry, spellcasting map[string]ClassSpellcasting) int {
	total := 0
	for _, c := range classes {
		sc, ok := spellcasting[c.Class]
		if !ok {
			continue
		}
		switch sc.SlotProgression {
		case "full":
			total += c.Level
		case "half":
			total += c.Level / 2
		case "third":
			total += c.Level / 3
		}
	}
	return total
}

// CalculateSpellSlots computes the spell slots for a character.
// For single-class characters, uses own progression (which matches the multiclass table for full casters).
// For multiclass, sums caster levels and looks up the multiclass table.
func CalculateSpellSlots(classes []ClassEntry, spellcasting map[string]ClassSpellcasting) map[int]int {
	casterLevel := CalculateCasterLevel(classes, spellcasting)
	if casterLevel == 0 {
		return nil
	}
	return MulticastSpellSlots(casterLevel)
}
