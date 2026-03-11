package character

// AbilityScores represents the six core D&D ability scores.
type AbilityScores struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIS int `json:"wis"`
	CHA int `json:"cha"`
}

// Get returns the ability score for the given abbreviation (case-insensitive).
func (a AbilityScores) Get(ability string) int {
	switch ability {
	case "str", "STR":
		return a.STR
	case "dex", "DEX":
		return a.DEX
	case "con", "CON":
		return a.CON
	case "int", "INT":
		return a.INT
	case "wis", "WIS":
		return a.WIS
	case "cha", "CHA":
		return a.CHA
	}
	return 0
}

// ClassEntry represents a single class (and optional subclass) with a level.
type ClassEntry struct {
	Class    string `json:"class"`
	Subclass string `json:"subclass,omitempty"`
	Level    int    `json:"level"`
}

// Proficiencies represents all character proficiencies.
type Proficiencies struct {
	Saves    []string `json:"saves"`
	Skills   []string `json:"skills"`
	Weapons  []string `json:"weapons"`
	Armor    []string `json:"armor"`
}

// FeatureUse tracks a single feature with limited uses.
type FeatureUse struct {
	Current  int    `json:"current"`
	Max      int    `json:"max"`
	Recharge string `json:"recharge"`
}

// Feature represents a class or racial feature.
type Feature struct {
	Name             string `json:"name"`
	Source           string `json:"source"`
	Level            int    `json:"level"`
	Description      string `json:"description"`
	MechanicalEffect string `json:"mechanical_effect,omitempty"`
}

// SlotInfo tracks current and max for a spell slot level.
type SlotInfo struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

// PactMagicSlots represents Warlock pact magic.
type PactMagicSlots struct {
	SlotLevel int `json:"slot_level"`
	Current   int `json:"current"`
	Max       int `json:"max"`
}

// AttunementSlot represents an attuned magic item.
type AttunementSlot struct {
	ItemID string `json:"item_id"`
	Name   string `json:"name"`
}

// InventoryItem represents a single item in the character's inventory.
type InventoryItem struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	Equipped bool   `json:"equipped"`
	Type     string `json:"type"`
	IsMagic  bool   `json:"is_magic"`
}

// ArmorInfo represents the armor data needed for AC calculation.
type ArmorInfo struct {
	ACBase    int  `json:"ac_base"`
	DexBonus  bool `json:"dex_bonus"`
	DexMax    int  `json:"dex_max"`    // 0 means no cap
	IsShield  bool `json:"is_shield"`
}

// ClassSpellcasting holds the spellcasting data from the class reference.
type ClassSpellcasting struct {
	SpellAbility    string `json:"spell_ability"`
	SlotProgression string `json:"slot_progression"` // "full", "half", "third", "pact", "none"
}

// HitDieValue returns the numeric value for a hit die string.
func HitDieValue(hitDie string) int {
	switch hitDie {
	case "d6":
		return 6
	case "d8":
		return 8
	case "d10":
		return 10
	case "d12":
		return 12
	}
	return 0
}

// SkillAbilityMap maps each skill to its associated ability.
var SkillAbilityMap = map[string]string{
	"acrobatics":      "dex",
	"animal-handling": "wis",
	"arcana":          "int",
	"athletics":       "str",
	"deception":       "cha",
	"history":         "int",
	"insight":         "wis",
	"intimidation":    "cha",
	"investigation":   "int",
	"medicine":        "wis",
	"nature":          "int",
	"perception":      "wis",
	"performance":     "cha",
	"persuasion":      "cha",
	"religion":        "int",
	"sleight-of-hand": "dex",
	"stealth":         "dex",
	"survival":        "wis",
}
