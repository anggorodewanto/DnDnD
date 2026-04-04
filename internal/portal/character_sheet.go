package portal

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/ab/dndnd/internal/character"
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
	EquippedMainHand string
	EquippedOffHand  string
	EquippedArmor    string
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
	HitDiceRemaining map[string]int
	FeatureUses      map[string]character.FeatureUse
	Features         []character.Feature
	Proficiencies    character.Proficiencies
	Gold             int
	AttunementSlots  []character.AttunementSlot
	Languages        []string
	Inventory        []character.InventoryItem

	Spells []SpellDisplayEntry

	// Computed fields for display
	AbilityModifiers map[string]int
	Skills           []SkillDisplay
	SavingThrows     []SavingThrowDisplay
	ClassSummary     string
}

// SpellDisplayEntry holds data for displaying a single spell on the character sheet.
type SpellDisplayEntry struct {
	ID          string
	Name        string
	Level       int
	School      string
	CastingTime string
	Range       string
	Prepared    bool
	Source      string
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
}

// CharacterSheetService loads character data for the sheet view.
type CharacterSheetService struct {
	store CharacterSheetStore
}

// NewCharacterSheetService creates a new CharacterSheetService.
func NewCharacterSheetService(store CharacterSheetStore) *CharacterSheetService {
	return &CharacterSheetService{store: store}
}

// LoadCharacterSheet loads and enriches character data for display.
// Returns ErrNotOwner if the requesting user does not own the character.
func (svc *CharacterSheetService) LoadCharacterSheet(ctx context.Context, characterID, requestingUserID string) (*CharacterSheetData, error) {
	ownerID, err := svc.store.GetCharacterOwner(ctx, characterID)
	if err != nil {
		return nil, err
	}

	if ownerID != requestingUserID {
		return nil, ErrNotOwner
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
		mod := character.SkillModifier(scores, name, profs.Skills, nil, false, profBonus)
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
