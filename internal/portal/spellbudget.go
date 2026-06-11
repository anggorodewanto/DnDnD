package portal

import (
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
)

// cantripBaseKnown is the number of cantrips a class knows at levels 1–3.
// Every cantrip-casting class gains one more cantrip at level 4 and another at
// level 10 (the PHB "Cantrips Known" columns all step up at those two levels),
// so the per-level count is the base plus those step-ups. Classes absent from
// the map (paladin, ranger, non-casters) know no cantrips.
var cantripBaseKnown = map[string]int{
	"bard": 2, "cleric": 3, "druid": 2, "sorcerer": 4, "warlock": 2, "wizard": 3,
}

// cantripsKnown returns how many cantrips a class knows at the given class
// level. Non-cantrip classes return 0.
func cantripsKnown(class string, level int) int {
	base, ok := cantripBaseKnown[strings.ToLower(class)]
	if !ok {
		return 0
	}
	if level < 1 {
		level = 1
	}
	n := base
	if level >= 4 {
		n++
	}
	if level >= 10 {
		n++
	}
	return n
}

// spellsKnownTable maps a "known" caster class to its per-level leveled-spell
// count (index 0 == class level 1). These are the PHB Spells Known columns and
// exclude cantrips. Prepared casters (cleric, druid, wizard, paladin) are not
// listed — they prepare a number derived from ability modifier + level instead.
var spellsKnownTable = map[string][20]int{
	"bard":     {4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 15, 16, 18, 19, 19, 20, 22, 22, 22},
	"sorcerer": {2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 12, 13, 13, 14, 14, 15, 15, 15, 15},
	"ranger":   {0, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11},
	"warlock":  {2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15},
}

// spellsKnown returns (count, true) for a known-caster class at the given
// level, or (0, false) for prepared casters and non-casters.
func spellsKnown(class string, level int) (int, bool) {
	tbl, ok := spellsKnownTable[strings.ToLower(class)]
	if !ok {
		return 0, false
	}
	if level < 1 {
		level = 1
	}
	if level > 20 {
		level = 20
	}
	return tbl[level-1], true
}

// leveledSpellCap returns the number of leveled (non-cantrip) spells a class
// may have at the given level. Known casters use their Spells Known table;
// full prepared casters (cleric, druid, wizard) prepare ability modifier +
// level; paladins (half-caster) prepare ability modifier + half level and have
// no spellcasting before level 2. Non-casters return 0.
func leveledSpellCap(class string, level, abilityMod int) int {
	if known, ok := spellsKnown(class, level); ok {
		return known
	}
	switch strings.ToLower(class) {
	case "cleric", "druid", "wizard":
		return combat.MaxPreparedSpells(abilityMod, level)
	case "paladin":
		if level < 2 {
			return 0
		}
		return combat.MaxPreparedSpells(abilityMod, level/2)
	default:
		return 0
	}
}

// spellBudget returns the total number of spells (cantrips + leveled) a caster
// of the given class/level/ability modifier may select during creation. It is
// the server-side ceiling: the builder UI enforces the precise per-bucket split
// because it knows each spell's level, while the server guards only the combined
// total. A legal build never exceeds this sum, so the ceiling never rejects one,
// yet an over-picked submission is still caught without per-spell level data.
func spellBudget(class string, level, abilityMod int) int {
	return cantripsKnown(class, level) + leveledSpellCap(class, level, abilityMod)
}

// abilityModForCaster resolves a submission's spellcasting-ability modifier for
// the given primary class, mirroring the lookup used by spellCountCap.
func abilityModForCaster(class string, scores character.AbilityScores) int {
	sc := classSpellcasting(class)
	return character.AbilityModifier(scores.Get(sc.SpellAbility))
}
