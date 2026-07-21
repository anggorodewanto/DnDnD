package portal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// DerivedStats holds the calculated statistics for a character submission.
// It is used both for the live preview endpoint and to populate the stored
// character record at creation time.
type DerivedStats struct {
	HPMax             int            `json:"hp_max"`
	AC                int            `json:"ac"`
	SpeedFt           int            `json:"speed_ft"`
	TotalLevel        int            `json:"total_level"`
	ProficiencyBonus  int            `json:"proficiency_bonus"`
	SaveProficiencies []string       `json:"save_proficiencies"`
	Saves             map[string]int `json:"saves"`
	Skills            map[string]int `json:"skills"`
	HitDiceRemaining  map[string]int `json:"hit_dice_remaining"`
	SpellSlots        map[int]int    `json:"spell_slots,omitempty"`
	// PactMagicSlots holds Warlock pact-magic slots when the character has any
	// pact-progression levels; nil otherwise.
	PactMagicSlots *character.PactMagicSlots `json:"pact_magic_slots,omitempty"`
	MaxSpellLevel  int                       `json:"max_spell_level"`
	Features       []character.Feature       `json:"features,omitempty"`
}

// FeatureProvider supplies class/subclass/racial feature data for creation.
type FeatureProvider interface {
	ClassFeatures() map[string]map[string][]character.Feature
	SubclassFeatures() map[string]map[string]map[string][]character.Feature
	RacialTraits(race string) []character.Feature
}

// SubmissionClasses returns the normalized multiclass list for a submission.
// When the Classes slice is empty it falls back to a single level-1 entry
// built from the legacy Class/Subclass fields.
func SubmissionClasses(sub CharacterSubmission) []character.ClassEntry {
	if hasNonEmptyClass(sub.Classes) {
		return sub.Classes
	}
	if sub.Class == "" {
		return nil
	}
	return []character.ClassEntry{{Class: sub.Class, Subclass: sub.Subclass, Level: 1, IsPrimary: true}}
}

// primaryClassEntry returns the entry flagged IsPrimary, or the first entry,
// or nil when the list is empty.
func primaryClassEntry(classes []character.ClassEntry) *character.ClassEntry {
	if len(classes) == 0 {
		return nil
	}
	for i := range classes {
		if classes[i].IsPrimary {
			return &classes[i]
		}
	}
	return &classes[0]
}

// DeriveStats calculates derived statistics from a character submission. An
// optional raceSpeeds map (from the DB) overrides the hardcoded speed table.
func DeriveStats(sub CharacterSubmission, raceSpeeds map[string]int) DerivedStats {
	classes := SubmissionClasses(sub)
	scores := sub.AbilityScores.Character()
	totalLevel := character.TotalLevel(classes)
	profBonus := character.ProficiencyBonus(totalLevel)

	hitDice := make(map[string]string, len(classes))
	for _, c := range classes {
		hitDice[c.Class] = ClassHitDie(c.Class)
	}
	hp := character.CalculateHP(classes, hitDice, scores)

	armor := lookupArmorInfo(sub.WornArmor)
	hasShield := hasEquipmentItem(sub.Equipment, "shield")
	acFormula := unarmoredDefenseFormula(classes, sub.WornArmor, hasShield, hasArmorOfShadowsInvocation(sub.Invocations))
	ac := character.CalculateAC(scores, armor, hasShield, acFormula)

	saveProficiencies := classSaveProficiencies(classes)
	saves := make(map[string]int, 6)
	for _, ab := range []string{"str", "dex", "con", "int", "wis", "cha"} {
		saves[ab] = character.SavingThrowModifier(scores, ab, saveProficiencies, profBonus)
	}

	skillProfs := append(classSkillProficiencies(classes), backgroundSkillProficiencies(sub.Background)...)
	skills := make(map[string]int, len(character.SkillAbilityMap))
	for skill := range character.SkillAbilityMap {
		skills[skill] = character.SkillModifier(scores, skill, skillProfs, nil, false, profBonus)
	}

	// Key by die string (e.g. "d12"), not class name: the rest flow and the
	// hit-dice buttons look up HitDieValue by die string. Classes sharing a
	// die (fighter+paladin => d10) accumulate under the one key.
	hitDiceRemaining := make(map[string]int, len(classes))
	for _, c := range classes {
		hitDiceRemaining[ClassHitDie(c.Class)] += c.Level
	}

	spellSlots := character.CalculateSpellSlots(classes, classSpellcastingMap(classes))
	maxSpellLevel := 0
	for lvl := range spellSlots {
		if lvl > maxSpellLevel {
			maxSpellLevel = lvl
		}
	}

	pactSlots := pactMagicSlotsForClasses(classes)
	if pactSlots != nil && pactSlots.SlotLevel > maxSpellLevel {
		maxSpellLevel = pactSlots.SlotLevel
	}

	return DerivedStats{
		HPMax:             hp,
		AC:                ac,
		SpeedFt:           raceSpeedWithLookup(sub.Race, raceSpeeds),
		TotalLevel:        totalLevel,
		ProficiencyBonus:  profBonus,
		SaveProficiencies: saveProficiencies,
		Saves:             saves,
		Skills:            skills,
		HitDiceRemaining:  hitDiceRemaining,
		SpellSlots:        spellSlots,
		PactMagicSlots:    pactSlots,
		MaxSpellLevel:     maxSpellLevel,
	}
}

// pactMagicSlotsForClasses returns the Warlock pact-magic slots for the total
// pact-progression level across the class list, or nil when no class uses pact
// progression. Pact levels stack (multiple pact classes are not a real 5e
// scenario, but summing keeps the table lookup well-defined).
func pactMagicSlotsForClasses(classes []character.ClassEntry) *character.PactMagicSlots {
	pactLevel := 0
	for _, c := range classes {
		if classSpellcasting(c.Class).SlotProgression == "pact" {
			pactLevel += c.Level
		}
	}
	if pactLevel < 1 {
		return nil
	}
	slots := character.PactMagicSlotsForLevel(pactLevel)
	return &slots
}

// spellSlotsForClasses returns the standard (non-pact) spell slots for the
// class list in the canonical stored shape: string-keyed slot levels mapping
// to {current,max} objects (character.SlotInfo). This matches what the play
// read path (combat.ParseSpellSlots / parseIntKeyedSlots) and the dashboard
// character sheet expect. Returns nil for non-casters (empty slot map) so the
// caller can leave spell_slots NULL. A freshly created caster starts with all
// slots full (current == max).
func spellSlotsForClasses(classes []character.ClassEntry) map[string]character.SlotInfo {
	slots := character.CalculateSpellSlots(classes, classSpellcastingMap(classes))
	if len(slots) == 0 {
		return nil
	}
	out := make(map[string]character.SlotInfo, len(slots))
	for level, count := range slots {
		out[strconv.Itoa(level)] = character.SlotInfo{Current: count, Max: count}
	}
	return out
}

// unarmoredDefenseFormula returns the ac_formula string for a class list's
// Unarmored Defense, or "" when none applies. The returned strings are the
// exact forms character.CalculateAC / combat.RecalculateAC parse ("10 + DEX +
// CON", "10 + DEX + WIS") — NOT the seed mechanical_effect labels
// ("ac_10_plus_dex_plus_con"), which are only used for feature definitions.
//
// Rules (SRD):
//   - Barbarian: 10 + DEX + CON while wearing no armor (a shield still applies).
//   - Monk: 10 + DEX + WIS while wearing no armor AND wielding no shield.
//
// A multiclass barbarian+monk prefers the barbarian formula because it is
// shield-compatible; choosing monk would silently void the bonus the moment a
// shield is equipped.
//
// hasArmorOfShadows carries the Warlock "Armor of Shadows" invocation, which
// grants Mage Armor at will (13 + DEX while wearing no armor). It is the weakest
// of these unarmored sources, so it only applies when no barbarian/monk formula
// does; a barbarian/monk who also has the invocation keeps the stronger formula.
func unarmoredDefenseFormula(classes []character.ClassEntry, wornArmor string, hasShield, hasArmorOfShadows bool) string {
	if wornArmor != "" {
		return ""
	}
	hasBarbarian := false
	hasMonk := false
	for _, c := range classes {
		switch strings.ToLower(c.Class) {
		case "barbarian":
			hasBarbarian = true
		case "monk":
			hasMonk = true
		}
	}
	if hasBarbarian {
		return "10 + DEX + CON"
	}
	if hasMonk && !hasShield {
		return "10 + DEX + WIS"
	}
	if hasArmorOfShadows {
		return "13 + DEX"
	}
	return ""
}

// hasArmorOfShadowsInvocation reports whether the Warlock "Armor of Shadows"
// eldritch invocation (Mage Armor at will) is among the chosen invocations.
func hasArmorOfShadowsInvocation(invocations []string) bool {
	for _, inv := range invocations {
		if strings.EqualFold(inv, "armor_of_shadows") {
			return true
		}
	}
	return false
}

// CollectFeatures gathers features from racial traits, class features, and
// subclass features up to each class's current level. Racial traits appear
// first, then class features in order.
func CollectFeatures(
	classes []character.ClassEntry,
	classFeatures map[string]map[string][]character.Feature,
	subclassFeatures map[string]map[string]map[string][]character.Feature,
	racialTraits []character.Feature,
) []character.Feature {
	var result []character.Feature
	result = append(result, racialTraits...)

	for _, c := range classes {
		classKey := strings.ToLower(c.Class)
		cf := classFeatures[classKey] // nil map is safe to read from

		for lvl := 1; lvl <= c.Level; lvl++ {
			key := fmt.Sprintf("%d", lvl)
			result = append(result, cf[key]...)

			if c.Subclass == "" || subclassFeatures == nil {
				continue
			}
			scMap, ok := subclassFeatures[classKey]
			if !ok {
				continue
			}
			scFeats, ok := scMap[strings.ToLower(c.Subclass)]
			if !ok {
				continue
			}
			result = append(result, scFeats[key]...)
		}
	}

	return result
}

// MaxSpellLevelForClasses returns the highest spell slot level available to
// the character, or 0 when the character has no spellcasting.
func MaxSpellLevelForClasses(classes []character.ClassEntry) int {
	slots := character.CalculateSpellSlots(classes, classSpellcastingMap(classes))
	maxLvl := 0
	for lvl := range slots {
		if lvl > maxLvl {
			maxLvl = lvl
		}
	}
	return maxLvl
}

// raceSpeedWithLookup returns the base walking speed in feet for a race,
// preferring a DB-sourced map when provided and falling back to the SRD table.
func raceSpeedWithLookup(race string, dbSpeeds map[string]int) int {
	lower := strings.ToLower(race)
	if dbSpeeds != nil {
		if speed, ok := dbSpeeds[lower]; ok {
			return speed
		}
	}
	switch lower {
	case "dwarf", "hill dwarf", "mountain dwarf",
		"halfling", "lightfoot halfling", "stout halfling",
		"gnome", "forest gnome", "rock gnome":
		return 25
	case "wood elf":
		return 35
	default:
		return 30
	}
}

// classSkillProficiencies returns default skill proficiencies for the primary
// (first) class — the SRD defaults granted by each class.
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

// backgroundSkillProficiencies returns the skills granted by a background.
//
// The lookup table (backgroundSkillsByID) is GENERATED from the canonical
// portal/svelte/src/lib/backgrounds.json — the same source the Svelte builder
// reads — so the two sides cannot drift (the drift between hand-maintained
// copies caused the guild-artisan / folk-hero submit 400; see ISSUE-013). Edit
// the JSON + regenerate, never this table. Slugs are lowercase kebab-case.
func backgroundSkillProficiencies(background string) []string {
	return backgroundSkillsByID[strings.ToLower(background)]
}

// classSaveProficiencies returns save proficiencies for the primary class.
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

// classSpellcastingMap returns spellcasting data for each spellcasting class.
func classSpellcastingMap(classes []character.ClassEntry) map[string]character.ClassSpellcasting {
	m := make(map[string]character.ClassSpellcasting)
	for _, c := range classes {
		sc := classSpellcasting(c.Class)
		if sc.SlotProgression != "none" {
			m[c.Class] = sc
		}
	}
	return m
}

// classSpellcasting returns the spellcasting data for a class name.
func classSpellcasting(className string) character.ClassSpellcasting {
	switch strings.ToLower(className) {
	case "bard":
		return character.ClassSpellcasting{SpellAbility: "cha", SlotProgression: "full"}
	case "cleric":
		return character.ClassSpellcasting{SpellAbility: "wis", SlotProgression: "full"}
	case "druid":
		return character.ClassSpellcasting{SpellAbility: "wis", SlotProgression: "full"}
	case "sorcerer":
		return character.ClassSpellcasting{SpellAbility: "cha", SlotProgression: "full"}
	case "wizard":
		return character.ClassSpellcasting{SpellAbility: "int", SlotProgression: "full"}
	case "paladin":
		return character.ClassSpellcasting{SpellAbility: "cha", SlotProgression: "half"}
	case "ranger":
		return character.ClassSpellcasting{SpellAbility: "wis", SlotProgression: "half"}
	case "eldritch knight", "arcane trickster":
		return character.ClassSpellcasting{SpellAbility: "int", SlotProgression: "third"}
	case "warlock":
		return character.ClassSpellcasting{SpellAbility: "cha", SlotProgression: "pact"}
	default:
		return character.ClassSpellcasting{SlotProgression: "none"}
	}
}

// armorTable maps armor IDs to their ArmorInfo for AC calculation.
var armorTable = map[string]character.ArmorInfo{
	"padded":          {ACBase: 11, DexBonus: true},
	"leather":         {ACBase: 11, DexBonus: true},
	"studded-leather": {ACBase: 12, DexBonus: true},
	"hide":            {ACBase: 12, DexBonus: true, DexMax: 2},
	"chain-shirt":     {ACBase: 13, DexBonus: true, DexMax: 2},
	"scale-mail":      {ACBase: 14, DexBonus: true, DexMax: 2},
	"breastplate":     {ACBase: 14, DexBonus: true, DexMax: 2},
	"half-plate":      {ACBase: 15, DexBonus: true, DexMax: 2},
	"ring-mail":       {ACBase: 14},
	"chain-mail":      {ACBase: 16},
	"splint":          {ACBase: 17},
	"plate":           {ACBase: 18},
}

// lookupArmorInfo returns the ArmorInfo for an armor ID, or nil when empty/unknown.
func lookupArmorInfo(armorID string) *character.ArmorInfo {
	if armorID == "" {
		return nil
	}
	info, ok := armorTable[strings.ToLower(armorID)]
	if !ok {
		return nil
	}
	return &info
}

// hasEquipmentItem reports whether the equipment list contains the given item ID.
func hasEquipmentItem(equipment []string, itemID string) bool {
	for _, e := range equipment {
		if strings.EqualFold(e, itemID) {
			return true
		}
	}
	return false
}
