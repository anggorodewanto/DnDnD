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
	if !spell.Concentration.Valid || !spell.Concentration.Bool {
		return ConcentrationResult{NewConcentration: currentConcentration}
	}

	return ConcentrationResult{
		DroppedPrevious:  currentConcentration != "",
		PreviousSpell:    currentConcentration,
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
	CasterName      string
	SpellName       string
	SpellLevel      int
	IsBonusAction   bool
	IsAttack        bool
	AttackRoll      int
	AttackTotal     int
	TargetAC        int
	Hit             bool
	TargetName      string
	SaveDC          int
	SaveAbility     string
	Concentration   ConcentrationResult
	ResolutionMode  string
	SlotUsed        int
	SlotsRemaining  int
	IsRitual        bool
	ScaledDamageDice string // damage dice after upcast/cantrip scaling
	DamageType      string // damage type from spell damage JSON
	ScaledHealingDice string // healing dice after upcast scaling
}

// FormatCastLog produces the combat log output for a spell cast.
func FormatCastLog(result CastResult) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "\u2728 %s casts %s", result.CasterName, result.SpellName)

	if result.IsRitual {
		b.WriteString(" (ritual)")
	}

	if result.IsBonusAction {
		b.WriteString(" (bonus action)")
	}

	if result.TargetName != "" {
		fmt.Fprintf(&b, " on %s", result.TargetName)
	}

	b.WriteString("\n")

	// Slot usage (not for rituals or cantrips)
	if result.SpellLevel > 0 && !result.IsRitual {
		fmt.Fprintf(&b, "\U0001f4a0 Used %s-level slot (%d remaining)\n", ordinal(result.SlotUsed), result.SlotsRemaining)
	}

	// Scaled damage dice
	if result.ScaledDamageDice != "" {
		fmt.Fprintf(&b, "\U0001f4a5 Damage: %s %s\n", result.ScaledDamageDice, result.DamageType)
	}

	// Scaled healing dice
	if result.ScaledHealingDice != "" {
		fmt.Fprintf(&b, "\U0001f49a Healing: %s\n", result.ScaledHealingDice)
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
	if result.Concentration.NewConcentration == result.SpellName && result.Concentration.NewConcentration != "" {
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
	SlotLevel            int    // explicit slot choice; 0 = auto-select lowest available
	IsRitual             bool   // true = cast as ritual (no slot consumed)
	EncounterStatus      string // encounter status for ritual validation ("preparing", "active", "completed")
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
	resource := ResourceAction
	if isBonusAction {
		resource = ResourceBonusAction
	}
	if err := ValidateResource(cmd.Turn, resource); err != nil {
		return CastResult{}, err
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
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return CastResult{}, err
	}

	// 6a. Parse classes (needed for ritual validation and spellcasting ability)
	var classes []CharacterClass
	if err := json.Unmarshal(char.Classes, &classes); err != nil {
		return CastResult{}, fmt.Errorf("parsing classes: %w", err)
	}

	// 6b. Ritual validation
	if cmd.IsRitual {
		primaryClass := ""
		if len(classes) > 0 {
			primaryClass = classes[0].Class
		}
		if err := ValidateRitual(spell.Ritual.Valid && spell.Ritual.Bool, cmd.EncounterStatus, primaryClass); err != nil {
			return CastResult{}, err
		}
	}

	// 6c. Select spell slot (unless ritual or cantrip)
	effectiveSlotLevel := 0
	if spellLevel > 0 && !cmd.IsRitual {
		effectiveSlotLevel, err = SelectSpellSlot(slots, spellLevel, cmd.SlotLevel)
		if err != nil {
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
		IsRitual:       cmd.IsRitual,
	}

	if hasTarget {
		result.TargetName = target.DisplayName
	}

	// 9a. Compute scaled damage dice (cantrip scaling or upcast)
	if spell.Damage.Valid && len(spell.Damage.RawMessage) > 0 {
		dmgInfo, err := ParseSpellDamage(spell.Damage.RawMessage)
		if err == nil {
			result.DamageType = dmgInfo.DamageType
			result.ScaledDamageDice = ScaleSpellDice(dmgInfo, spellLevel, effectiveSlotLevel, int(char.Level))
		}
	}

	// 9b. Compute scaled healing dice (upcast)
	if spell.Healing.Valid && len(spell.Healing.RawMessage) > 0 {
		healInfo, err := ParseSpellHealing(spell.Healing.RawMessage)
		if err == nil {
			result.ScaledHealingDice = ScaleHealingDice(healInfo, spellLevel, effectiveSlotLevel)
		}
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
	turn, err = UseResource(turn, resource)
	if err != nil {
		return CastResult{}, err
	}
	if isBonusAction {
		turn.BonusActionSpellCast = true
	} else if spellLevel > 0 {
		turn.ActionSpellCast = true
	}

	// 14. Persist turn state
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return CastResult{}, fmt.Errorf("updating turn: %w", err)
	}

	// 15. Deduct spell slot and persist (leveled spells, not rituals)
	deduction, err := s.deductAndPersistSlot(ctx, char.ID, slots, effectiveSlotLevel)
	if err != nil {
		return CastResult{}, err
	}
	result.SlotUsed = deduction.SlotUsed
	result.SlotsRemaining = deduction.SlotsRemaining

	return result, nil
}

// SlotDeduction holds the outcome of deducting a spell slot.
type SlotDeduction struct {
	SlotUsed       int
	SlotsRemaining int
}

// deductAndPersistSlot deducts a spell slot and persists the change to the database.
// Returns zero values if slotLevel is 0 (cantrip or ritual).
func (s *Service) deductAndPersistSlot(ctx context.Context, charID uuid.UUID, slots map[int]SlotInfo, slotLevel int) (SlotDeduction, error) {
	if slotLevel <= 0 {
		return SlotDeduction{}, nil
	}
	newSlots := DeductSpellSlot(slots, slotLevel)
	remaining := 0
	if info, ok := newSlots[slotLevel]; ok {
		remaining = info.Current
	}

	slotsJSON, err := json.Marshal(intToStringKeyedSlots(newSlots))
	if err != nil {
		return SlotDeduction{}, fmt.Errorf("marshaling spell slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         charID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return SlotDeduction{}, fmt.Errorf("updating spell slots: %w", err)
	}
	return SlotDeduction{SlotUsed: slotLevel, SlotsRemaining: remaining}, nil
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

// SpellDamageInfo holds parsed damage data from a spell's Damage JSON.
type SpellDamageInfo struct {
	Dice           string `json:"dice"`
	DamageType     string `json:"type"`
	HigherLevelDice string `json:"higher_level_dice"`
	CantripScaling bool   `json:"cantrip_scaling"`
}

// ParseSpellDamage parses the damage JSON from a spell.
func ParseSpellDamage(raw []byte) (SpellDamageInfo, error) {
	if len(raw) == 0 {
		return SpellDamageInfo{}, errors.New("damage data is empty")
	}
	var d SpellDamageInfo
	if err := json.Unmarshal(raw, &d); err != nil {
		return SpellDamageInfo{}, fmt.Errorf("parsing damage: %w", err)
	}
	return d, nil
}

// SpellHealingInfo holds parsed healing data from a spell's Healing JSON.
type SpellHealingInfo struct {
	Dice           string `json:"dice"`
	HigherLevelDice string `json:"higher_level_dice"`
}

// ParseSpellHealing parses the healing JSON from a spell.
func ParseSpellHealing(raw []byte) (SpellHealingInfo, error) {
	if len(raw) == 0 {
		return SpellHealingInfo{}, errors.New("healing data is empty")
	}
	var h SpellHealingInfo
	if err := json.Unmarshal(raw, &h); err != nil {
		return SpellHealingInfo{}, fmt.Errorf("parsing healing: %w", err)
	}
	return h, nil
}

// ScaleSpellDice computes the effective dice string considering upcasting and cantrip scaling.
// For cantrips with cantrip_scaling: multiplies the base dice count by CantripDiceMultiplier.
// For upcasting: adds (slotLevel - spellLevel) * higher_level_dice to the base.
// Supports both simple format "8d6" and ray format "3x2d6".
func ScaleSpellDice(d SpellDamageInfo, spellLevel, slotLevel, charLevel int) string {
	if d.CantripScaling {
		return scaleCantripDice(d.Dice, charLevel)
	}
	if d.HigherLevelDice == "" || slotLevel <= spellLevel {
		return d.Dice
	}
	levelsAbove := slotLevel - spellLevel
	return addDice(d.Dice, d.HigherLevelDice, levelsAbove)
}

// ScaleHealingDice computes the effective healing dice string for upcasting.
func ScaleHealingDice(h SpellHealingInfo, spellLevel, slotLevel int) string {
	if h.HigherLevelDice == "" || slotLevel <= spellLevel {
		return h.Dice
	}
	levelsAbove := slotLevel - spellLevel
	return addDice(h.Dice, h.HigherLevelDice, levelsAbove)
}

// scaleCantripDice multiplies the dice count in a dice string by the cantrip multiplier.
// e.g., "1d8" at level 5 -> "2d8"
func scaleCantripDice(baseDice string, charLevel int) string {
	mult := CantripDiceMultiplier(charLevel)
	count, die := parseDiceExpr(baseDice)
	return fmt.Sprintf("%d%s", count*mult, die)
}

// addDice adds bonus dice to a base dice expression.
// Supports "NdX" format (adds to count) and "NxMdX" format (adds to multiplier).
func addDice(base, bonus string, times int) string {
	// Try ray format: "NxMdX"
	if strings.Contains(base, "x") && strings.Contains(bonus, "x") {
		baseParts := strings.SplitN(base, "x", 2)
		bonusParts := strings.SplitN(bonus, "x", 2)
		baseCount, _ := strconv.Atoi(baseParts[0])
		bonusCount, _ := strconv.Atoi(bonusParts[0])
		return fmt.Sprintf("%dx%s", baseCount+bonusCount*times, baseParts[1])
	}

	// Simple format: "NdX" or "NdX+mod"
	baseCount, baseDie := parseDiceExpr(base)
	bonusCount, _ := parseDiceExpr(bonus)
	return fmt.Sprintf("%d%s", baseCount+bonusCount*times, baseDie)
}

// parseDiceExpr splits a dice expression "NdX" into count and "dX" (keeping any suffix like "+mod").
// Returns (N, "dX+suffix").
func parseDiceExpr(expr string) (int, string) {
	dIdx := strings.Index(expr, "d")
	if dIdx < 0 {
		return 0, expr
	}
	countStr := expr[:dIdx]
	rest := expr[dIdx:] // e.g., "d8+mod"
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 1, rest
	}
	return count, rest
}

// HasRitualCasting returns true if the given class has the Ritual Casting feature.
func HasRitualCasting(className string) bool {
	switch strings.ToLower(className) {
	case "wizard", "cleric", "druid", "bard":
		return true
	default:
		return false
	}
}

// ValidateRitual checks that a spell can be cast as a ritual.
func ValidateRitual(spellRitual bool, encounterStatus string, className string) error {
	if !spellRitual {
		return errors.New("this spell cannot be cast as a ritual")
	}
	if encounterStatus == "active" {
		return errors.New("cannot cast rituals during active combat")
	}
	if !HasRitualCasting(className) {
		return fmt.Errorf("%s does not have the Ritual Casting feature", className)
	}
	return nil
}

// CantripDiceMultiplier returns how many dice a cantrip rolls based on character level.
// Level 1-4: 1, Level 5-10: 2, Level 11-16: 3, Level 17+: 4.
func CantripDiceMultiplier(charLevel int) int {
	if charLevel >= 17 {
		return 4
	}
	if charLevel >= 11 {
		return 3
	}
	if charLevel >= 5 {
		return 2
	}
	return 1
}

// SelectSpellSlot determines which spell slot level to use.
// For cantrips (spellLevel 0), returns 0.
// If slotLevel is specified (> 0), validates it is >= spellLevel and has remaining slots.
// If slotLevel is 0 (auto), picks the lowest available slot >= spellLevel.
func SelectSpellSlot(slots map[int]SlotInfo, spellLevel int, slotLevel int) (int, error) {
	if spellLevel == 0 {
		return 0, nil
	}

	// Explicit slot selection
	if slotLevel > 0 {
		if slotLevel < spellLevel {
			return 0, fmt.Errorf("slot level %d is below spell level %d", slotLevel, spellLevel)
		}
		info, ok := slots[slotLevel]
		if !ok || info.Current <= 0 {
			return 0, fmt.Errorf("no %s-level spell slots remaining", ordinal(slotLevel))
		}
		return slotLevel, nil
	}

	// Auto-select: find lowest available slot >= spellLevel
	for level := spellLevel; level <= 9; level++ {
		info, ok := slots[level]
		if ok && info.Current > 0 {
			return level, nil
		}
	}
	return 0, fmt.Errorf("no spell slots remaining at level %d or above", spellLevel)
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
		score := scores.ScoreByName(ability)
		if score > best {
			best = score
		}
	}
	return best
}
