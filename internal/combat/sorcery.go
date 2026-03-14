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

// FontOfMagicConvertSlot converts a spell slot into sorcery points.
func (s *Service) FontOfMagicConvertSlot(ctx context.Context, cmd FontOfMagicCommand) (FontOfMagicResult, error) {
	// 1. Validate bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return FontOfMagicResult{}, err
	}

	// 2. Look up caster
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("getting caster: %w", err)
	}
	if !caster.CharacterID.Valid {
		return FontOfMagicResult{}, fmt.Errorf("Font of Magic requires a player character")
	}

	// 3. Look up character
	char, err := s.store.GetCharacter(ctx, caster.CharacterID.UUID)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("getting character: %w", err)
	}

	// 4. Validate sorcerer level 2+
	sorcLevel := ClassLevelFromJSON(char.Classes, "Sorcerer")
	if sorcLevel < 2 {
		return FontOfMagicResult{}, fmt.Errorf("Font of Magic requires Sorcerer level 2+, have level %d", sorcLevel)
	}

	// 5. Parse feature uses for sorcery points
	featureUses, currentPoints, err := ParseFeatureUses(char, FeatureKeySorceryPoints)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	// 6. Check that gaining points won't exceed max
	maxPoints := sorcLevel
	pointsGained := cmd.SlotLevel
	if currentPoints+pointsGained > maxPoints {
		return FontOfMagicResult{}, fmt.Errorf("conversion would exceed sorcery point maximum (%d + %d > %d)", currentPoints, pointsGained, maxPoints)
	}

	// 7. Parse and validate spell slot
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	if err := ValidateSpellSlot(slots, cmd.SlotLevel); err != nil {
		return FontOfMagicResult{}, err
	}

	// 8. Deduct spell slot
	newSlots := DeductSpellSlot(slots, cmd.SlotLevel)
	slotsJSON, err := json.Marshal(intToStringKeyedSlots(newSlots))
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("marshaling spell slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         char.ID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating spell slots: %w", err)
	}

	// 9. Add sorcery points
	newPoints := currentPoints + pointsGained
	featureUses[FeatureKeySorceryPoints] = newPoints
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	// 10. Use bonus action
	turn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating turn: %w", err)
	}

	return FontOfMagicResult{
		PointsRemaining: newPoints,
		CombatLog:       FormatFontOfMagicConvert(caster.DisplayName, cmd.SlotLevel, pointsGained, newPoints),
		Remaining:       FormatRemainingResources(turn, nil),
	}, nil
}

// FontOfMagicCreateSlot creates a spell slot from sorcery points.
func (s *Service) FontOfMagicCreateSlot(ctx context.Context, cmd FontOfMagicCommand) (FontOfMagicResult, error) {
	// 1. Validate bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return FontOfMagicResult{}, err
	}

	// 2. Validate slot level
	if cmd.CreateSlotLevel < 1 || cmd.CreateSlotLevel > 5 {
		return FontOfMagicResult{}, fmt.Errorf("cannot create slots above 5th level (requested %d)", cmd.CreateSlotLevel)
	}

	cost, err := SlotCreationCost(cmd.CreateSlotLevel)
	if err != nil {
		return FontOfMagicResult{}, err
	}

	// 3. Look up caster
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("getting caster: %w", err)
	}
	if !caster.CharacterID.Valid {
		return FontOfMagicResult{}, fmt.Errorf("Font of Magic requires a player character")
	}

	// 4. Look up character
	char, err := s.store.GetCharacter(ctx, caster.CharacterID.UUID)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("getting character: %w", err)
	}

	// 5. Validate sorcerer level 2+
	sorcLevel := ClassLevelFromJSON(char.Classes, "Sorcerer")
	if sorcLevel < 2 {
		return FontOfMagicResult{}, fmt.Errorf("Font of Magic requires Sorcerer level 2+, have level %d", sorcLevel)
	}

	// 6. Parse feature uses and validate points
	featureUses, currentPoints, err := ParseFeatureUses(char, FeatureKeySorceryPoints)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	if cost > currentPoints {
		return FontOfMagicResult{}, fmt.Errorf("insufficient sorcery points: need %d to create %s-level slot, have %d",
			cost, ordinal(cmd.CreateSlotLevel), currentPoints)
	}

	// 7. Deduct sorcery points
	newPoints := currentPoints - cost
	featureUses[FeatureKeySorceryPoints] = newPoints
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	// 8. Add spell slot
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	info := slots[cmd.CreateSlotLevel]
	info.Current++
	slots[cmd.CreateSlotLevel] = info
	slotsJSON, err := json.Marshal(intToStringKeyedSlots(slots))
	if err != nil {
		return FontOfMagicResult{}, fmt.Errorf("marshaling spell slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         char.ID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating spell slots: %w", err)
	}

	// 9. Use bonus action
	turn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return FontOfMagicResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return FontOfMagicResult{}, fmt.Errorf("updating turn: %w", err)
	}

	return FontOfMagicResult{
		PointsRemaining: newPoints,
		CombatLog:       FormatFontOfMagicCreate(caster.DisplayName, cmd.CreateSlotLevel, currentPoints, newPoints),
		Remaining:       FormatRemainingResources(turn, nil),
	}, nil
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
