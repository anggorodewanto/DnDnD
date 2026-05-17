package character

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	Saves   []string `json:"saves"`
	Skills  []string `json:"skills"`
	Weapons []string `json:"weapons"`
	Armor   []string `json:"armor"`
}

// FeatureUse tracks a single feature with limited uses.
type FeatureUse struct {
	Current  int    `json:"current"`
	Max      int    `json:"max"`
	Recharge string `json:"recharge"`
}

// Feature represents a class or racial feature.
type Feature struct {
	Name             string              `json:"name"`
	Source           string              `json:"source"`
	Level            int                 `json:"level"`
	Description      string              `json:"description"`
	MechanicalEffect string              `json:"mechanical_effect,omitempty"`
	Choices          map[string][]string `json:"choices,omitempty"`
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
	ItemID                string `json:"item_id"`
	Name                  string `json:"name"`
	Quantity              int    `json:"quantity"`
	Equipped              bool   `json:"equipped"`
	EquipSlot             string `json:"equip_slot,omitempty"`
	Type                  string `json:"type"`
	IsMagic               bool   `json:"is_magic"`
	MagicBonus            int    `json:"magic_bonus,omitempty"`
	MagicProperties       string `json:"magic_properties,omitempty"`
	RequiresAttunement    bool   `json:"requires_attunement,omitempty"`
	IsAttuned             bool   `json:"is_attuned,omitempty"`
	AttunementRestriction string `json:"attunement_restriction,omitempty"`
	Rarity                string `json:"rarity,omitempty"`
	Charges               int    `json:"charges,omitempty"`
	MaxCharges            int    `json:"max_charges,omitempty"`
	Identified            *bool  `json:"identified,omitempty"` // nil or true = identified; false = unidentified
	Source                string `json:"source,omitempty"`
	Homebrew              bool   `json:"homebrew,omitempty"`
	IsLit                 bool   `json:"is_lit,omitempty"` // SR-068: torch/lantern currently lit (emits light on map)
}

// ParseInventoryItems unmarshals a character's JSONB inventory field.
func ParseInventoryItems(raw []byte, valid bool) ([]InventoryItem, error) {
	if !valid || len(raw) == 0 {
		return nil, nil
	}
	var items []InventoryItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing inventory: %w", err)
	}
	return items, nil
}

// ParseAttunementSlots unmarshals a character's JSONB attunement_slots field.
func ParseAttunementSlots(raw []byte, valid bool) ([]AttunementSlot, error) {
	if !valid || len(raw) == 0 {
		return nil, nil
	}
	var slots []AttunementSlot
	if err := json.Unmarshal(raw, &slots); err != nil {
		return nil, fmt.Errorf("parsing attunement slots: %w", err)
	}
	return slots, nil
}

// MarshalInventory marshals inventory items to a NullRawMessage-compatible byte slice.
func MarshalInventory(items []InventoryItem) ([]byte, error) {
	return json.Marshal(items)
}

// MarshalAttunementSlots marshals attunement slots to a NullRawMessage-compatible byte slice.
func MarshalAttunementSlots(slots []AttunementSlot) ([]byte, error) {
	return json.Marshal(slots)
}

// ArmorInfo represents the armor data needed for AC calculation.
type ArmorInfo struct {
	ACBase   int  `json:"ac_base"`
	DexBonus bool `json:"dex_bonus"`
	DexMax   int  `json:"dex_max"` // 0 means no cap
	IsShield bool `json:"is_shield"`
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

// FormatClassSummary formats a slice of ClassEntry into a human-readable summary
// like "Fighter 5 / Wizard 3 (Evocation)".
func FormatClassSummary(classes []ClassEntry) string {
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		s := fmt.Sprintf("%s %d", c.Class, c.Level)
		if c.Subclass != "" {
			s += fmt.Sprintf(" (%s)", c.Subclass)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " / ")
}

// Slugify converts a display name to a lowercase hyphenated ID (e.g., "Fire Bolt" -> "fire-bolt").
func Slugify(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
}

// DDBSpellEntry matches the DDB import spell format stored in character_data.
type DDBSpellEntry struct {
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Source   string `json:"source"`
	Homebrew bool   `json:"homebrew,omitempty"`
	OffList  bool   `json:"off_list,omitempty"`
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
