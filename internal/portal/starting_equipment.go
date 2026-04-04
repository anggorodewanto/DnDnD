package portal

import "strings"

// EquipmentChoice represents an either/or choice in starting equipment.
type EquipmentChoice struct {
	Label   string   `json:"label"`
	Options []string `json:"options"` // each option is an item ID (or "item-id:qty" for quantity > 1)
}

// EquipmentPack is a set of equipment choices and guaranteed items for a class.
type EquipmentPack struct {
	Choices    []EquipmentChoice `json:"choices"`
	Guaranteed []string          `json:"guaranteed"` // always-included item IDs
}

// StartingEquipmentPacks returns the SRD starting equipment options for a class.
func StartingEquipmentPacks(class string) []EquipmentPack {
	packs, ok := classStartingEquipment[strings.ToLower(class)]
	if !ok {
		return nil
	}
	return packs
}

// classStartingEquipment defines SRD starting equipment per class.
// Each class gets one EquipmentPack containing choices (pick one per choice)
// and guaranteed items (always included).
var classStartingEquipment = map[string][]EquipmentPack{
	"barbarian": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"greataxe", "any-martial-melee"}},
				{Label: "Secondary weapon", Options: []string{"handaxe:2", "any-simple"}},
			},
			Guaranteed: []string{"javelin:4", "explorers-pack"},
		},
	},
	"bard": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"rapier", "longsword", "any-simple"}},
				{Label: "Pack", Options: []string{"diplomats-pack", "entertainers-pack"}},
			},
			Guaranteed: []string{"leather", "dagger"},
		},
	},
	"cleric": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"mace", "warhammer"}},
				{Label: "Armor", Options: []string{"scale-mail", "leather", "chain-mail"}},
				{Label: "Secondary weapon", Options: []string{"light-crossbow:1,crossbow-bolt:20", "any-simple"}},
				{Label: "Pack", Options: []string{"priests-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"shield"},
		},
	},
	"druid": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"shield", "any-simple"}},
				{Label: "Secondary weapon", Options: []string{"scimitar", "any-simple-melee"}},
				{Label: "Pack", Options: []string{"explorers-pack"}},
			},
			Guaranteed: []string{"leather", "druidic-focus"},
		},
	},
	"fighter": {
		{
			Choices: []EquipmentChoice{
				{Label: "Armor", Options: []string{"chain-mail", "leather:1,longbow:1,arrow:20"}},
				{Label: "Primary weapon", Options: []string{"any-martial:1,shield", "any-martial:2"}},
				{Label: "Ranged weapon", Options: []string{"light-crossbow:1,crossbow-bolt:20", "handaxe:2"}},
				{Label: "Pack", Options: []string{"dungeoneers-pack", "explorers-pack"}},
			},
			Guaranteed: nil,
		},
	},
	"monk": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"shortsword", "any-simple"}},
				{Label: "Pack", Options: []string{"dungeoneers-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"dart:10"},
		},
	},
	"paladin": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"any-martial:1,shield", "any-martial:2"}},
				{Label: "Secondary weapon", Options: []string{"javelin:5", "any-simple-melee"}},
				{Label: "Pack", Options: []string{"priests-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"chain-mail"},
		},
	},
	"ranger": {
		{
			Choices: []EquipmentChoice{
				{Label: "Armor", Options: []string{"scale-mail", "leather"}},
				{Label: "Primary weapon", Options: []string{"shortsword:2", "any-simple-melee:2"}},
				{Label: "Pack", Options: []string{"dungeoneers-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"longbow", "arrow:20"},
		},
	},
	"rogue": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"rapier", "shortsword"}},
				{Label: "Secondary weapon", Options: []string{"shortbow:1,arrow:20", "shortsword"}},
				{Label: "Pack", Options: []string{"burglars-pack", "dungeoneers-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"leather", "dagger:2", "thieves-tools"},
		},
	},
	"sorcerer": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"light-crossbow:1,crossbow-bolt:20", "any-simple"}},
				{Label: "Focus", Options: []string{"component-pouch", "arcane-focus"}},
				{Label: "Pack", Options: []string{"dungeoneers-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"dagger:2"},
		},
	},
	"warlock": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"light-crossbow:1,crossbow-bolt:20", "any-simple"}},
				{Label: "Focus", Options: []string{"component-pouch", "arcane-focus"}},
				{Label: "Pack", Options: []string{"scholars-pack", "dungeoneers-pack"}},
			},
			Guaranteed: []string{"leather", "any-simple", "dagger:2"},
		},
	},
	"wizard": {
		{
			Choices: []EquipmentChoice{
				{Label: "Primary weapon", Options: []string{"quarterstaff", "dagger"}},
				{Label: "Focus", Options: []string{"component-pouch", "arcane-focus"}},
				{Label: "Pack", Options: []string{"scholars-pack", "explorers-pack"}},
			},
			Guaranteed: []string{"spellbook"},
		},
	},
}
