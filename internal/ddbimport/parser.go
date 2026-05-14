package ddbimport

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// SpellEntry represents a spell known or prepared by the character.
type SpellEntry struct {
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Source   string `json:"source"`
	Homebrew bool   `json:"homebrew,omitempty"`
	OffList  bool   `json:"off_list,omitempty"`
}

// ParsedCharacter holds the parsed result from DDB JSON, ready for conversion to internal format.
type ParsedCharacter struct {
	Name          string
	Race          string
	Classes       []character.ClassEntry
	Level         int
	AbilityScores character.AbilityScores
	HPMax         int
	HPCurrent     int
	TempHP        int
	AC            int
	SpeedFt       int
	Gold          int
	Languages     []string
	Inventory     []character.InventoryItem
	Proficiencies character.Proficiencies
	Features      []character.Feature
	Spells        []SpellEntry
}

// ddbResponse represents the top-level DDB API response.
type ddbResponse struct {
	Data *ddbCharacter `json:"data"`
}

type ddbCharacter struct {
	Name               string        `json:"name"`
	Race               ddbRace       `json:"race"`
	Classes            []ddbClass    `json:"classes"`
	Stats              []ddbStat     `json:"stats"`
	BonusStats         []ddbStat     `json:"bonusStats"`
	OverrideStats      []ddbStat     `json:"overrideStats"`
	BaseHitPoints      int           `json:"baseHitPoints"`
	BonusHitPoints     int           `json:"bonusHitPoints"`
	OverrideHitPoints  *int          `json:"overrideHitPoints"`
	RemovedHitPoints   int           `json:"removedHitPoints"`
	TemporaryHitPoints int           `json:"temporaryHitPoints"`
	Inventory          []ddbItem     `json:"inventory"`
	Currencies         ddbCurrencies `json:"currencies"`
	Modifiers          ddbModifiers  `json:"modifiers"`
	Spells             ddbSpells     `json:"spells"`
}

type ddbRace struct {
	FullName string `json:"fullName"`
}

type ddbClass struct {
	Definition         ddbClassDef  `json:"definition"`
	SubclassDefinition *ddbSubclass `json:"subclassDefinition"`
	Level              int          `json:"level"`
}

type ddbClassDef struct {
	Name          string            `json:"name"`
	HitDice       int               `json:"hitDice"`
	ClassFeatures []ddbClassFeature `json:"classFeatures"`
}

type ddbSubclass struct {
	Name          string            `json:"name"`
	ClassFeatures []ddbClassFeature `json:"classFeatures"`
}

type ddbClassFeature struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	RequiredLevel int    `json:"requiredLevel"`
	Description   string `json:"description"`
}

type ddbStat struct {
	ID    int  `json:"id"`
	Value *int `json:"value"`
}

type ddbItem struct {
	ID         int        `json:"id"`
	Definition ddbItemDef `json:"definition"`
	Equipped   bool       `json:"equipped"`
	Quantity   int        `json:"quantity"`
}

type ddbItemDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	FilterType  string `json:"filterType"`
	ArmorClass  int    `json:"armorClass"`
	CanAttune   bool   `json:"canAttune"`
	Magic       bool   `json:"magic"`
	Rarity      string `json:"rarity"`
	Description string `json:"description"`
	SourceName  string `json:"sourceName"`
	IsHomebrew  bool   `json:"isHomebrew"`
	Homebrew    bool   `json:"homebrew"`
}

type ddbCurrencies struct {
	GP int `json:"gp"`
	SP int `json:"sp"`
	CP int `json:"cp"`
	EP int `json:"ep"`
	PP int `json:"pp"`
}

type ddbModifiers struct {
	Race       []ddbModifier `json:"race"`
	Class      []ddbModifier `json:"class"`
	Background []ddbModifier `json:"background"`
	Item       []ddbModifier `json:"item"`
	Feat       []ddbModifier `json:"feat"`
	Condition  []ddbModifier `json:"condition"`
}

type ddbModifier struct {
	Type                string `json:"type"`
	SubType             string `json:"subType"`
	FriendlyTypeName    string `json:"friendlyTypeName"`
	FriendlySubtypeName string `json:"friendlySubtypeName"`
}

type ddbSpells struct {
	Class []ddbSpellEntry `json:"class"`
	Race  []ddbSpellEntry `json:"race"`
	Item  []ddbSpellEntry `json:"item"`
	Feat  []ddbSpellEntry `json:"feat"`
}

type ddbSpellEntry struct {
	Definition ddbSpellDef `json:"definition"`
}

type ddbSpellDef struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// ParseDDBJSON parses a DDB API response into ParsedCharacter.
func ParseDDBJSON(data []byte) (*ParsedCharacter, error) {
	var resp ddbResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing DDB JSON: %w", err)
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("DDB response has no character data")
	}

	d := resp.Data
	pc := &ParsedCharacter{
		Name: d.Name,
		Race: d.Race.FullName,
	}

	// Parse classes and compute total level
	for _, c := range d.Classes {
		entry := character.ClassEntry{
			Class: c.Definition.Name,
			Level: c.Level,
		}
		if c.SubclassDefinition != nil {
			entry.Subclass = c.SubclassDefinition.Name
		}
		pc.Classes = append(pc.Classes, entry)
		pc.Level += c.Level
	}

	// Parse ability scores: base + bonus, overridden if set
	pc.AbilityScores = parseAbilityScores(d.Stats, d.BonusStats, d.OverrideStats)

	// Parse HP
	pc.HPMax = d.BaseHitPoints + d.BonusHitPoints
	if d.OverrideHitPoints != nil {
		pc.HPMax = *d.OverrideHitPoints
	}
	pc.HPCurrent = pc.HPMax - d.RemovedHitPoints
	if pc.HPCurrent < 0 {
		pc.HPCurrent = 0
	}
	pc.TempHP = d.TemporaryHitPoints

	// Parse inventory and compute AC
	pc.Inventory = parseInventory(d.Inventory)
	pc.AC = computeAC(d.Inventory, pc.AbilityScores)

	// Parse gold (convert all to GP)
	pc.Gold = d.Currencies.GP + d.Currencies.SP/10 + d.Currencies.CP/100 + d.Currencies.EP/2 + d.Currencies.PP*10

	// Parse languages from modifiers
	pc.Languages = parseLanguages(&d.Modifiers)

	// Parse proficiencies from modifiers
	pc.Proficiencies = parseProficiencies(&d.Modifiers)

	// Parse features from class definitions
	pc.Features = parseFeatures(d.Classes)

	// Parse spells from all sources
	pc.Spells = parseSpells(&d.Spells, pc.Classes)

	// Speed defaults to 30 (DDB doesn't have a simple speed field; it's computed from modifiers)
	pc.SpeedFt = 30

	return pc, nil
}

// parseAbilityScores computes final ability scores from base, bonus, and override stats.
// DDB stat IDs: 1=STR, 2=DEX, 3=CON, 4=INT, 5=WIS, 6=CHA
func parseAbilityScores(base, bonus, override []ddbStat) character.AbilityScores {
	scores := make(map[int]int)

	for _, s := range base {
		if s.Value != nil {
			scores[s.ID] = *s.Value
		}
	}

	for _, s := range bonus {
		if s.Value != nil {
			scores[s.ID] += *s.Value
		}
	}

	for _, s := range override {
		if s.Value != nil {
			scores[s.ID] = *s.Value
		}
	}

	return character.AbilityScores{
		STR: scores[1],
		DEX: scores[2],
		CON: scores[3],
		INT: scores[4],
		WIS: scores[5],
		CHA: scores[6],
	}
}

func parseInventory(items []ddbItem) []character.InventoryItem {
	var result []character.InventoryItem
	for _, item := range items {
		result = append(result, character.InventoryItem{
			ItemID:             fmt.Sprintf("ddb-%d", item.ID),
			Name:               item.Definition.Name,
			Quantity:           item.Quantity,
			Equipped:           item.Equipped,
			Type:               item.Definition.FilterType,
			IsMagic:            item.Definition.Magic,
			RequiresAttunement: item.Definition.CanAttune,
			Rarity:             item.Definition.Rarity,
			Source:             item.Definition.SourceName,
			Homebrew:           isDDBItemHomebrew(item.Definition),
		})
	}
	return result
}

func isDDBItemHomebrew(def ddbItemDef) bool {
	if def.IsHomebrew || def.Homebrew {
		return true
	}
	return strings.Contains(strings.ToLower(def.SourceName), "homebrew")
}

// computeAC calculates AC from equipped armor and shield.
// Falls back to 10+DEX if no armor is equipped.
func computeAC(items []ddbItem, scores character.AbilityScores) int {
	baseAC := 0
	shieldBonus := 0

	for _, item := range items {
		if !item.Equipped {
			continue
		}
		if item.Definition.FilterType != "Armor" {
			continue
		}
		ac := item.Definition.ArmorClass
		if ac <= 3 {
			// Shield
			shieldBonus += ac
		} else {
			baseAC = ac
		}
	}

	if baseAC == 0 {
		// Unarmored: 10 + DEX mod
		dexMod := (scores.DEX - 10) / 2
		baseAC = 10 + dexMod
	}

	return baseAC + shieldBonus
}

func parseFeatures(classes []ddbClass) []character.Feature {
	var features []character.Feature

	for _, c := range classes {
		for _, f := range c.Definition.ClassFeatures {
			if f.RequiredLevel > c.Level {
				continue
			}
			features = append(features, character.Feature{
				Name:        f.Name,
				Source:      c.Definition.Name,
				Level:       f.RequiredLevel,
				Description: f.Description,
			})
		}

		if c.SubclassDefinition == nil {
			continue
		}
		for _, f := range c.SubclassDefinition.ClassFeatures {
			if f.RequiredLevel > c.Level {
				continue
			}
			features = append(features, character.Feature{
				Name:        f.Name,
				Source:      c.SubclassDefinition.Name,
				Level:       f.RequiredLevel,
				Description: f.Description,
			})
		}
	}

	return features
}

func parseSpells(spells *ddbSpells, classes []character.ClassEntry) []SpellEntry {
	var result []SpellEntry

	sources := []struct {
		entries []ddbSpellEntry
		source  string
	}{
		{spells.Class, "class"},
		{spells.Race, "race"},
		{spells.Item, "item"},
		{spells.Feat, "feat"},
	}

	for _, src := range sources {
		for _, entry := range src.entries {
			spell := SpellEntry{
				Name:   entry.Definition.Name,
				Level:  entry.Definition.Level,
				Source: src.source,
			}
			if src.source == "class" && isOffListClassSpell(spell.Name, classes) {
				spell.OffList = true
				spell.Homebrew = true
			}
			result = append(result, spell)
		}
	}

	return result
}

var classSpellLists = map[string]map[string]struct{}{
	"wizard": {
		"acid splash":      {},
		"burning hands":    {},
		"charm person":     {},
		"detect magic":     {},
		"fire bolt":        {},
		"fireball":         {},
		"light":            {},
		"mage armor":       {},
		"magic missile":    {},
		"prestidigitation": {},
		"ray of frost":     {},
		"shield":           {},
		"shocking grasp":   {},
		"sleep":            {},
		"thunderwave":      {},
		"true strike":      {},
	},
}

func isOffListClassSpell(name string, classes []character.ClassEntry) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return false
	}

	hasKnownList := false
	for _, class := range classes {
		list, ok := classSpellLists[strings.ToLower(strings.TrimSpace(class.Class))]
		if !ok {
			continue
		}
		hasKnownList = true
		if _, ok := list[normalized]; ok {
			return false
		}
	}

	return hasKnownList
}

// allModifiers collects modifiers from all sources (race, class, background, feat).
func allModifiers(mods *ddbModifiers) []ddbModifier {
	var all []ddbModifier
	all = append(all, mods.Race...)
	all = append(all, mods.Class...)
	all = append(all, mods.Background...)
	all = append(all, mods.Feat...)
	return all
}

func parseLanguages(mods *ddbModifiers) []string {
	seen := make(map[string]bool)
	var langs []string

	for _, m := range allModifiers(mods) {
		if m.Type != "language" {
			continue
		}
		name := m.FriendlySubtypeName
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		langs = append(langs, name)
	}

	return langs
}

func parseProficiencies(mods *ddbModifiers) character.Proficiencies {
	var profs character.Proficiencies
	seenSaves := make(map[string]bool)
	seenSkills := make(map[string]bool)

	for _, m := range allModifiers(mods) {
		if m.Type != "proficiency" {
			continue
		}
		name := m.FriendlySubtypeName
		if name == "" {
			continue
		}

		switch {
		case m.SubType == "saving-throws" && !seenSaves[name]:
			seenSaves[name] = true
			profs.Saves = append(profs.Saves, name)
		case m.FriendlyTypeName == "Skill" && !seenSkills[name]:
			seenSkills[name] = true
			profs.Skills = append(profs.Skills, name)
		}
	}

	return profs
}
