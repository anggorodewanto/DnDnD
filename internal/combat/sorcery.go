package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// Metamagic sorcery point costs.
var metamagicCosts = map[string]int{
	"careful":    1,
	"distant":    1,
	"empowered":  1,
	"extended":   1,
	"heightened": 3,
	"quickened":  2,
	"subtle":     1,
	// "twinned" is special: cost = spell level (1 for cantrips)
}

// Slot creation cost table: spell level → sorcery point cost.
var slotCreationCosts = map[int]int{
	1: 2,
	2: 3,
	3: 5,
	4: 6,
	5: 7,
}

// SorceryPointCost returns the sorcery point cost for a metamagic option at the given spell level.
func SorceryPointCost(metamagic string, spellLevel int) (int, error) {
	if metamagic == "twinned" {
		if spellLevel == 0 {
			return 1, nil
		}
		return spellLevel, nil
	}
	cost, ok := metamagicCosts[metamagic]
	if !ok {
		return 0, fmt.Errorf("unknown metamagic option: %q", metamagic)
	}
	return cost, nil
}

// MetamagicTotalCost returns the total sorcery point cost for all metamagic options.
func MetamagicTotalCost(metamagics []string, spellLevel int) (int, error) {
	total := 0
	for _, m := range metamagics {
		cost, err := SorceryPointCost(m, spellLevel)
		if err != nil {
			return 0, err
		}
		total += cost
	}
	return total, nil
}

// ValidateMetamagic validates metamagic options: one-per-spell rule (Empowered can combine),
// checks for unknown options, and verifies sufficient sorcery points.
func ValidateMetamagic(metamagics []string, spellLevel int, sorceryPoints int) error {
	if len(metamagics) == 0 {
		return nil
	}

	// Normalize all inputs to lowercase for consistent comparison
	normalized := make([]string, len(metamagics))
	for i, m := range metamagics {
		normalized[i] = strings.ToLower(m)
	}

	// Check one-per-spell rule: only one metamagic unless one of them is empowered
	if len(normalized) > 1 {
		nonEmpowered := 0
		for _, m := range normalized {
			if m != "empowered" {
				nonEmpowered++
			}
		}
		if nonEmpowered > 1 {
			return fmt.Errorf("only one Metamagic option can be used per spell (Empowered Spell can combine with another)")
		}
	}

	// Validate all options and compute total cost
	totalCost, err := MetamagicTotalCost(normalized, spellLevel)
	if err != nil {
		return err
	}

	if totalCost > sorceryPoints {
		return fmt.Errorf("insufficient sorcery points: need %d, have %d", totalCost, sorceryPoints)
	}

	return nil
}

// SlotCreationCost returns the sorcery point cost to create a spell slot at the given level.
func SlotCreationCost(level int) (int, error) {
	cost, ok := slotCreationCosts[level]
	if !ok {
		return 0, fmt.Errorf("cannot create spell slots above 5th level (requested level %d)", level)
	}
	return cost, nil
}

// FormatFontOfMagicConvert produces the combat log for converting a slot to sorcery points.
func FormatFontOfMagicConvert(name string, slotLevel int, pointsGained int, pointsRemaining int) string {
	return fmt.Sprintf("\U0001f52e %s converts a %s-level spell slot \u2192 %d sorcery points (%d SP remaining)",
		name, ordinal(slotLevel), pointsGained, pointsRemaining)
}

// FormatFontOfMagicCreate produces the combat log for creating a slot from sorcery points.
func FormatFontOfMagicCreate(name string, slotLevel int, pointsBefore int, pointsAfter int) string {
	return fmt.Sprintf("\U0001f52e %s creates a %s-level spell slot (%d SP \u2192 %d SP)",
		name, ordinal(slotLevel), pointsBefore, pointsAfter)
}

// FontOfMagicCommand holds the inputs for a Font of Magic conversion.
type FontOfMagicCommand struct {
	CasterID        uuid.UUID // combatant ID
	Turn            refdata.Turn
	SlotLevel       int // for slot→points: which slot level to expend
	CreateSlotLevel int // for points→slot: which slot level to create
}

// FontOfMagicResult holds the output of a Font of Magic conversion.
type FontOfMagicResult struct {
	PointsRemaining int
	CombatLog       string
	Remaining       string
}

// fontOfMagicContext holds the validated state shared by both Font of Magic operations.
type fontOfMagicContext struct {
	caster        refdata.Combatant
	char          refdata.Character
	sorcLevel     int
	featureUses   map[string]character.FeatureUse
	currentPoints int
}

// validateFontOfMagic performs the shared validation for Font of Magic:
// bonus action availability, player character check, sorcerer level 2+, and sorcery point parsing.
func (s *Service) validateFontOfMagic(ctx context.Context, cmd FontOfMagicCommand) (fontOfMagicContext, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return fontOfMagicContext{}, err
	}

	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return fontOfMagicContext{}, fmt.Errorf("getting caster: %w", err)
	}
	if !caster.CharacterID.Valid {
		return fontOfMagicContext{}, fmt.Errorf("Font of Magic requires a player character")
	}

	char, err := s.store.GetCharacter(ctx, caster.CharacterID.UUID)
	if err != nil {
		return fontOfMagicContext{}, fmt.Errorf("getting character: %w", err)
	}

	sorcLevel := ClassLevelFromJSON(char.Classes, "Sorcerer")
	if sorcLevel < 2 {
		return fontOfMagicContext{}, fmt.Errorf("Font of Magic requires Sorcerer level 2+, have level %d", sorcLevel)
	}

	featureUses, currentPoints, err := ParseFeatureUses(char, FeatureKeySorceryPoints)
	if err != nil {
		return fontOfMagicContext{}, err
	}

	return fontOfMagicContext{
		caster:        caster,
		char:          char,
		sorcLevel:     sorcLevel,
		featureUses:   featureUses,
		currentPoints: currentPoints,
	}, nil
}

// useBonusActionAndPersist consumes a bonus action and persists the turn update.
func (s *Service) useBonusActionAndPersist(ctx context.Context, turn refdata.Turn) (refdata.Turn, error) {
	updated, err := UseResource(turn, ResourceBonusAction)
	if err != nil {
		return refdata.Turn{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updated)); err != nil {
		return refdata.Turn{}, fmt.Errorf("updating turn: %w", err)
	}
	return updated, nil
}

// FontOfMagicConvertSlot converts a spell slot into sorcery points.
func (s *Service) FontOfMagicConvertSlot(ctx context.Context, cmd FontOfMagicCommand) (FontOfMagicResult, error) {
	fom, err := s.validateFontOfMagic(ctx, cmd)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	// Check that gaining points won't exceed max (max = sorcerer level)
	pointsGained := cmd.SlotLevel
	if fom.currentPoints+pointsGained > fom.sorcLevel {
		return FontOfMagicResult{}, fmt.Errorf("conversion would exceed sorcery point maximum (%d + %d > %d)", fom.currentPoints, pointsGained, fom.sorcLevel)
	}

	// Validate and deduct spell slot
	slots, err := parseIntKeyedSlots(fom.char.SpellSlots.RawMessage)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	if err := ValidateSpellSlot(slots, cmd.SlotLevel); err != nil {
		return FontOfMagicResult{}, err
	}
	if _, err := s.deductAndPersistSlot(ctx, fom.char.ID, slots, cmd.SlotLevel); err != nil {
		return FontOfMagicResult{}, err
	}

	// Add sorcery points
	newPoints, err := s.SetFeaturePool(ctx, fom.char, FeatureKeySorceryPoints, fom.featureUses, fom.currentPoints+pointsGained)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	turn, err := s.useBonusActionAndPersist(ctx, cmd.Turn)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	return FontOfMagicResult{
		PointsRemaining: newPoints,
		CombatLog:       FormatFontOfMagicConvert(fom.caster.DisplayName, cmd.SlotLevel, pointsGained, newPoints),
		Remaining:       FormatRemainingResources(turn, nil),
	}, nil
}

// FontOfMagicCreateSlot creates a spell slot from sorcery points.
func (s *Service) FontOfMagicCreateSlot(ctx context.Context, cmd FontOfMagicCommand) (FontOfMagicResult, error) {
	if cmd.CreateSlotLevel < 1 || cmd.CreateSlotLevel > 5 {
		return FontOfMagicResult{}, fmt.Errorf("cannot create slots above 5th level (requested %d)", cmd.CreateSlotLevel)
	}

	cost, err := SlotCreationCost(cmd.CreateSlotLevel)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	fom, err := s.validateFontOfMagic(ctx, cmd)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	if cost > fom.currentPoints {
		return FontOfMagicResult{}, fmt.Errorf("insufficient sorcery points: need %d to create %s-level slot, have %d",
			cost, ordinal(cmd.CreateSlotLevel), fom.currentPoints)
	}

	// Deduct sorcery points
	newPoints, err := s.DeductFeaturePool(ctx, fom.char, FeatureKeySorceryPoints, fom.featureUses, fom.currentPoints, cost)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	// Add spell slot
	if err := s.addAndPersistSlot(ctx, fom.char.ID, fom.char.SpellSlots.RawMessage, cmd.CreateSlotLevel); err != nil {
		return FontOfMagicResult{}, err
	}

	turn, err := s.useBonusActionAndPersist(ctx, cmd.Turn)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	return FontOfMagicResult{
		PointsRemaining: newPoints,
		CombatLog:       FormatFontOfMagicCreate(fom.caster.DisplayName, cmd.CreateSlotLevel, fom.currentPoints, newPoints),
		Remaining:       FormatRemainingResources(turn, nil),
	}, nil
}

// addAndPersistSlot adds one spell slot at the given level and persists the change.
func (s *Service) addAndPersistSlot(ctx context.Context, charID uuid.UUID, slotsRaw json.RawMessage, slotLevel int) error {
	slots, err := parseIntKeyedSlots(slotsRaw)
	if err != nil {
		return err
	}
	info := slots[slotLevel]
	info.Current++
	slots[slotLevel] = info
	slotsJSON, err := json.Marshal(intToStringKeyedSlots(slots))
	if err != nil {
		return fmt.Errorf("marshaling spell slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         charID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return fmt.Errorf("updating spell slots: %w", err)
	}
	return nil
}

// hasMetamagic returns true if the metamagic list contains the given option.
func hasMetamagic(metamagics []string, option string) bool {
	for _, m := range metamagics {
		if strings.EqualFold(m, option) {
			return true
		}
	}
	return false
}
