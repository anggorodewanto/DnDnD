package portal

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

var (
	// ErrNotOwner is returned when the requesting user does not own the character.
	ErrNotOwner = errors.New("not the character owner")
	// ErrCharacterNotFound is returned when the character does not exist.
	ErrCharacterNotFound = errors.New("character not found")
)

// CharacterSheetData holds all data needed to render a full character sheet.
type CharacterSheetData struct {
	ID               string
	Name             string
	Race             string
	Level            int
	Classes          []character.ClassEntry
	AbilityScores    character.AbilityScores
	HpMax            int
	HpCurrent        int
	TempHP           int
	AC               int
	ACFormula        string
	SpeedFt          int
	ProficiencyBonus int
	EquippedMainHand EquippedSlot
	EquippedOffHand  EquippedSlot
	EquippedArmor    EquippedSlot
	SpellSlots       map[string]character.SlotInfo
	SortedSlotLevels []string // numerically sorted keys of SpellSlots
	PactMagicSlots   *character.PactMagicSlots
	HitDiceRemaining map[string]int
	FeatureUses      map[string]character.FeatureUse
	Features         []character.Feature
	Proficiencies    character.Proficiencies
	Gold             int
	AttunementSlots  []character.AttunementSlot
	Languages        []string
	Inventory        []InventoryDisplayItem
	// WeaponMasteries are the weapon masteries the character has chosen (2024
	// rules), resolved by enrichEquipment from the weapon ids stored in
	// character_data. Empty hides the sheet section.
	WeaponMasteries []WeaponMasteryDisplay
	Conditions      []string // active conditions (poisoned, frightened, etc.)
	ExhaustionLevel int
	ConcentrationOn string // spell name if concentrating, empty otherwise

	// Optional free-form description (display-only flavor). Empty when the
	// player wrote none; the template hides the section in that case.
	Appearance string
	Backstory  string

	Spells []SpellDisplayEntry

	// Computed fields for display
	AbilityModifiers map[string]int
	Skills           []SkillDisplay
	SavingThrows     []SavingThrowDisplay
	ClassSummary     string

	// PossibleActions is the reference list of what this character can do on a
	// turn, grouped by action economy and derived from the canonical action
	// catalog (refdata.ActionCatalog) filtered by the character's classes. It is
	// guidance ("what CAN you do?"), not live turn tracking — the read-only
	// sheet has no encounter/Turn context. Computed in renderSheet (template
	// prep) so it is set on every render path.
	PossibleActions []ActionGroup

	// masteryWeaponIDs carries the chosen weapon-mastery ids from
	// mapCharacterToSheet to enrichEquipment, which resolves them against
	// refdata into WeaponMasteries. Not rendered directly.
	masteryWeaponIDs []string
}

// ActionGroup is a set of available actions sharing one action-economy slot,
// rendered as one labelled block on the sheet.
type ActionGroup struct {
	Economy string // display label: "Actions", "Bonus Actions", "Reactions", "Other"
	Actions []ActionDisplay
}

// ActionDisplay is one row in the "Possible Actions" section.
type ActionDisplay struct {
	Name    string
	Command string
	Summary string
	// Classes is the human-readable gating class(es) for a class-specific
	// action (e.g. "Barbarian", "Cleric / Paladin"); empty when universal.
	Classes string
}

// SpellDisplayEntry holds data for displaying a single spell on the character sheet.
type SpellDisplayEntry struct {
	ID            string
	Name          string
	Level         int
	School        string
	CastingTime   string
	Range         string
	Components    []string
	Duration      string
	Description   string
	Concentration bool
	Prepared      bool
	Source        string
	Homebrew      bool
	OffList       bool
}

// SkillDisplay holds display data for a single skill.
type SkillDisplay struct {
	Name       string
	Ability    string
	Modifier   int
	Proficient bool
}

// SavingThrowDisplay holds display data for a single saving throw.
type SavingThrowDisplay struct {
	Ability    string
	Modifier   int
	Proficient bool
}

// CharacterSheetStore abstracts the persistence layer for character sheet data.
type CharacterSheetStore interface {
	GetCharacterForSheet(ctx context.Context, characterID string) (*CharacterSheetData, error)
	GetCharacterOwner(ctx context.Context, characterID string) (string, error)
	// CanViewCharacter reports whether a non-owner may view the sheet: true for
	// the campaign DM or any non-retired player in the character's campaign.
	CanViewCharacter(ctx context.Context, characterID, requestingUserID string) (bool, error)
}

// CharacterSheetService loads character data for the sheet view.
type CharacterSheetService struct {
	store CharacterSheetStore
}

// NewCharacterSheetService creates a new CharacterSheetService.
func NewCharacterSheetService(store CharacterSheetStore) *CharacterSheetService {
	return &CharacterSheetService{store: store}
}

// LoadCharacterSheet loads and enriches character data for display. The owner
// always has access; otherwise the campaign DM and any player in the
// character's campaign may view it. Returns ErrNotOwner when the requesting
// user qualifies for none of those.
func (svc *CharacterSheetService) LoadCharacterSheet(ctx context.Context, characterID, requestingUserID string) (*CharacterSheetData, error) {
	ownerID, err := svc.store.GetCharacterOwner(ctx, characterID)
	if err != nil {
		return nil, err
	}

	if ownerID != requestingUserID {
		canView, err := svc.store.CanViewCharacter(ctx, characterID, requestingUserID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, ErrNotOwner
		}
	}

	data, err := svc.store.GetCharacterForSheet(ctx, characterID)
	if err != nil {
		return nil, err
	}

	enrichCharacterSheet(data)
	return data, nil
}

// enrichCharacterSheet computes derived display fields.
func enrichCharacterSheet(data *CharacterSheetData) {
	data.AbilityModifiers = computeAbilityModifiers(data.AbilityScores)
	data.Skills = computeSkills(data.AbilityScores, data.Proficiencies, data.ProficiencyBonus)
	data.SavingThrows = computeSavingThrows(data.AbilityScores, data.Proficiencies.Saves, data.ProficiencyBonus)
	data.ClassSummary = character.FormatClassSummary(data.Classes)
}

// actionEconomyLabels maps a catalog economy to its sheet section label.
var actionEconomyLabels = map[refdata.ActionEconomy]string{
	refdata.EconomyAction:      "Actions",
	refdata.EconomyBonusAction: "Bonus Actions",
	refdata.EconomyReaction:    "Reactions",
	refdata.EconomyFree:        "Other",
}

// buildActionGroups filters the canonical action catalog by the character's
// classes and groups the result by action economy in display order. Only
// non-empty groups are returned. The grouping is deterministic: economy order
// follows refdata.ActionEconomyOrder and within a group the catalog's
// declaration order is preserved.
func buildActionGroups(classes []character.ClassEntry) []ActionGroup {
	available := refdata.AvailableActions(classLevelMap(classes))

	byEconomy := make(map[refdata.ActionEconomy][]ActionDisplay)
	for _, e := range available {
		byEconomy[e.Economy] = append(byEconomy[e.Economy], ActionDisplay{
			Name:    e.Name,
			Command: e.Command,
			Summary: e.Summary,
			Classes: formatGatingClasses(e.Classes),
		})
	}

	groups := make([]ActionGroup, 0, len(refdata.ActionEconomyOrder))
	for _, economy := range refdata.ActionEconomyOrder {
		actions := byEconomy[economy]
		if len(actions) == 0 {
			continue
		}
		groups = append(groups, ActionGroup{
			Economy: actionEconomyLabels[economy],
			Actions: actions,
		})
	}
	return groups
}

// classLevelMap reduces the character's classes to a class-name → highest-level
// map for action gating. Case folding is owned by refdata.AvailableActions (the
// public consumer), so it is not repeated here.
func classLevelMap(classes []character.ClassEntry) map[string]int {
	levels := make(map[string]int, len(classes))
	for _, c := range classes {
		if c.Level > levels[c.Class] {
			levels[c.Class] = c.Level
		}
	}
	return levels
}

// formatGatingClasses renders gating class slugs as a capitalized,
// slash-joined label (e.g. "Cleric / Paladin"). Empty for universal actions.
func formatGatingClasses(classes []string) string {
	if len(classes) == 0 {
		return ""
	}
	titled := make([]string, len(classes))
	for i, c := range classes {
		titled[i] = formatSkillName(c) // capitalizes each hyphen-separated word
	}
	return strings.Join(titled, " / ")
}

func computeAbilityModifiers(scores character.AbilityScores) map[string]int {
	return map[string]int{
		"STR": character.AbilityModifier(scores.STR),
		"DEX": character.AbilityModifier(scores.DEX),
		"CON": character.AbilityModifier(scores.CON),
		"INT": character.AbilityModifier(scores.INT),
		"WIS": character.AbilityModifier(scores.WIS),
		"CHA": character.AbilityModifier(scores.CHA),
	}
}

func computeSkills(scores character.AbilityScores, profs character.Proficiencies, profBonus int) []SkillDisplay {
	skillNames := make([]string, 0, len(character.SkillAbilityMap))
	for name := range character.SkillAbilityMap {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)

	skills := make([]SkillDisplay, 0, len(skillNames))
	for _, name := range skillNames {
		ability := character.SkillAbilityMap[name]
		mod := character.SkillModifier(scores, name, profs.Skills, profs.Expertise, profs.JackOfAllTrades, profBonus)
		proficient := slices.Contains(profs.Skills, name)
		skills = append(skills, SkillDisplay{
			Name:       formatSkillName(name),
			Ability:    strings.ToUpper(ability),
			Modifier:   mod,
			Proficient: proficient,
		})
	}
	return skills
}

func formatSkillName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func computeSavingThrows(scores character.AbilityScores, profSaves []string, profBonus int) []SavingThrowDisplay {
	abilities := []string{"str", "dex", "con", "int", "wis", "cha"}
	throws := make([]SavingThrowDisplay, 0, len(abilities))
	for _, ab := range abilities {
		mod := character.SavingThrowModifier(scores, ab, profSaves, profBonus)
		proficient := slices.Contains(profSaves, ab)
		throws = append(throws, SavingThrowDisplay{
			Ability:    strings.ToUpper(ab),
			Modifier:   mod,
			Proficient: proficient,
		})
	}
	return throws
}
