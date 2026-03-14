package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// ParsePreparedSpells extracts the prepared spell list from character_data JSONB.
// Returns nil if no prepared spells are set.
func ParsePreparedSpells(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsing character_data: %w", err)
	}
	spellsRaw, ok := m["prepared_spells"]
	if !ok {
		return nil, nil
	}
	var spells []string
	if err := json.Unmarshal(spellsRaw, &spells); err != nil {
		return nil, fmt.Errorf("parsing prepared_spells: %w", err)
	}
	return spells, nil
}

// BuildCharacterDataWithPreparedSpells merges prepared spells into existing character_data JSONB.
// Preserves all other keys in the character_data object.
func BuildCharacterDataWithPreparedSpells(existing json.RawMessage, spells []string) (json.RawMessage, error) {
	m := make(map[string]json.RawMessage)
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &m); err != nil {
			return nil, fmt.Errorf("parsing existing character_data: %w", err)
		}
	}
	spellsJSON, err := json.Marshal(spells)
	if err != nil {
		return nil, fmt.Errorf("marshaling prepared_spells: %w", err)
	}
	m["prepared_spells"] = spellsJSON
	result, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshaling character_data: %w", err)
	}
	return result, nil
}

// IsPreparedCaster returns true if the class is a prepared caster (Cleric, Druid, Paladin).
func IsPreparedCaster(className string) bool {
	switch strings.ToLower(className) {
	case "cleric", "druid", "paladin":
		return true
	default:
		return false
	}
}

// alwaysPreparedBySubclass maps class:subclass to a list of {classLevel, spellIDs} entries.
// Spells are granted when the character reaches the specified class level.
var alwaysPreparedBySubclass = map[string][]struct {
	Level    int
	SpellIDs []string
}{
	"cleric:life": {
		{1, []string{"bless", "cure-wounds"}},
		{3, []string{"lesser-restoration", "spiritual-weapon"}},
		{5, []string{"beacon-of-hope", "revivify"}},
		{7, []string{"death-ward", "guardian-of-faith"}},
		{9, []string{"mass-cure-wounds", "raise-dead"}},
	},
	"paladin:devotion": {
		{3, []string{"protection-from-evil-and-good", "sanctuary"}},
		{5, []string{"lesser-restoration", "zone-of-truth"}},
		{9, []string{"beacon-of-hope", "dispel-magic"}},
		{13, []string{"freedom-of-movement", "guardian-of-faith"}},
		{17, []string{"commune", "flame-strike"}},
	},
	"druid:land": {
		{3, []string{"hold-person", "spike-growth"}},
		{5, []string{"sleet-storm", "slow"}},
		{7, []string{"freedom-of-movement", "ice-storm"}},
		{9, []string{"commune-with-nature", "cone-of-cold"}},
	},
}

// AlwaysPreparedSpells returns the spell IDs that are always prepared for the given
// class, subclass, and class level. Returns nil if no always-prepared spells apply.
func AlwaysPreparedSpells(className, subclass string, classLevel int) []string {
	key := strings.ToLower(className) + ":" + strings.ToLower(subclass)
	entries, ok := alwaysPreparedBySubclass[key]
	if !ok {
		return nil
	}

	var result []string
	for _, entry := range entries {
		if classLevel < entry.Level {
			break
		}
		result = append(result, entry.SpellIDs...)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// MaxPreparedSpells returns the maximum number of spells a prepared caster can prepare.
// Formula: ability modifier + class level, minimum 1.
func MaxPreparedSpells(abilityMod, classLevel int) int {
	total := abilityMod + classLevel
	if total < 1 {
		return 1
	}
	return total
}

// AvailableSlotLevels returns a set of spell levels for which the character has
// max slots > 0 (regardless of how many are currently expended).
func AvailableSlotLevels(slots map[int]SlotInfo) map[int]bool {
	result := make(map[int]bool)
	for level, info := range slots {
		if info.Max > 0 {
			result[level] = true
		}
	}
	return result
}

// toStringSet converts a slice of strings to a set (map[string]bool).
func toStringSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

// countNonAlwaysPrepared counts how many spells in selected are not in alwaysSet.
func countNonAlwaysPrepared(selected []string, alwaysSet map[string]bool) int {
	count := 0
	for _, id := range selected {
		if !alwaysSet[id] {
			count++
		}
	}
	return count
}

// ValidateSpellPreparation validates a prepared spell list.
// Always-prepared spells (alwaysIDs) are excluded from the max count.
// All selected spells must be on the class spell list and within available slot levels.
func ValidateSpellPreparation(selected []string, maxPrepared int, slotLevels map[int]bool, classSpells []refdata.Spell, alwaysIDs []string) error {
	classSpellMap := make(map[string]int32, len(classSpells))
	for _, s := range classSpells {
		classSpellMap[s.ID] = s.Level
	}

	alwaysSet := toStringSet(alwaysIDs)

	counted := 0
	for _, spellID := range selected {
		level, ok := classSpellMap[spellID]
		if !ok {
			return fmt.Errorf("%q is not on your class spell list", spellID)
		}
		if level > 0 && !slotLevels[int(level)] {
			return fmt.Errorf("no spell slots of level %d available for %q", level, spellID)
		}
		if !alwaysSet[spellID] {
			counted++
		}
	}

	if counted > maxPrepared {
		return fmt.Errorf("too many spells prepared: %d selected, maximum %d", counted, maxPrepared)
	}

	return nil
}

// PreparationInfo holds the data needed to render the /prepare UI.
type PreparationInfo struct {
	MaxPrepared         int
	CurrentPrepared     []string
	ClassSpells         []refdata.Spell
	AlwaysPrepared      []string
	AvailableSlotLevels map[int]bool
}

// preparationContext holds the resolved character data shared by
// GetPreparationInfo and PrepareSpells.
type preparationContext struct {
	char        refdata.Character
	classLevel  int
	maxPrepared int
	slotLevels  map[int]bool
	classSpells []refdata.Spell
	alwaysIDs   []string
}

// resolvePreparationContext loads and parses the character data needed for
// spell preparation operations.
func (s *Service) resolvePreparationContext(ctx context.Context, charID uuid.UUID, className, subclass string) (preparationContext, error) {
	char, err := s.store.GetCharacter(ctx, charID)
	if err != nil {
		return preparationContext{}, fmt.Errorf("getting character: %w", err)
	}

	var classes []CharacterClass
	if err := json.Unmarshal(char.Classes, &classes); err != nil {
		return preparationContext{}, fmt.Errorf("parsing classes: %w", err)
	}

	classLevel := 0
	for _, c := range classes {
		if strings.EqualFold(c.Class, className) {
			classLevel = c.Level
			break
		}
	}
	if classLevel == 0 {
		return preparationContext{}, fmt.Errorf("character has no levels in %s", className)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return preparationContext{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	abilityMod := AbilityModifier(scores.ScoreByName(SpellcastingAbilityForClass(className)))
	maxPrepared := MaxPreparedSpells(abilityMod, classLevel)

	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return preparationContext{}, fmt.Errorf("parsing spell slots: %w", err)
	}
	slotLevels := AvailableSlotLevels(slots)

	classSpells, err := s.store.ListSpellsByClass(ctx, strings.ToLower(className))
	if err != nil {
		return preparationContext{}, fmt.Errorf("listing class spells: %w", err)
	}

	return preparationContext{
		char:        char,
		classLevel:  classLevel,
		maxPrepared: maxPrepared,
		slotLevels:  slotLevels,
		classSpells: classSpells,
		alwaysIDs:   AlwaysPreparedSpells(className, subclass, classLevel),
	}, nil
}

// GetPreparationInfo returns the current preparation state for a character,
// used to render the ephemeral /prepare message.
func (s *Service) GetPreparationInfo(ctx context.Context, charID uuid.UUID, className, subclass string) (PreparationInfo, error) {
	pc, err := s.resolvePreparationContext(ctx, charID, className, subclass)
	if err != nil {
		return PreparationInfo{}, err
	}

	// Filter class spells to only include spells at available slot levels
	filtered := make([]refdata.Spell, 0, len(pc.classSpells))
	for _, sp := range pc.classSpells {
		if sp.Level == 0 || pc.slotLevels[int(sp.Level)] {
			filtered = append(filtered, sp)
		}
	}

	currentPrepared, err := ParsePreparedSpells(pc.char.CharacterData.RawMessage)
	if err != nil {
		return PreparationInfo{}, fmt.Errorf("parsing prepared spells: %w", err)
	}

	return PreparationInfo{
		MaxPrepared:         pc.maxPrepared,
		CurrentPrepared:     currentPrepared,
		ClassSpells:         filtered,
		AlwaysPrepared:      pc.alwaysIDs,
		AvailableSlotLevels: pc.slotLevels,
	}, nil
}

// LongRestPrepareReminder returns a reminder message for prepared casters after a long rest.
// Returns empty string for non-prepared casters.
func LongRestPrepareReminder(classes []CharacterClass) string {
	for _, c := range classes {
		if IsPreparedCaster(c.Class) {
			return "You can change your prepared spells with `/prepare`."
		}
	}
	return ""
}

// FormatPreparationMessage produces the ephemeral message content for /prepare.
func FormatPreparationMessage(charName string, info PreparationInfo) string {
	var b strings.Builder

	alwaysSet := toStringSet(info.AlwaysPrepared)
	preparedSet := toStringSet(info.CurrentPrepared)
	counted := countNonAlwaysPrepared(info.CurrentPrepared, alwaysSet)

	fmt.Fprintf(&b, "**%s — Spell Preparation**\n", charName)
	fmt.Fprintf(&b, "**%d / %d** spells prepared\n\n", counted, info.MaxPrepared)

	if len(info.AlwaysPrepared) > 0 {
		b.WriteString("**Always Prepared** (subclass, do not count against limit):\n")
		for _, id := range info.AlwaysPrepared {
			fmt.Fprintf(&b, "  - %s\n", id)
		}
		b.WriteString("\n")
	}

	if counted > 0 {
		b.WriteString("**Currently Prepared:**\n")
		for _, id := range info.CurrentPrepared {
			if !alwaysSet[id] {
				fmt.Fprintf(&b, "  - %s\n", id)
			}
		}
		b.WriteString("\n")
	}

	byLevel := make(map[int32][]refdata.Spell)
	for _, sp := range info.ClassSpells {
		if sp.Level > 0 {
			byLevel[sp.Level] = append(byLevel[sp.Level], sp)
		}
	}

	for level := int32(1); level <= 9; level++ {
		spells, ok := byLevel[level]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "**Level %d Spells:**\n", level)
		for _, sp := range spells {
			marker := " "
			if preparedSet[sp.ID] || alwaysSet[sp.ID] {
				marker = "x"
			}
			fmt.Fprintf(&b, "  [%s] %s (%s)\n", marker, sp.ID, sp.School)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// PrepareSpellsInput holds the inputs for the PrepareSpells service method.
type PrepareSpellsInput struct {
	CharacterID uuid.UUID
	ClassName   string
	Subclass    string
	Selected    []string // spell IDs the player selected
}

// PrepareSpellsResult holds the outcome of a spell preparation.
type PrepareSpellsResult struct {
	PreparedCount  int      // number of non-always-prepared spells
	MaxPrepared    int      // maximum allowed
	AlwaysPrepared []string // always-prepared spell IDs (subclass)
}

// PrepareSpells validates and saves a character's prepared spell list.
func (s *Service) PrepareSpells(ctx context.Context, input PrepareSpellsInput) (PrepareSpellsResult, error) {
	if !IsPreparedCaster(input.ClassName) {
		return PrepareSpellsResult{}, fmt.Errorf("%s is not a prepared caster", input.ClassName)
	}

	pc, err := s.resolvePreparationContext(ctx, input.CharacterID, input.ClassName, input.Subclass)
	if err != nil {
		return PrepareSpellsResult{}, err
	}

	if err := ValidateSpellPreparation(input.Selected, pc.maxPrepared, pc.slotLevels, pc.classSpells, pc.alwaysIDs); err != nil {
		return PrepareSpellsResult{}, err
	}

	newData, err := BuildCharacterDataWithPreparedSpells(pc.char.CharacterData.RawMessage, input.Selected)
	if err != nil {
		return PrepareSpellsResult{}, fmt.Errorf("building character data: %w", err)
	}

	if _, err := s.store.UpdateCharacterData(ctx, refdata.UpdateCharacterDataParams{
		ID:            pc.char.ID,
		CharacterData: pqtype.NullRawMessage{RawMessage: newData, Valid: true},
	}); err != nil {
		return PrepareSpellsResult{}, fmt.Errorf("updating character data: %w", err)
	}

	alwaysSet := toStringSet(pc.alwaysIDs)
	counted := countNonAlwaysPrepared(input.Selected, alwaysSet)

	return PrepareSpellsResult{
		PreparedCount:  counted,
		MaxPrepared:    pc.maxPrepared,
		AlwaysPrepared: pc.alwaysIDs,
	}, nil
}
