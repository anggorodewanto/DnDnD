package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// IsBonusActionSpell returns true if the spell's casting time is a bonus action.
func IsBonusActionSpell(spell refdata.Spell) bool {
	return strings.Contains(strings.ToLower(spell.CastingTime), "bonus action")
}

// ValidateBonusActionSpellRestriction enforces the 5e bonus action spell restriction
// in both directions per Sage Advice. The isBonusAction parameter indicates whether
// the spell is effectively a bonus action spell (accounting for metamagic like Quickened Spell).
func ValidateBonusActionSpellRestriction(turn refdata.Turn, spell refdata.Spell, isBonusAction bool) error {
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
	CasterName             string
	SpellName              string
	SpellLevel             int
	IsBonusAction          bool
	IsAttack               bool
	AttackRoll             int
	AttackTotal            int
	TargetAC               int
	Hit                    bool
	TargetName             string
	SaveDC                 int
	SaveAbility            string
	Concentration          ConcentrationResult
	ResolutionMode         string
	SlotUsed               int
	SlotsRemaining         int
	UsedPactSlot           bool
	PactSlotsRemaining     int
	IsRitual               bool
	ScaledDamageDice       string                     // damage dice after upcast/cantrip scaling
	DamageType             string                     // damage type from spell damage JSON
	ScaledHealingDice      string                     // healing dice after upcast scaling
	Teleport               *TeleportResult            // teleportation outcome, nil if not a teleport spell
	MaterialComponent      *CastMaterialComponentInfo // material component outcome, nil if no costly component
	MetamagicCost          int                        // total sorcery points spent on metamagic
	SorceryPointsRemaining int                        // sorcery points remaining after metamagic cost
	// Metamagic effect fields
	CarefulSpellCreatures int    // number of creatures that auto-succeed on AoE save (CHA mod)
	DistantRange          string // new range description after Distant Spell
	IsEmpowered           bool   // true if Empowered Spell is active (reroll damage dice)
	EmpoweredRerolls      int    // max damage dice that can be rerolled (CHA mod)
	ExtendedDuration      string // doubled duration after Extended Spell
	IsHeightened          bool   // true if target has disadvantage on first save
	IsSubtle              bool   // true if V/S components removed (bypasses Counterspell/Silence)
	TwinTargetName        string // display name of the second target for Twinned Spell
	TwinTargetID          string // ID of the second target for Twinned Spell
	InvisibilityBroken    bool   // true if standard Invisibility was broken by casting
	InvisibilityApplied   bool   // true if this cast applied the invisible condition to a target
	InvisibilityTargetID  string // combatant that received the invisible condition (if any)
	// SR-017: when /cast spare-the-dying lands on a dying creature, the
	// stabilize log line surfaces here so handlers / FormatCastLog can mirror
	// the 🩹 outcome alongside the cast header. Empty when the spell was cast
	// on a non-dying target (no-op) or when the cast was not spare-the-dying.
	StabilizeMessage string
	// Phase 118: when this cast replaced an existing concentration spell, the
	// dropped spell's effects were cleaned up server-side. The full result is
	// surfaced here so the handler can post the consolidated 💨 log line.
	ConcentrationCleanup BreakConcentrationFullyResult
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
		if result.UsedPactSlot {
			fmt.Fprintf(&b, "\U0001f4a0 Used pact slot (%d remaining)\n", result.PactSlotsRemaining)
		} else {
			fmt.Fprintf(&b, "\U0001f4a0 Used %s-level slot (%d remaining)\n", ordinal(result.SlotUsed), result.SlotsRemaining)
		}
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

	// Teleportation
	if result.Teleport != nil {
		if result.Teleport.CasterMoved {
			fmt.Fprintf(&b, "\U0001f300 %s teleports to %s%d\n", result.CasterName, result.Teleport.CasterDestCol, result.Teleport.CasterDestRow)
		}
		if result.Teleport.CompanionMoved {
			fmt.Fprintf(&b, "\U0001f300 %s teleports to %s%d\n", result.Teleport.CompanionName, result.Teleport.CompanionDestCol, result.Teleport.CompanionDestRow)
		}
		if result.Teleport.AdditionalEffects != "" {
			fmt.Fprintf(&b, "\u26a1 %s\n", result.Teleport.AdditionalEffects)
		}
	}

	// Metamagic effects
	if result.CarefulSpellCreatures > 0 {
		fmt.Fprintf(&b, "\u2728 Careful Spell: up to %d creatures auto-succeed on the save\n", result.CarefulSpellCreatures)
	}
	if result.DistantRange != "" {
		fmt.Fprintf(&b, "\u2728 Distant Spell: range extended to %s\n", result.DistantRange)
	}
	if result.IsEmpowered {
		fmt.Fprintf(&b, "\u2728 Empowered Spell: may reroll up to %d damage dice\n", result.EmpoweredRerolls)
	}
	if result.ExtendedDuration != "" {
		fmt.Fprintf(&b, "\u2728 Extended Spell: duration is %s\n", result.ExtendedDuration)
	}
	if result.IsHeightened {
		b.WriteString("\u2728 Heightened Spell: target has disadvantage on first save\n")
	}
	if result.IsSubtle {
		b.WriteString("\u2728 Subtle Spell: no verbal/somatic components\n")
	}
	if result.TwinTargetName != "" {
		fmt.Fprintf(&b, "\u2728 Twinned Spell: also targets %s\n", result.TwinTargetName)
	}

	// Invisibility effects
	if result.InvisibilityApplied {
		recipient := result.TargetName
		if recipient == "" {
			recipient = result.CasterName
		}
		fmt.Fprintf(&b, "\U0001f441\ufe0f %s becomes invisible.\n", recipient)
	}
	if result.InvisibilityBroken {
		fmt.Fprintf(&b, "\U0001f441\ufe0f Invisibility ends \u2014 %s is visible again.\n", result.CasterName)
	}

	// SR-017: stabilize outcome from /cast spare-the-dying.
	if result.StabilizeMessage != "" {
		fmt.Fprintf(&b, "%s\n", result.StabilizeMessage)
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
	CurrentConcentration string    // name of current concentration spell, if any
	SlotLevel            int       // explicit slot choice; 0 = auto-select lowest available
	UseSpellSlot         bool      // true = force using regular spell slots instead of pact slots
	IsRitual             bool      // true = cast as ritual (no slot consumed)
	EncounterStatus      string    // encounter status for ritual validation ("preparing", "active", "completed")
	EncounterID          uuid.UUID // encounter ID (needed for teleport occupant checks)
	TeleportDestCol      string    // teleport destination column (e.g. "D")
	TeleportDestRow      int32     // teleport destination row (e.g. 5)
	CompanionID          uuid.UUID // companion combatant ID for self+creature teleports
	CompanionDestCol     string    // companion teleport destination column
	CompanionDestRow     int32     // companion teleport destination row
	GoldFallback         bool      // true if user confirmed "Buy & Cast" gold fallback
	Metamagic            []string  // metamagic options: "careful", "distant", "empowered", etc.
	TwinTargetID         uuid.UUID // second target for Twinned Spell
	Walls                []renderer.WallSegment
	FogOfWar             *renderer.FogOfWar
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

	// 1a. Quickened Spell metamagic changes casting time to bonus action
	if hasMetamagic(cmd.Metamagic, "quickened") {
		isBonusAction = true
	}

	// 2. Validate action/bonus action resource
	resource := ResourceAction
	if isBonusAction {
		resource = ResourceBonusAction
	}
	if err := ValidateResource(cmd.Turn, resource); err != nil {
		return CastResult{}, err
	}

	// 3. Validate bonus action spell restriction (both directions)
	if err := ValidateBonusActionSpellRestriction(cmd.Turn, spell, isBonusAction); err != nil {
		return CastResult{}, err
	}

	// 4. Look up caster combatant
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("getting caster: %w", err)
	}

	// D-46-rage-spellcasting-block — a raging barbarian cannot cast any
	// spell (the Rage feature in PHB explicitly blocks spellcasting and
	// breaks concentration). Reject BEFORE any resource is touched so a
	// player who fat-fingers /cast while raging doesn't burn a slot.
	if caster.IsRaging {
		return CastResult{}, errors.New("you cannot cast spells while raging")
	}

	// 4a. med-25 / Phase 61: pre-validate Silence zones. A caster standing
	// in a Silence zone cannot cast spells with V or S components — this
	// must reject BEFORE slot deduction so the player doesn't lose a slot
	// to a silenced cast attempt.
	inSilence, err := s.combatantInSilenceZone(ctx, caster)
	if err != nil {
		return CastResult{}, fmt.Errorf("checking silence zone: %w", err)
	}
	if err := ValidateSilenceZone(inSilence, spell); err != nil {
		return CastResult{}, err
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

	// 6a-bis. med-43 / Phase 47: a Wild-Shaped Druid below level 18 cannot
	// cast spells (Beast Spells unlocks at level 18). Reject BEFORE slot
	// deduction so the player doesn't lose a slot to a beast-form cast.
	if caster.IsWildShaped {
		druidLevel := druidLevelFromClasses(classes)
		if !CanWildShapeSpellcast(druidLevel) {
			return CastResult{}, fmt.Errorf("cannot cast spells while in Wild Shape (Beast Spells unlocks at Druid 18)")
		}
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

	// 6c. Parse pact magic slots and select spell slot (unless ritual or cantrip)
	pactSlots, _ := parsePactMagicSlots(char.PactMagicSlots.RawMessage)

	effectiveSlotLevel := 0
	usePactSlot := false
	if spellLevel > 0 && !cmd.IsRitual {
		// Use pact slot if available, spell fits, and not forced to regular slots
		if !cmd.UseSpellSlot && pactSlots.Current > 0 && spellLevel <= pactSlots.SlotLevel {
			effectiveSlotLevel = pactSlots.SlotLevel
			usePactSlot = true
		} else {
			effectiveSlotLevel, err = SelectSpellSlot(slots, spellLevel, cmd.SlotLevel)
			if err != nil {
				return CastResult{}, err
			}
		}
	}

	// 6d. Material component check (validation only — deduction deferred until
	// all later validations pass, see step 12b).
	type materialDeduction struct {
		deductGold       bool
		newGold          int32
		addItem          bool
		removeItem       bool
		inventory        []InventoryItem
		componentName    string
	}
	var matDeduction *materialDeduction
	if spell.MaterialCostGp.Valid {
		inventory, err := ParseInventory(char.Inventory.RawMessage)
		if err != nil {
			return CastResult{}, fmt.Errorf("parsing inventory: %w", err)
		}

		matResult := ValidateMaterialComponent(spell, inventory, char.Gold)

		switch matResult.Outcome {
		case MaterialCheckRejected:
			return CastResult{}, errors.New(FormatMaterialRejection(matResult))

		case MaterialCheckNeedsGoldConfirmation:
			if !cmd.GoldFallback {
				// Return a result indicating gold confirmation is needed
				return CastResult{
					CasterName: caster.DisplayName,
					SpellName:  spell.Name,
					SpellLevel: spellLevel,
					MaterialComponent: &CastMaterialComponentInfo{
						NeedsGoldConfirmation: true,
						ComponentName:         matResult.ComponentName,
						CostGp:                matResult.CostGp,
						CurrentGold:           matResult.CurrentGold,
					},
				}, nil
			}
			// User confirmed: record deduction for after validations pass
			matDeduction = &materialDeduction{
				deductGold:    true,
				newGold:       char.Gold - int32(matResult.CostGp),
				addItem:       !matResult.MaterialConsumed,
				inventory:     inventory,
				componentName: matResult.ComponentName,
			}

		case MaterialCheckProceed:
			// Component found in inventory — consume if needed after cast succeeds
			if matResult.MaterialConsumed {
				matDeduction = &materialDeduction{
					removeItem:    true,
					inventory:     inventory,
					componentName: matResult.ComponentName,
				}
			}
		}
	}

	// 6e. Metamagic validation and sorcery point tracking
	var metamagicCost int
	var metamagicFeatureUses map[string]character.FeatureUse
	var metamagicCurrentPoints int
	if len(cmd.Metamagic) > 0 {
		metamagicFeatureUses, metamagicCurrentPoints, err = ParseFeatureUses(char, FeatureKeySorceryPoints)
		if err != nil {
			return CastResult{}, err
		}
		if err := ValidateMetamagic(cmd.Metamagic, spellLevel, metamagicCurrentPoints); err != nil {
			return CastResult{}, err
		}
		if err := ValidateMetamagicOptions(cmd.Metamagic, spell); err != nil {
			return CastResult{}, err
		}
		metamagicCost, err = MetamagicTotalCost(cmd.Metamagic, spellLevel)
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

		// See-the-target validation: a single-target spell against an invisible
		// target is blocked unless the spell is AoE. Self-targeted spells
		// bypass this because the caster always knows their own position.
		if err := ValidateSeeTarget(spell, target); err != nil {
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
	if hasDamage(spell) {
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

	// 9c. Apply metamagic effects to result
	if len(cmd.Metamagic) > 0 {
		applyMetamagicEffects(&result, cmd.Metamagic, spell, scores.Cha)
	}

	// 9d. Resolve Twinned Spell second target
	if hasMetamagic(cmd.Metamagic, "twinned") && cmd.TwinTargetID != uuid.Nil {
		twinTarget, err := s.store.GetCombatant(ctx, cmd.TwinTargetID)
		if err != nil {
			return CastResult{}, fmt.Errorf("getting twin target: %w", err)
		}
		// Validate twin target is in range
		twinDistFt := combatantDistance(caster, twinTarget)
		if err := ValidateSpellRange(spell, twinDistFt); err != nil {
			return CastResult{}, fmt.Errorf("twin target out of range: %w", err)
		}
		result.TwinTargetName = twinTarget.DisplayName
		result.TwinTargetID = twinTarget.ID.String()
	}

	// 10. Resolve concentration: clean up any dropped spell, persist the new
	// concentration to the authoritative columns when applicable.
	result.Concentration = ResolveConcentration(cmd.CurrentConcentration, spell)
	cleanup, err := s.applyConcentrationOnCast(ctx, caster, spell, result.Concentration)
	if err != nil {
		return CastResult{}, err
	}
	result.ConcentrationCleanup = cleanup

	// 11. Save DC for save-based spells
	if hasSavingThrow(spell) {
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

	// 12a. Teleportation handling
	if spell.Teleport.Valid && len(spell.Teleport.RawMessage) > 0 {
		teleResult, err := s.resolveTeleport(ctx, spell.Teleport.RawMessage, caster, cmd)
		if err != nil {
			return CastResult{}, err
		}
		result.Teleport = teleResult
		if teleResult.DMQueueRouted {
			result.ResolutionMode = "dm_required"
		}
	}

	// 12b. Deferred material deduction — all validations have passed.
	if matDeduction != nil {
		if matDeduction.deductGold {
			if err := s.store.UpdateCharacterGold(ctx, char.ID, matDeduction.newGold); err != nil {
				return CastResult{}, fmt.Errorf("deducting gold: %w", err)
			}
			if matDeduction.addItem {
				newItems := AddInventoryItem(matDeduction.inventory, matDeduction.componentName)
				if err := s.persistInventory(ctx, char.ID, newItems); err != nil {
					return CastResult{}, fmt.Errorf("updating inventory: %w", err)
				}
			}
		}
		if matDeduction.removeItem {
			newItems := RemoveInventoryItem(matDeduction.inventory, matDeduction.componentName)
			if err := s.persistInventory(ctx, char.ID, newItems); err != nil {
				return CastResult{}, fmt.Errorf("updating inventory: %w", err)
			}
		}
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
	if usePactSlot {
		deduction, err := s.deductAndPersistPactSlot(ctx, char.ID, pactSlots)
		if err != nil {
			return CastResult{}, err
		}
		result.SlotUsed = effectiveSlotLevel
		result.UsedPactSlot = true
		result.PactSlotsRemaining = deduction.SlotsRemaining
	} else {
		deduction, err := s.deductAndPersistSlot(ctx, char.ID, slots, effectiveSlotLevel)
		if err != nil {
			return CastResult{}, err
		}
		result.SlotUsed = deduction.SlotUsed
		result.SlotsRemaining = deduction.SlotsRemaining
	}

	// 16. Deduct sorcery points for metamagic
	if metamagicCost > 0 {
		newPoints, err := s.DeductFeaturePool(ctx, char, FeatureKeySorceryPoints, metamagicFeatureUses, metamagicCurrentPoints, metamagicCost)
		if err != nil {
			return CastResult{}, err
		}
		result.MetamagicCost = metamagicCost
		result.SorceryPointsRemaining = newPoints
	}

	// 17. Apply invisibility condition when casting Invisibility / Greater Invisibility.
	if spell.ID == InvisibilitySpellID || spell.ID == GreaterInvisibilitySpellID {
		recipient, err := s.applyInvisibilityConditionFromCast(ctx, spell, caster, cmd.TargetID)
		if err != nil {
			return CastResult{}, err
		}
		result.InvisibilityApplied = true
		result.InvisibilityTargetID = recipient
	}

	if spell.ID == FlySpellID {
		if err := s.applyFlySpeedConditionFromCast(ctx, spell, caster, cmd.TargetID); err != nil {
			return CastResult{}, err
		}
	}

	// 17a. SR-017: /cast spare-the-dying on a dying creature stabilizes them
	// per SRD ("a living creature that has 0 hit points ... becomes stable").
	// Skipped silently when the target is not dying — the spell still spends
	// the action but no death-save state changes.
	if spell.ID == SpareTheDyingSpellID && hasTarget {
		stabilizeMsg, err := s.applySpareTheDyingFromCast(ctx, target)
		if err != nil {
			return CastResult{}, err
		}
		result.StabilizeMessage = stabilizeMsg
	}

	// 18. Break standard Invisibility on the caster (Greater persists).
	broken, err := s.breakInvisibilityAndPersist(ctx, caster)
	if err != nil {
		return CastResult{}, err
	}
	result.InvisibilityBroken = broken

	// 19. med-26 / Phase 67: auto-create persistent zones for known
	// AoE / area-effect spells. Best-effort — zone creation errors are
	// logged via the returned error so DM can investigate but Cast itself
	// has already succeeded by this point.
	if zoneErr := s.maybeCreateSpellZone(ctx, spell, caster, cmd); zoneErr != nil {
		return result, zoneErr
	}

	return result, nil
}

// maybeCreateSpellZone inserts an encounter_zones row when the cast spell has
// a known ZoneDefinition. AnchorMode "combatant" pins the zone to the caster
// so subsequent UpdateCombatantPosition calls move the zone with them. This
// is the wiring that makes Spirit Guardians, Wall of Fire, Darkness, Silence,
// Fog Cloud, Cloud of Daggers, Moonbeam, and Stinking Cloud actually appear
// on the encounter map. (med-26 / Phase 67)
func (s *Service) maybeCreateSpellZone(ctx context.Context, spell refdata.Spell, caster refdata.Combatant, cmd CastCommand) error {
	def, ok := LookupZoneDefinition(spell.Name)
	if !ok {
		return nil
	}
	dimensions := zoneDimensionsForDefinition(def, spell)
	anchorID := uuid.NullUUID{}
	if def.AnchorMode == "combatant" {
		anchorID = uuid.NullUUID{UUID: caster.ID, Valid: true}
	}
	expiresAt := s.computeZoneExpiry(ctx, caster.EncounterID, spell)
	_, err := s.CreateZone(ctx, CreateZoneInput{
		EncounterID:           caster.EncounterID,
		SourceCombatantID:     caster.ID,
		SourceSpell:           spell.Name,
		Shape:                 def.Shape,
		OriginCol:             caster.PositionCol,
		OriginRow:             caster.PositionRow,
		Dimensions:            dimensions,
		AnchorMode:            zoneAnchorOrDefault(def.AnchorMode),
		AnchorCombatantID:     anchorID,
		ZoneType:              def.ZoneType,
		OverlayColor:          def.OverlayColor,
		MarkerIcon:            def.MarkerIcon,
		RequiresConcentration: def.RequiresConcentration,
		ExpiresAtRound:        expiresAt,
		Triggers:              def.Triggers,
	})
	if err != nil {
		return fmt.Errorf("creating zone for %s: %w", spell.Name, err)
	}
	return nil
}

// computeZoneExpiry resolves a spell's Duration string ("10 minutes", "1
// minute", "Concentration, up to 1 minute", "Instantaneous", ...) into
// `currentRound + rounds` so CleanupExpiredZones can remove the zone when its
// life ends. A duration the parser cannot interpret (e.g. "Until dispelled"
// or "Special") yields an invalid sql.NullInt32 so the zone persists until
// explicitly torn down via concentration drop, DM /undo, or encounter end.
// (E-67-zone-cleanup)
func (s *Service) computeZoneExpiry(ctx context.Context, encounterID uuid.UUID, spell refdata.Spell) sql.NullInt32 {
	rounds, ok := SpellDurationRounds(spell.Duration)
	if !ok {
		return sql.NullInt32{}
	}
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return sql.NullInt32{}
	}
	// The cleanup query drops zones with ExpiresAtRound <= currentRound,
	// so a freshly-cast zone with 10 rounds of life expires at
	// (currentRound + 10).
	return sql.NullInt32{Int32: enc.RoundNumber + int32(rounds), Valid: true}
}

// SpellDurationRounds parses a 5e spell duration string into a count of
// 6-second combat rounds.
//
// Recognised forms (case-insensitive, leading "concentration, up to "
// stripped):
//
//	"1 round"          => 1
//	"10 rounds"        => 10
//	"1 minute"         => 10  (1 min / 6 s)
//	"10 minutes"       => 100
//	"1 hour"           => 600
//	"24 hours"         => 14400
//	"instantaneous"    => 0, false  (no zone persistence needed)
//
// Anything else returns (0, false) so the caller can leave ExpiresAtRound
// unset for "Until dispelled" / "Special" / unknown values. (E-67-zone-cleanup)
func SpellDurationRounds(duration string) (int, bool) {
	d := strings.ToLower(strings.TrimSpace(duration))
	d = strings.TrimPrefix(d, "concentration,")
	d = strings.TrimSpace(d)
	d = strings.TrimPrefix(d, "up to ")
	d = strings.TrimSpace(d)
	if d == "" || d == "instantaneous" || d == "permanent" || strings.HasPrefix(d, "until ") || strings.HasPrefix(d, "special") {
		return 0, false
	}
	parts := strings.Fields(d)
	if len(parts) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return 0, false
	}
	unit := strings.TrimSuffix(parts[1], "s")
	switch unit {
	case "round":
		return n, true
	case "minute":
		return n * 10, true
	case "hour":
		return n * 600, true
	}
	return 0, false
}

// zoneAnchorOrDefault returns "static" when the def has no explicit anchor
// mode set, matching the encounter_zones default.
func zoneAnchorOrDefault(mode string) string {
	if mode == "" {
		return "static"
	}
	return mode
}

// zoneDimensionsForDefinition synthesises a minimal Dimensions JSON blob
// matching the zone's Shape so DrawZoneOverlays + tile-coverage helpers have
// the expected radius/side/length keys. Sizes use spec defaults (Spirit
// Guardians 15ft, Fog Cloud 20ft, etc.). When a future schema migration
// adds a per-spell area_size column, this helper can read from it. (med-26)
func zoneDimensionsForDefinition(def ZoneDefinition, _ refdata.Spell) json.RawMessage {
	size := defaultZoneSizeFt(def)
	switch def.Shape {
	case "square":
		return json.RawMessage(fmt.Sprintf(`{"side_ft":%d}`, size))
	case "line":
		return json.RawMessage(fmt.Sprintf(`{"length_ft":%d,"width_ft":5}`, size))
	default:
		// circle / sphere
		return json.RawMessage(fmt.Sprintf(`{"radius_ft":%d}`, size))
	}
}

func defaultZoneSizeFt(def ZoneDefinition) int32 {
	switch def.SpellName {
	case "Spirit Guardians":
		return 15
	case "Fog Cloud":
		return 20
	case "Darkness", "Silence":
		return 20
	case "Cloud of Daggers":
		return 5
	case "Moonbeam":
		return 5
	case "Wall of Fire":
		return 60
	case "Stinking Cloud":
		return 20
	}
	return 20
}

// lookupCasterConcentrationID reads the authoritative concentration spell ID
// from the combatants row. Returns "" when the caster is not concentrating
// or the column is unset (e.g. concentration was tracked via the legacy
// `cmd.CurrentConcentration` string only). Lookup errors are logged via
// slog and a "" sentinel is returned so the downstream cleanup path can
// still emit a best-effort log line based on the human-readable spell name;
// callers that need stricter error handling should call the underlying
// store directly.
func (s *Service) lookupCasterConcentrationID(ctx context.Context, casterID uuid.UUID) string {
	row, err := s.store.GetCombatantConcentration(ctx, casterID)
	if err != nil {
		slog.WarnContext(ctx, "concentration lookup failed",
			"caster_id", casterID,
			"error", err,
		)
		return ""
	}
	if !row.ConcentrationSpellID.Valid {
		return ""
	}
	return row.ConcentrationSpellID.String
}

// applySpareTheDyingFromCast wires the spare-the-dying cantrip's auto-resolve
// effect (SR-017). When the target is a dying combatant (alive at 0 HP, not
// yet stable per IsDying) it calls StabilizeTarget and persists the 3-success
// outcome via UpdateCombatantDeathSaves; any prior failure tally is preserved
// in case the DM later wants to inspect what happened. On a non-dying target
// the helper returns "" with no error so the cast still consumes the action
// but no death-save state changes — matches the SRD restriction "a living
// creature that has 0 hit points".
func (s *Service) applySpareTheDyingFromCast(ctx context.Context, target refdata.Combatant) (string, error) {
	ds, err := ParseDeathSaves(target.DeathSaves.RawMessage)
	if err != nil {
		return "", fmt.Errorf("parsing target death saves: %w", err)
	}
	if !IsDying(target.IsAlive, int(target.HpCurrent), ds) {
		return "", nil
	}
	outcome := StabilizeTarget(target.DisplayName, ds, "Spare the Dying")
	if _, err := s.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         target.ID,
		DeathSaves: MarshalDeathSaves(outcome.DeathSaves),
	}); err != nil {
		return "", fmt.Errorf("persisting stabilize: %w", err)
	}
	return strings.Join(outcome.Messages, "\n"), nil
}

// applyInvisibilityConditionFromCast adds an "invisible" condition to the spell's
// target (or caster when no explicit target). Returns the combatant ID that
// received the condition as a string.
func (s *Service) applyInvisibilityConditionFromCast(ctx context.Context, spell refdata.Spell, caster refdata.Combatant, targetID uuid.UUID) (string, error) {
	recipientID := targetID
	if recipientID == uuid.Nil {
		recipientID = caster.ID
	}
	cond := CombatCondition{
		Condition:         "invisible",
		SourceCombatantID: caster.ID.String(),
		SourceSpell:       spell.ID,
		// DurationRounds=0 -> indefinite; concentration tracks the spell, and
		// concentration loss / duration end remove the condition separately.
	}
	if _, _, err := s.ApplyCondition(ctx, recipientID, cond); err != nil {
		return "", fmt.Errorf("applying invisible condition: %w", err)
	}
	return recipientID.String(), nil
}

func (s *Service) applyFlySpeedConditionFromCast(ctx context.Context, spell refdata.Spell, caster refdata.Combatant, targetID uuid.UUID) error {
	recipientID := targetID
	if recipientID == uuid.Nil {
		recipientID = caster.ID
	}
	cond := CombatCondition{
		Condition:         FlySpeedCondition,
		SourceCombatantID: caster.ID.String(),
		SourceSpell:       spell.ID,
	}
	if _, _, err := s.ApplyCondition(ctx, recipientID, cond); err != nil {
		return fmt.Errorf("applying fly speed condition: %w", err)
	}
	return nil
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

// PactMagicSlotState is an alias for character.PactMagicSlots used within the combat package.
type PactMagicSlotState = character.PactMagicSlots

// parsePactMagicSlots parses the pact_magic_slots JSONB column.
// Returns zero-value if data is nil/empty or unparseable.
func parsePactMagicSlots(raw []byte) (PactMagicSlotState, error) {
	if len(raw) == 0 {
		return PactMagicSlotState{}, nil
	}
	var ps PactMagicSlotState
	if err := json.Unmarshal(raw, &ps); err != nil {
		return PactMagicSlotState{}, fmt.Errorf("parsing pact magic slots: %w", err)
	}
	return ps, nil
}

// persistPactMagicSlots marshals and saves pact magic slot state to the database.
func (s *Service) persistPactMagicSlots(ctx context.Context, charID uuid.UUID, pact PactMagicSlotState) error {
	pactJSON, err := json.Marshal(pact)
	if err != nil {
		return fmt.Errorf("marshaling pact magic slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterPactMagicSlots(ctx, refdata.UpdateCharacterPactMagicSlotsParams{
		ID:             charID,
		PactMagicSlots: pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true},
	}); err != nil {
		return fmt.Errorf("updating pact magic slots: %w", err)
	}
	return nil
}

// RechargePactMagicSlots restores all pact magic slots to their maximum.
// This is called on short rest. No-op for characters without pact magic slots.
func (s *Service) RechargePactMagicSlots(ctx context.Context, charID uuid.UUID) error {
	char, err := s.store.GetCharacter(ctx, charID)
	if err != nil {
		return fmt.Errorf("getting character: %w", err)
	}

	pact, err := parsePactMagicSlots(char.PactMagicSlots.RawMessage)
	if err != nil {
		return err
	}
	if pact.Max == 0 {
		return nil
	}

	pact.Current = pact.Max
	return s.persistPactMagicSlots(ctx, charID, pact)
}

// deductAndPersistPactSlot deducts one pact magic slot and persists the change.
func (s *Service) deductAndPersistPactSlot(ctx context.Context, charID uuid.UUID, pact PactMagicSlotState) (SlotDeduction, error) {
	pact.Current--
	if err := s.persistPactMagicSlots(ctx, charID, pact); err != nil {
		return SlotDeduction{}, err
	}
	return SlotDeduction{SlotUsed: pact.SlotLevel, SlotsRemaining: pact.Current}, nil
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
	Dice            string `json:"dice"`
	DamageType      string `json:"type"`
	HigherLevelDice string `json:"higher_level_dice"`
	CantripScaling  bool   `json:"cantrip_scaling"`
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
	Dice            string `json:"dice"`
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

// resolveTeleport handles the teleportation portion of a spell cast: parses teleport data,
// validates destinations, and moves combatants on the grid.
func (s *Service) resolveTeleport(ctx context.Context, raw json.RawMessage, caster refdata.Combatant, cmd CastCommand) (*TeleportResult, error) {
	info, err := ParseTeleportInfo(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing teleport data: %w", err)
	}

	if IsDMQueueTeleport(info.Target) {
		return &TeleportResult{DMQueueRouted: true}, nil
	}

	occupants, err := s.store.ListCombatantsByEncounterID(ctx, cmd.EncounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants for teleport: %w", err)
	}

	var companion *refdata.Combatant
	if info.Target == TeleportTargetSelfCreature && cmd.CompanionID != uuid.Nil {
		comp, err := s.store.GetCombatant(ctx, cmd.CompanionID)
		if err != nil {
			return nil, fmt.Errorf("getting companion: %w", err)
		}
		companion = &comp
	}

	sight := TeleportSightOptions{Walls: cmd.Walls, FogOfWar: cmd.FogOfWar}
	if err := ValidateTeleportDestinationWithSight(info, caster, cmd.TeleportDestCol, cmd.TeleportDestRow, occupants, companion, sight); err != nil {
		return nil, err
	}

	result := &TeleportResult{
		AdditionalEffects: info.AdditionalEffects,
	}

	// Move caster for "self" and "self+creature" targets
	if info.Target == TeleportTargetSelf || info.Target == TeleportTargetSelfCreature {
		if _, err := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
			ID:          caster.ID,
			PositionCol: cmd.TeleportDestCol,
			PositionRow: cmd.TeleportDestRow,
			AltitudeFt:  caster.AltitudeFt,
		}); err != nil {
			return nil, fmt.Errorf("moving caster: %w", err)
		}
		result.CasterMoved = true
		result.CasterDestCol = cmd.TeleportDestCol
		result.CasterDestRow = cmd.TeleportDestRow
	}

	// Move companion for "self+creature" targets
	if info.Target == TeleportTargetSelfCreature && companion != nil && cmd.CompanionDestCol != "" {
		if _, err := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
			ID:          companion.ID,
			PositionCol: cmd.CompanionDestCol,
			PositionRow: cmd.CompanionDestRow,
			AltitudeFt:  companion.AltitudeFt,
		}); err != nil {
			return nil, fmt.Errorf("moving companion: %w", err)
		}
		result.CompanionMoved = true
		result.CompanionName = companion.DisplayName
		result.CompanionDestCol = cmd.CompanionDestCol
		result.CompanionDestRow = cmd.CompanionDestRow
	}

	return result, nil
}

// CastMaterialComponentInfo holds material component information returned from Cast.
type CastMaterialComponentInfo struct {
	NeedsGoldConfirmation bool    // true if the user must confirm gold purchase
	ComponentName         string  // description of the material component
	CostGp                float64 // gold cost
	CurrentGold           int32   // character's current gold
}

// MaterialCheckOutcome represents the result of a material component validation.
type MaterialCheckOutcome int

const (
	// MaterialCheckProceed means the cast can proceed (no costly component, or component found).
	MaterialCheckProceed MaterialCheckOutcome = iota
	// MaterialCheckNeedsGoldConfirmation means the component is missing but gold is sufficient.
	MaterialCheckNeedsGoldConfirmation
	// MaterialCheckRejected means neither component nor gold is available.
	MaterialCheckRejected
)

// MaterialComponentResult holds the outcome of a material component check.
type MaterialComponentResult struct {
	Outcome          MaterialCheckOutcome
	ComponentName    string  // material description
	CostGp           float64 // gold cost
	CurrentGold      int32   // character's current gold
	MaterialConsumed bool    // whether the material is consumed on cast
}

// normalizeComponentName strips leading articles, "worth (at least) N gp" suffixes,
// and normalizes casing/whitespace for material component comparison.
func normalizeComponentName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Strip leading articles
	for _, article := range []string{"a ", "an ", "the "} {
		if strings.HasPrefix(s, article) {
			s = s[len(article):]
			break
		}
	}
	// Strip "worth (at least) N gp" suffix and anything after
	if idx := strings.Index(s, " worth "); idx >= 0 {
		s = s[:idx]
	}
	// Strip ", which the spell consumes" and similar trailing clauses
	if idx := strings.Index(s, ", which "); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// matchesMaterialComponent checks whether an inventory item name matches a spell's
// material description using normalized comparison rather than exact string matching.
func matchesMaterialComponent(itemName, materialDesc string) bool {
	normItem := normalizeComponentName(itemName)
	normDesc := normalizeComponentName(materialDesc)
	if normItem == "" || normDesc == "" {
		return false
	}
	// Either the normalized item matches the normalized description,
	// or one contains the other (e.g., item "diamond" matches desc "diamond")
	return normItem == normDesc || strings.Contains(normDesc, normItem) || strings.Contains(normItem, normDesc)
}

// ValidateMaterialComponent checks whether a spell's material component requirements are met.
// Returns MaterialCheckProceed if no costly component or if the item is in inventory.
// Returns MaterialCheckNeedsGoldConfirmation if the item is missing but the caster has enough gold.
// Returns MaterialCheckRejected if neither component nor gold is available.
func ValidateMaterialComponent(spell refdata.Spell, inventory []InventoryItem, gold int32) MaterialComponentResult {
	if !spell.MaterialCostGp.Valid {
		return MaterialComponentResult{Outcome: MaterialCheckProceed}
	}

	desc := ""
	if spell.MaterialDescription.Valid {
		desc = spell.MaterialDescription.String
	}

	base := MaterialComponentResult{
		ComponentName:    desc,
		CostGp:           spell.MaterialCostGp.Float64,
		CurrentGold:      gold,
		MaterialConsumed: spell.MaterialConsumed.Valid && spell.MaterialConsumed.Bool,
	}

	// Check inventory for the required item (normalized comparison)
	for _, item := range inventory {
		if matchesMaterialComponent(item.Name, desc) && item.Quantity > 0 {
			base.Outcome = MaterialCheckProceed
			return base
		}
	}

	// No component found — check gold
	if gold >= int32(base.CostGp) {
		base.Outcome = MaterialCheckNeedsGoldConfirmation
		return base
	}

	// Neither component nor gold
	base.Outcome = MaterialCheckRejected
	return base
}

// FormatMaterialRejection formats the rejection message when a costly material component is missing.
func FormatMaterialRejection(r MaterialComponentResult) string {
	return fmt.Sprintf("Requires %s — you don't have one and can't afford it (current gold: %dgp).",
		r.ComponentName, r.CurrentGold)
}

// FormatGoldFallbackPrompt formats the gold fallback prompt message.
func FormatGoldFallbackPrompt(r MaterialComponentResult) string {
	return fmt.Sprintf("You don't have %s — buy one for %dgp?",
		r.ComponentName, int(r.CostGp))
}

// RemoveInventoryItem decrements the quantity of a named item by 1, removing it if quantity reaches 0.
// Uses normalized matching consistent with ValidateMaterialComponent.
func RemoveInventoryItem(items []InventoryItem, name string) []InventoryItem {
	result := make([]InventoryItem, 0, len(items))
	removed := false
	for _, item := range items {
		if removed || !matchesMaterialComponent(item.Name, name) {
			result = append(result, item)
			continue
		}
		removed = true
		if item.Quantity > 1 {
			item.Quantity--
			result = append(result, item)
		}
	}
	return result
}

// persistInventory marshals and saves the inventory to the database.
func (s *Service) persistInventory(ctx context.Context, charID uuid.UUID, items []InventoryItem) error {
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshaling inventory: %w", err)
	}
	return s.store.UpdateCharacterInventory(ctx, charID, pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true})
}

// AddInventoryItem adds an item to the inventory. If an item with the same name exists, increments quantity.
func AddInventoryItem(items []InventoryItem, name string) []InventoryItem {
	for i := range items {
		if strings.EqualFold(items[i].Name, name) {
			result := make([]InventoryItem, len(items))
			copy(result, items)
			result[i].Quantity++
			return result
		}
	}
	return append(items, InventoryItem{Name: name, Quantity: 1, Type: "component"})
}

// druidLevelFromClasses returns the Druid class level (case-insensitive) or
// 0 when the character has no Druid level. Used by med-43 / Phase 47 to
// gate Wild Shape spellcasting on Beast Spells (level 18+).
func druidLevelFromClasses(classes []CharacterClass) int {
	for _, cc := range classes {
		if strings.EqualFold(cc.Class, "druid") {
			return cc.Level
		}
	}
	return 0
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
