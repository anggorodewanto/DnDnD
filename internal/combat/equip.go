package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// EquipCommand holds the inputs for the /equip command.
type EquipCommand struct {
	Character refdata.Character
	Combatant *refdata.Combatant // nil if out of combat
	Turn      *refdata.Turn      // nil if out of combat
	ItemName  string
	Offhand   bool
	Armor     bool
}

// EquipResult holds the outputs of the /equip command.
type EquipResult struct {
	Character    refdata.Character
	Turn         *refdata.Turn
	CombatLog    string
	ACChanged    bool
	OldAC        int32
	NewAC        int32
	SpeedPenalty int32 // heavy armor STR penalty (10ft if STR below requirement)
}

// slotOccupied returns true if a NullString equipment slot has a value.
func slotOccupied(slot sql.NullString) bool {
	return slot.Valid && slot.String != ""
}

// HasFreeHand returns true if either main hand or off-hand is empty.
func HasFreeHand(char refdata.Character) bool {
	return !slotOccupied(char.EquippedMainHand) || !slotOccupied(char.EquippedOffHand)
}

// BothHandsOccupied returns true if both main hand and off-hand are occupied.
func BothHandsOccupied(char refdata.Character) bool {
	return !HasFreeHand(char)
}

// useResourceAndSave validates a turn resource, consumes it, and persists the update.
// userMsg is the error message shown when the resource is unavailable.
func (s *Service) useResourceAndSave(ctx context.Context, turn refdata.Turn, resource ResourceType, userMsg string) (refdata.Turn, error) {
	if err := ValidateResource(turn, resource); err != nil {
		return turn, errors.New(userMsg)
	}
	updated, err := UseResource(turn, resource)
	if err != nil {
		return turn, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updated)); err != nil {
		return turn, fmt.Errorf("updating turn actions: %w", err)
	}
	return updated, nil
}

// hasEquippedShield checks whether the character's off-hand slot holds a shield.
func (s *Service) hasEquippedShield(ctx context.Context, char refdata.Character) bool {
	if !slotOccupied(char.EquippedOffHand) {
		return false
	}
	a, err := s.store.GetArmor(ctx, char.EquippedOffHand.String)
	return err == nil && a.ArmorType == "shield"
}

// lookupBodyArmor returns the equipped body armor (non-shield), or nil if none.
func (s *Service) lookupBodyArmor(ctx context.Context, char refdata.Character) *refdata.Armor {
	if !slotOccupied(char.EquippedArmor) {
		return nil
	}
	a, err := s.store.GetArmor(ctx, char.EquippedArmor.String)
	if err != nil || a.ArmorType == "shield" {
		return nil
	}
	return &a
}

// equipUpdateParams builds the standard UpdateCharacterEquipmentParams from a character.
func equipUpdateParams(char refdata.Character, ac int32) refdata.UpdateCharacterEquipmentParams {
	return refdata.UpdateCharacterEquipmentParams{
		ID:               char.ID,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		Ac:               ac,
	}
}

// Equip handles the /equip command for weapons, shields, and armor.
//
// SR-007: on a successful equipment write the character card is refreshed via
// the CardUpdater hook so the persistent #character-cards message keeps the
// "Equipped:" / AC lines in sync with the just-written `equipped_*` columns.
// The fan-out is best-effort — a Discord hiccup must not undo the DB write.
func (s *Service) Equip(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	result, err := s.dispatchEquip(ctx, cmd)
	if err != nil {
		return result, err
	}
	s.notifyCardUpdateByCharacterID(ctx, result.Character.ID)
	return result, nil
}

// dispatchEquip routes the EquipCommand to the matching sub-handler. Kept
// separate from Equip so the single post-success card-update fan-out can wrap
// every branch.
func (s *Service) dispatchEquip(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	// Handle unequip ("none")
	if strings.EqualFold(cmd.ItemName, "none") {
		return s.unequip(ctx, cmd)
	}

	if cmd.Armor {
		return s.equipArmor(ctx, cmd)
	}

	// Try shield first
	armor, err := s.store.GetArmor(ctx, cmd.ItemName)
	if err == nil && armor.ArmorType == "shield" {
		return s.equipShield(ctx, cmd, armor)
	}

	// Try weapon
	weapon, err := s.store.GetWeapon(ctx, cmd.ItemName)
	if err != nil {
		return EquipResult{}, fmt.Errorf("item %q not found", cmd.ItemName)
	}

	return s.equipWeapon(ctx, cmd, weapon)
}

func (s *Service) equipWeapon(ctx context.Context, cmd EquipCommand, weapon refdata.Weapon) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	// Two-handed weapon: off-hand must be free
	if HasProperty(weapon, "two-handed") && slotOccupied(char.EquippedOffHand) {
		return EquipResult{}, fmt.Errorf("cannot equip %s — off-hand must be free for two-handed weapons", weapon.Name)
	}

	// In combat: weapon equip costs free object interaction
	if cmd.Turn != nil {
		updatedTurn, err := s.useResourceAndSave(ctx, *cmd.Turn, ResourceFreeInteract, "Free object interaction already used this turn.")
		if err != nil {
			return EquipResult{}, err
		}
		cmd.Turn = &updatedTurn
	}

	// Set the equipped slot
	slot := "main hand"
	if cmd.Offhand {
		if s.hasEquippedShield(ctx, char) {
			return EquipResult{}, fmt.Errorf("off-hand has a shield equipped — doff the shield first (requires action in combat)")
		}
		char.EquippedOffHand = sql.NullString{String: weapon.ID, Valid: true}
		slot = "off-hand"
	} else {
		char.EquippedMainHand = sql.NullString{String: weapon.ID, Valid: true}
	}

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, char.Ac))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: fmt.Sprintf("⚔️ %s equips %s (%s)", char.Name, weapon.Name, slot),
		ACChanged: oldAC != char.Ac,
		OldAC:     oldAC,
		NewAC:     char.Ac,
	}, nil
}

func (s *Service) equipShield(ctx context.Context, cmd EquipCommand, armor refdata.Armor) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	// In combat: shield don/doff costs action
	if cmd.Turn != nil {
		updatedTurn, err := s.useResourceAndSave(ctx, *cmd.Turn, ResourceAction, "Action already used — donning/doffing a shield requires an action.")
		if err != nil {
			return EquipResult{}, err
		}
		cmd.Turn = &updatedTurn
	}

	// Off-hand weapon (if any) is automatically stowed when equipping a shield (no extra cost).
	char.EquippedOffHand = sql.NullString{String: armor.ID, Valid: true}

	newAC := RecalculateAC(char, s.lookupBodyArmor(ctx, char), true)

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, newAC))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: fmt.Sprintf("🛡️ %s equips %s (off-hand, AC %d → %d)", char.Name, armor.Name, oldAC, newAC),
		ACChanged: oldAC != newAC,
		OldAC:     oldAC,
		NewAC:     newAC,
	}, nil
}

func (s *Service) equipArmor(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	if cmd.Turn != nil {
		return EquipResult{}, fmt.Errorf("You can't don or doff armor during combat.")
	}

	armor, err := s.store.GetArmor(ctx, cmd.ItemName)
	if err != nil {
		return EquipResult{}, fmt.Errorf("armor %q not found", cmd.ItemName)
	}
	if armor.ArmorType == "shield" {
		return EquipResult{}, fmt.Errorf("use /equip %s without --armor for shields", cmd.ItemName)
	}

	char.EquippedArmor = sql.NullString{String: armor.ID, Valid: true}
	newAC := RecalculateAC(char, &armor, s.hasEquippedShield(ctx, char))
	speedPenalty := CheckHeavyArmorPenalty(char, armor)

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, newAC))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("🛡️ %s dons %s (AC %d → %d)", char.Name, armor.Name, oldAC, newAC)
	if speedPenalty > 0 {
		combatLog += fmt.Sprintf(" ⚠️ speed reduced by %dft (STR below %d)", speedPenalty, armor.StrengthReq.Int32)
	}

	return EquipResult{
		Character:    updatedChar,
		Turn:         cmd.Turn,
		CombatLog:    combatLog,
		ACChanged:    oldAC != newAC,
		OldAC:        oldAC,
		NewAC:        newAC,
		SpeedPenalty: speedPenalty,
	}, nil
}

func (s *Service) unequip(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	if cmd.Armor {
		return s.unequipArmor(ctx, cmd)
	}
	if cmd.Offhand {
		return s.unequipOffHand(ctx, cmd)
	}
	return s.unequipMainHand(ctx, cmd)
}

func (s *Service) unequipArmor(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	if cmd.Turn != nil {
		return EquipResult{}, fmt.Errorf("You can't don or doff armor during combat.")
	}

	char := cmd.Character
	oldAC := char.Ac
	char.EquippedArmor = sql.NullString{}
	newAC := RecalculateAC(char, nil, s.hasEquippedShield(ctx, char))

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, newAC))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: fmt.Sprintf("🛡️ %s doffs armor (AC %d → %d)", char.Name, oldAC, newAC),
		ACChanged: oldAC != newAC,
		OldAC:     oldAC,
		NewAC:     newAC,
	}, nil
}

func (s *Service) unequipOffHand(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	if !slotOccupied(char.EquippedOffHand) {
		return EquipResult{
			Character: char,
			CombatLog: fmt.Sprintf("⚔️ %s — off-hand is already empty", char.Name),
		}, nil
	}

	isShield := s.hasEquippedShield(ctx, char)

	// In combat: shield doff costs action, weapon stow costs free interact
	if cmd.Turn != nil {
		if isShield {
			updatedTurn, err := s.useResourceAndSave(ctx, *cmd.Turn, ResourceAction, "Action already used — donning/doffing a shield requires an action.")
			if err != nil {
				return EquipResult{}, err
			}
			cmd.Turn = &updatedTurn
		} else {
			updatedTurn, err := s.useResourceAndSave(ctx, *cmd.Turn, ResourceFreeInteract, "Free object interaction already used this turn.")
			if err != nil {
				return EquipResult{}, err
			}
			cmd.Turn = &updatedTurn
		}
	}

	char.EquippedOffHand = sql.NullString{}

	newAC := oldAC
	if isShield {
		newAC = RecalculateAC(char, s.lookupBodyArmor(ctx, char), false)
	}

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, newAC))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("⚔️ %s unequips off-hand weapon", char.Name)
	if isShield {
		combatLog = fmt.Sprintf("🛡️ %s doffs shield (AC %d → %d)", char.Name, oldAC, newAC)
	}

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: combatLog,
		ACChanged: oldAC != newAC,
		OldAC:     oldAC,
		NewAC:     newAC,
	}, nil
}

func (s *Service) unequipMainHand(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	char := cmd.Character

	// Stowing a weapon costs free interact in combat
	if cmd.Turn != nil && slotOccupied(char.EquippedMainHand) {
		updatedTurn, err := s.useResourceAndSave(ctx, *cmd.Turn, ResourceFreeInteract, "Free object interaction already used this turn.")
		if err != nil {
			return EquipResult{}, err
		}
		cmd.Turn = &updatedTurn
	}

	char.EquippedMainHand = sql.NullString{}
	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, equipUpdateParams(char, char.Ac))
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: fmt.Sprintf("⚔️ %s unequips main hand (unarmed strike)", char.Name),
	}, nil
}

// RecalculateAC computes the cached AC for a character based on equipped armor,
// ac_formula (Unarmored Defense / Natural Armor), and shield. Does NOT include
// modify_ac effects — those are applied at resolution time.
//
// Rules:
//   - If armor is equipped, use armor AC (base + DEX, capped as appropriate)
//   - If no armor and ac_formula is set, take max of base AC (10+DEX) and formula AC
//   - If no armor and no formula, use base AC (10+DEX)
//   - Shield adds +2 in all cases
func RecalculateAC(char refdata.Character, armor *refdata.Armor, hasShield bool) int32 {
	scores := parseAbilityScores(char.AbilityScores)
	dexMod := int32(AbilityModifier(scores.Dex))

	var ac int32
	if armor != nil {
		// Armor-based AC
		ac = armor.AcBase
		if armor.AcDexBonus.Valid && armor.AcDexBonus.Bool {
			cappedDex := dexMod
			if armor.AcDexMax.Valid && armor.AcDexMax.Int32 > 0 {
				if cappedDex > armor.AcDexMax.Int32 {
					cappedDex = armor.AcDexMax.Int32
				}
			}
			ac += cappedDex
		}
	} else {
		// No armor: base AC
		ac = 10 + dexMod

		// Check ac_formula (Unarmored Defense)
		if char.AcFormula.Valid && char.AcFormula.String != "" {
			formulaAC := evaluateACFormula(scores, char.AcFormula.String)
			if formulaAC > ac {
				ac = formulaAC
			}
		}
	}

	if hasShield {
		ac += 2
	}

	return ac
}

// evaluateACFormula parses formulas like "10 + DEX + WIS" against combat ability scores.
func evaluateACFormula(scores AbilityScores, formula string) int32 {
	var result int32
	parts := strings.Fields(strings.ReplaceAll(formula, "+", " "))
	for _, part := range parts {
		switch strings.ToUpper(part) {
		case "STR":
			result += int32(AbilityModifier(scores.Str))
		case "DEX":
			result += int32(AbilityModifier(scores.Dex))
		case "CON":
			result += int32(AbilityModifier(scores.Con))
		case "INT":
			result += int32(AbilityModifier(scores.Int))
		case "WIS":
			result += int32(AbilityModifier(scores.Wis))
		case "CHA":
			result += int32(AbilityModifier(scores.Cha))
		default:
			var n int32
			fmt.Sscanf(part, "%d", &n)
			result += n
		}
	}
	return result
}

// CheckHeavyArmorPenalty returns the speed penalty (in feet) for a character
// whose STR score is below the armor's strength_req. Returns 0 if no penalty.
func CheckHeavyArmorPenalty(char refdata.Character, armor refdata.Armor) int32 {
	if !armor.StrengthReq.Valid || armor.StrengthReq.Int32 <= 0 {
		return 0
	}
	scores := parseAbilityScores(char.AbilityScores)
	if int32(scores.Str) >= armor.StrengthReq.Int32 {
		return 0
	}
	return 10
}

// CheckSomaticComponent validates that a character has a free hand for somatic
// spell components. Returns nil if the character has a free hand, or if the
// spell has no somatic component. The War Caster feat allows somatic components
// even with both hands occupied.
func CheckSomaticComponent(char refdata.Character, spell refdata.Spell, hasWarCaster bool) error {
	hasSomatic := false
	for _, c := range spell.Components {
		if c == "S" {
			hasSomatic = true
			break
		}
	}
	if !hasSomatic {
		return nil
	}
	if hasWarCaster {
		return nil
	}
	if HasFreeHand(char) {
		return nil
	}
	return fmt.Errorf("cannot cast %s — somatic component requires a free hand", spell.Name)
}

// parseAbilityScores parses the JSON ability scores.
func parseAbilityScores(raw json.RawMessage) AbilityScores {
	var scores AbilityScores
	_ = json.Unmarshal(raw, &scores)
	return scores
}
