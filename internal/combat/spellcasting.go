package combat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// IsBonusActionSpell returns true if the spell's casting time is a bonus action.
func IsBonusActionSpell(spell refdata.Spell) bool {
	return strings.Contains(strings.ToLower(spell.CastingTime), "bonus action")
}

// ValidateBonusActionSpellRestriction enforces the 5e bonus action spell restriction
// in both directions per Sage Advice.
func ValidateBonusActionSpellRestriction(turn refdata.Turn, spell refdata.Spell) error {
	isBonusAction := IsBonusActionSpell(spell)

	// Forward: if a bonus action spell was already cast this turn,
	// only cantrips can be cast with the action.
	if turn.BonusActionSpellCast && !isBonusAction && spell.Level > 0 {
		return errors.New("You already cast a bonus action spell this turn — you can only cast a cantrip with your action.")
	}

	// Reverse: if a leveled action spell was already cast this turn,
	// no bonus action spells can be cast.
	if turn.ActionSpellCast && isBonusAction {
		return errors.New("You already cast a leveled spell with your action this turn — you cannot cast a bonus action spell.")
	}

	return nil
}

// ValidateSpellSlot checks that the caster has a spell slot available at the given level.
// Cantrips (level 0) never require a slot.
func ValidateSpellSlot(slots map[int]SlotInfo, spellLevel int) error {
	if spellLevel == 0 {
		return nil
	}
	info, ok := slots[spellLevel]
	if !ok || info.Current <= 0 {
		return fmt.Errorf("no %s-level spell slots remaining", ordinal(spellLevel))
	}
	return nil
}

// DeductSpellSlot returns a new slot map with one slot deducted at the given level.
// For cantrips (level 0), returns the slots unchanged.
func DeductSpellSlot(slots map[int]SlotInfo, spellLevel int) map[int]SlotInfo {
	if spellLevel == 0 {
		return slots
	}
	result := make(map[int]SlotInfo, len(slots))
	for k, v := range slots {
		if k == spellLevel {
			result[k] = SlotInfo{Current: v.Current - 1, Max: v.Max}
		} else {
			result[k] = v
		}
	}
	return result
}

// ValidateSpellRange checks whether the target is within the spell's range.
func ValidateSpellRange(spell refdata.Spell, distanceFt int) error {
	switch spell.RangeType {
	case "self", "self (radius)", "sight", "unlimited":
		return nil
	case "touch":
		if distanceFt > 5 {
			return fmt.Errorf("target is %dft away — touch spells require adjacency (5ft): out of range", distanceFt)
		}
		return nil
	case "ranged":
		if spell.RangeFt.Valid && distanceFt > int(spell.RangeFt.Int32) {
			return fmt.Errorf("target is %dft away — %s has range %dft: out of range", distanceFt, spell.Name, spell.RangeFt.Int32)
		}
		return nil
	default:
		return nil
	}
}

// SpellAttackModifier returns the total spell attack modifier.
// Spell attack = proficiency bonus + spellcasting ability modifier.
func SpellAttackModifier(profBonus int, abilityScore int) int {
	return profBonus + AbilityModifier(abilityScore)
}

// ConcentrationResult describes what happened with concentration when casting a spell.
type ConcentrationResult struct {
	DroppedPrevious  bool
	PreviousSpell    string
	NewConcentration string
}

// ResolveConcentration determines the concentration outcome of casting a spell.
// If the new spell requires concentration and the caster is already concentrating,
// the previous spell's concentration is dropped.
func ResolveConcentration(currentConcentration string, spell refdata.Spell) ConcentrationResult {
	isConcentration := spell.Concentration.Valid && spell.Concentration.Bool

	if !isConcentration {
		return ConcentrationResult{
			NewConcentration: currentConcentration,
		}
	}

	if currentConcentration != "" {
		return ConcentrationResult{
			DroppedPrevious:  true,
			PreviousSpell:    currentConcentration,
			NewConcentration: spell.Name,
		}
	}

	return ConcentrationResult{
		NewConcentration: spell.Name,
	}
}

// SpellcastingAbilityForClass returns the spellcasting ability abbreviation for a class.
// Returns empty string for non-spellcasting classes.
func SpellcastingAbilityForClass(className string) string {
	switch strings.ToLower(className) {
	case "wizard":
		return "int"
	case "cleric", "druid", "ranger":
		return "wis"
	case "bard", "paladin", "sorcerer", "warlock":
		return "cha"
	default:
		return ""
	}
}

// CastResult holds the outcome of a /cast command.
type CastResult struct {
	CasterName     string
	SpellName      string
	SpellLevel     int
	IsBonusAction  bool
	IsAttack       bool
	AttackRoll     int
	AttackTotal    int
	TargetAC       int
	Hit            bool
	TargetName     string
	SaveDC         int
	SaveAbility    string
	Concentration  ConcentrationResult
	ResolutionMode string
	SlotUsed       int
	SlotsRemaining int
}

// FormatCastLog produces the combat log output for a spell cast.
func FormatCastLog(result CastResult) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "\u2728 %s casts %s", result.CasterName, result.SpellName)

	if result.IsBonusAction {
		b.WriteString(" (bonus action)")
	}

	if result.TargetName != "" {
		fmt.Fprintf(&b, " on %s", result.TargetName)
	}

	b.WriteString("\n")

	// Slot usage
	if result.SpellLevel > 0 {
		fmt.Fprintf(&b, "\U0001f4a0 Used %s-level slot (%d remaining)\n", ordinal(result.SlotUsed), result.SlotsRemaining)
	}

	// Attack roll
	if result.IsAttack {
		hitMiss := "Miss"
		if result.Hit {
			hitMiss = "Hit"
		}
		fmt.Fprintf(&b, "\U0001f3af Attack: d20(%d) + mod = %d vs AC %d — %s!\n",
			result.AttackRoll, result.AttackTotal, result.TargetAC, hitMiss)
	}

	// Save DC
	if result.SaveDC > 0 {
		fmt.Fprintf(&b, "\U0001f6e1\ufe0f DC %d %s save\n", result.SaveDC, strings.ToUpper(result.SaveAbility))
	}

	// Concentration
	if result.Concentration.DroppedPrevious {
		fmt.Fprintf(&b, "\u26a0\ufe0f Dropped concentration on %s\n", result.Concentration.PreviousSpell)
	}
	if result.Concentration.NewConcentration != "" && result.Concentration.NewConcentration == result.SpellName {
		fmt.Fprintf(&b, "\U0001f9e0 Concentrating on %s\n", result.Concentration.NewConcentration)
	}

	// DM required
	if result.ResolutionMode == "dm_required" {
		b.WriteString("\U0001f4e8 Routed to DM for resolution\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// CastCommand holds the inputs for a /cast command.
type CastCommand struct {
	SpellID              string
	CasterID             uuid.UUID
	TargetID             uuid.UUID // zero value for self spells
	Turn                 refdata.Turn
	CurrentConcentration string // name of current concentration spell, if any
}

// Cast orchestrates the full spell casting flow:
// lookup spell, validate resources, validate range, validate slots,
// check bonus action restrictions, resolve concentration,
// roll spell attack (if applicable), calculate save DC, deduct slot, persist turn.
func (s *Service) Cast(ctx context.Context, cmd CastCommand, roller *dice.Roller) (CastResult, error) {
	// 1. Look up the spell
	spell, err := s.store.GetSpell(ctx, cmd.SpellID)
	if err != nil {
		return CastResult{}, fmt.Errorf("looking up spell %q: %w", cmd.SpellID, err)
	}

	isBonusAction := IsBonusActionSpell(spell)

	// 2. Validate action/bonus action resource
	if isBonusAction {
		if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
			return CastResult{}, err
		}
	} else {
		if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
			return CastResult{}, err
		}
	}

	// 3. Validate bonus action spell restriction (both directions)
	if err := ValidateBonusActionSpellRestriction(cmd.Turn, spell); err != nil {
		return CastResult{}, err
	}

	// 4. Look up caster combatant
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("getting caster: %w", err)
	}

	// 5. Look up character for spell slots and ability scores
	if !caster.CharacterID.Valid {
		return CastResult{}, errors.New("only player characters can cast spells via /cast")
	}

	char, err := s.store.GetCharacter(ctx, caster.CharacterID.UUID)
	if err != nil {
		return CastResult{}, fmt.Errorf("getting character: %w", err)
	}

	// 6. Parse spell slots and validate
	spellLevel := int(spell.Level)
	if spellLevel > 0 {
		slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
		if err != nil {
			return CastResult{}, err
		}
		if err := ValidateSpellSlot(slots, spellLevel); err != nil {
			return CastResult{}, err
		}
	}

	// 7. Resolve target and validate range
	var target refdata.Combatant
	hasTarget := cmd.TargetID != uuid.Nil
	if hasTarget {
		target, err = s.store.GetCombatant(ctx, cmd.TargetID)
		if err != nil {
			return CastResult{}, fmt.Errorf("getting target: %w", err)
		}

		distFt := combatantDistance(caster, target)
		if err := ValidateSpellRange(spell, distFt); err != nil {
			return CastResult{}, err
		}
	}

	// 8. Determine spellcasting ability
	var classes []CharacterClass
	if err := json.Unmarshal(char.Classes, &classes); err != nil {
		return CastResult{}, fmt.Errorf("parsing classes: %w", err)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return CastResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	spellAbilityScore := resolveSpellcastingAbilityScore(classes, scores)

	// 9. Build result
	result := CastResult{
		CasterName:     caster.DisplayName,
		SpellName:      spell.Name,
		SpellLevel:     spellLevel,
		IsBonusAction:  isBonusAction,
		ResolutionMode: spell.ResolutionMode,
	}

	if hasTarget {
		result.TargetName = target.DisplayName
	}

	// 10. Resolve concentration
	result.Concentration = ResolveConcentration(cmd.CurrentConcentration, spell)

	// 11. Save DC for save-based spells
	if spell.SaveAbility.Valid && spell.SaveAbility.String != "" {
		result.SaveDC = SpellSaveDC(int(char.ProficiencyBonus), spellAbilityScore)
		result.SaveAbility = spell.SaveAbility.String
	}

	// 12. Spell attack roll
	if spell.AttackType.Valid && spell.AttackType.String != "" {
		attackMod := SpellAttackModifier(int(char.ProficiencyBonus), spellAbilityScore)
		d20Result, err := roller.RollD20(attackMod, dice.Normal)
		if err != nil {
			return CastResult{}, fmt.Errorf("rolling spell attack: %w", err)
		}
		result.IsAttack = true
		result.AttackRoll = d20Result.Chosen
		result.AttackTotal = d20Result.Total
		result.TargetAC = int(target.Ac)
		result.Hit = d20Result.Total >= int(target.Ac)
	}

	// 13. Use action/bonus action resource
	turn := cmd.Turn
	if isBonusAction {
		turn, err = UseResource(turn, ResourceBonusAction)
		if err != nil {
			return CastResult{}, err
		}
		turn.BonusActionSpellCast = true
	} else {
		turn, err = UseResource(turn, ResourceAction)
		if err != nil {
			return CastResult{}, err
		}
		if spellLevel > 0 {
			turn.ActionSpellCast = true
		}
	}

	// 14. Persist turn state
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return CastResult{}, fmt.Errorf("updating turn: %w", err)
	}

	// 15. Deduct spell slot and persist (only for leveled spells)
	if spellLevel > 0 {
		slots, _ := parseIntKeyedSlots(char.SpellSlots.RawMessage)
		newSlots := DeductSpellSlot(slots, spellLevel)
		result.SlotUsed = spellLevel
		if info, ok := newSlots[spellLevel]; ok {
			result.SlotsRemaining = info.Current
		}

		// Convert back to string-keyed for DB storage
		dbSlots := intToStringKeyedSlots(newSlots)
		slotsJSON, err := json.Marshal(dbSlots)
		if err != nil {
			return CastResult{}, fmt.Errorf("marshaling spell slots: %w", err)
		}
		if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
			ID:         char.ID,
			SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}); err != nil {
			return CastResult{}, fmt.Errorf("updating spell slots: %w", err)
		}
	}

	return result, nil
}

// parseIntKeyedSlots parses spell slots JSON (string-keyed) into int-keyed map.
func parseIntKeyedSlots(raw []byte) (map[int]SlotInfo, error) {
	strSlots, err := ParseSpellSlots(raw)
	if err != nil {
		return nil, err
	}
	result := make(map[int]SlotInfo, len(strSlots))
	for k, v := range strSlots {
		level, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		result[level] = SlotInfo{Current: v.Current, Max: v.Max}
	}
	return result, nil
}

// intToStringKeyedSlots converts int-keyed slot map back to string-keyed for DB storage.
func intToStringKeyedSlots(slots map[int]SlotInfo) map[string]SlotInfo {
	result := make(map[string]SlotInfo, len(slots))
	for k, v := range slots {
		result[strconv.Itoa(k)] = v
	}
	return result
}

// resolveSpellcastingAbilityScore determines the highest applicable spellcasting
// ability score from the character's classes.
func resolveSpellcastingAbilityScore(classes []CharacterClass, scores AbilityScores) int {
	best := 0
	for _, cc := range classes {
		ability := SpellcastingAbilityForClass(cc.Class)
		if ability == "" {
			continue
		}
		score := abilityScoreByName(scores, ability)
		if score > best {
			best = score
		}
	}
	return best
}

// abilityScoreByName returns the ability score for a given abbreviation.
func abilityScoreByName(scores AbilityScores, ability string) int {
	switch strings.ToLower(ability) {
	case "str":
		return scores.Str
	case "dex":
		return scores.Dex
	case "con":
		return scores.Con
	case "int":
		return scores.Int
	case "wis":
		return scores.Wis
	case "cha":
		return scores.Cha
	default:
		return 0
	}
}
