package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// EquipCommand holds the inputs for the /equip command.
type EquipCommand struct {
	Character refdata.Character
	Combatant *refdata.Combatant  // nil if out of combat
	Turn      *refdata.Turn       // nil if out of combat
	Encounter *refdata.Encounter  // nil if out of combat
	ItemName  string
	Offhand   bool
	Armor     bool
}

// EquipResult holds the outputs of the /equip command.
type EquipResult struct {
	Character refdata.Character
	Turn      *refdata.Turn
	CombatLog string
	ACChanged bool
	OldAC     int32
	NewAC     int32
}

// HasFreeHand returns true if either main hand or off-hand is empty.
func HasFreeHand(char refdata.Character) bool {
	mainOccupied := char.EquippedMainHand.Valid && char.EquippedMainHand.String != ""
	offOccupied := char.EquippedOffHand.Valid && char.EquippedOffHand.String != ""
	return !mainOccupied || !offOccupied
}

// BothHandsOccupied returns true if both main hand and off-hand are occupied.
func BothHandsOccupied(char refdata.Character) bool {
	return !HasFreeHand(char)
}

// Equip handles the /equip command for weapons, shields, and armor.
func (s *Service) Equip(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
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
	if HasProperty(weapon, "two-handed") && char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
		return EquipResult{}, fmt.Errorf("cannot equip %s — off-hand must be free for two-handed weapons", weapon.Name)
	}

	// In combat: weapon equip costs free object interaction
	if cmd.Turn != nil {
		if err := ValidateResource(*cmd.Turn, ResourceFreeInteract); err != nil {
			return EquipResult{}, fmt.Errorf("Free object interaction already used this turn.")
		}
		updatedTurn, err := UseResource(*cmd.Turn, ResourceFreeInteract)
		if err != nil {
			return EquipResult{}, err
		}
		if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
			return EquipResult{}, fmt.Errorf("updating turn actions: %w", err)
		}
		cmd.Turn = &updatedTurn
	}

	// Set the equipped slot
	slot := "main hand"
	if cmd.Offhand {
		// If off-hand has a shield, must doff it first
		if char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
			offArmor, err := s.store.GetArmor(ctx, char.EquippedOffHand.String)
			if err == nil && offArmor.ArmorType == "shield" {
				return EquipResult{}, fmt.Errorf("off-hand has a shield equipped — doff the shield first (requires action in combat)")
			}
		}
		char.EquippedOffHand = sql.NullString{String: weapon.ID, Valid: true}
		slot = "off-hand"
	} else {
		char.EquippedMainHand = sql.NullString{String: weapon.ID, Valid: true}
	}

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
		ID:               char.ID,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		Ac:               char.Ac,
	})
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("⚔️ %s equips %s (%s)", char.Name, weapon.Name, slot)

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: combatLog,
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
		if err := ValidateResource(*cmd.Turn, ResourceAction); err != nil {
			return EquipResult{}, fmt.Errorf("Action already used — donning/doffing a shield requires an action.")
		}
		updatedTurn, err := UseResource(*cmd.Turn, ResourceAction)
		if err != nil {
			return EquipResult{}, err
		}
		if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
			return EquipResult{}, fmt.Errorf("updating turn actions: %w", err)
		}
		cmd.Turn = &updatedTurn
	}

	// If off-hand has a weapon, stow it automatically (no extra cost)
	if char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
		// Check if it's not already a shield
		existingArmor, err := s.store.GetArmor(ctx, char.EquippedOffHand.String)
		if err != nil || existingArmor.ArmorType != "shield" {
			// It's a weapon — stow it
		}
	}

	char.EquippedOffHand = sql.NullString{String: armor.ID, Valid: true}
	newAC := oldAC + 2 // Shield gives +2 AC

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
		ID:               char.ID,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		Ac:               newAC,
	})
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("🛡️ %s equips %s (off-hand, AC %d → %d)", char.Name, armor.Name, oldAC, newAC)

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: combatLog,
		ACChanged: true,
		OldAC:     oldAC,
		NewAC:     newAC,
	}, nil
}

func (s *Service) equipArmor(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	// Blocked in combat
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

	// Calculate new AC based on armor
	newAC := s.calculateArmorAC(char, armor)

	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
		ID:               char.ID,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		Ac:               newAC,
	})
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("🛡️ %s dons %s (AC %d → %d)", char.Name, armor.Name, oldAC, newAC)

	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: combatLog,
		ACChanged: oldAC != newAC,
		OldAC:     oldAC,
		NewAC:     newAC,
	}, nil
}

func (s *Service) unequip(ctx context.Context, cmd EquipCommand) (EquipResult, error) {
	char := cmd.Character
	oldAC := char.Ac

	if cmd.Armor {
		// Unequip armor
		if cmd.Turn != nil {
			return EquipResult{}, fmt.Errorf("You can't don or doff armor during combat.")
		}
		char.EquippedArmor = sql.NullString{}
		// Recalculate AC: base 10 + DEX mod
		newAC := s.calculateBaseAC(char)

		// Add shield bonus if shield is equipped
		if char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
			shieldArmor, err := s.store.GetArmor(ctx, char.EquippedOffHand.String)
			if err == nil && shieldArmor.ArmorType == "shield" {
				newAC += 2
			}
		}

		updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
			ID:               char.ID,
			EquippedMainHand: char.EquippedMainHand,
			EquippedOffHand:  char.EquippedOffHand,
			EquippedArmor:    char.EquippedArmor,
			Ac:               newAC,
		})
		if err != nil {
			return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
		}

		combatLog := fmt.Sprintf("🛡️ %s doffs armor (AC %d → %d)", char.Name, oldAC, newAC)
		return EquipResult{
			Character: updatedChar,
			Turn:      cmd.Turn,
			CombatLog: combatLog,
			ACChanged: oldAC != newAC,
			OldAC:     oldAC,
			NewAC:     newAC,
		}, nil
	}

	if cmd.Offhand {
		// Unequip off-hand
		// Check if it's a shield — shield doff costs action in combat
		if char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
			shieldArmor, err := s.store.GetArmor(ctx, char.EquippedOffHand.String)
			isShield := err == nil && shieldArmor.ArmorType == "shield"

			if isShield && cmd.Turn != nil {
				if err := ValidateResource(*cmd.Turn, ResourceAction); err != nil {
					return EquipResult{}, fmt.Errorf("Action already used — donning/doffing a shield requires an action.")
				}
				updatedTurn, err := UseResource(*cmd.Turn, ResourceAction)
				if err != nil {
					return EquipResult{}, err
				}
				if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
					return EquipResult{}, fmt.Errorf("updating turn actions: %w", err)
				}
				cmd.Turn = &updatedTurn
			} else if !isShield && cmd.Turn != nil {
				// Weapon unequip costs free interact
				if err := ValidateResource(*cmd.Turn, ResourceFreeInteract); err != nil {
					return EquipResult{}, fmt.Errorf("Free object interaction already used this turn.")
				}
				updatedTurn, err := UseResource(*cmd.Turn, ResourceFreeInteract)
				if err != nil {
					return EquipResult{}, err
				}
				if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
					return EquipResult{}, fmt.Errorf("updating turn actions: %w", err)
				}
				cmd.Turn = &updatedTurn
			}

			newAC := oldAC
			if isShield {
				newAC = oldAC - 2
			}

			char.EquippedOffHand = sql.NullString{}
			updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
				ID:               char.ID,
				EquippedMainHand: char.EquippedMainHand,
				EquippedOffHand:  char.EquippedOffHand,
				EquippedArmor:    char.EquippedArmor,
				Ac:               newAC,
			})
			if err != nil {
				return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
			}

			var combatLog string
			if isShield {
				combatLog = fmt.Sprintf("🛡️ %s doffs shield (AC %d → %d)", char.Name, oldAC, newAC)
			} else {
				combatLog = fmt.Sprintf("⚔️ %s unequips off-hand weapon", char.Name)
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

		// Off-hand already empty
		return EquipResult{
			Character: char,
			CombatLog: fmt.Sprintf("⚔️ %s — off-hand is already empty", char.Name),
		}, nil
	}

	// Unequip main hand (defaults to unarmed)
	if cmd.Turn != nil {
		// Stowing a weapon costs free interact
		if char.EquippedMainHand.Valid && char.EquippedMainHand.String != "" {
			if err := ValidateResource(*cmd.Turn, ResourceFreeInteract); err != nil {
				return EquipResult{}, fmt.Errorf("Free object interaction already used this turn.")
			}
			updatedTurn, err := UseResource(*cmd.Turn, ResourceFreeInteract)
			if err != nil {
				return EquipResult{}, err
			}
			if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
				return EquipResult{}, fmt.Errorf("updating turn actions: %w", err)
			}
			cmd.Turn = &updatedTurn
		}
	}

	char.EquippedMainHand = sql.NullString{}
	updatedChar, err := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
		ID:               char.ID,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		Ac:               char.Ac,
	})
	if err != nil {
		return EquipResult{}, fmt.Errorf("updating character equipment: %w", err)
	}

	combatLog := fmt.Sprintf("⚔️ %s unequips main hand (unarmed strike)", char.Name)
	return EquipResult{
		Character: updatedChar,
		Turn:      cmd.Turn,
		CombatLog: combatLog,
	}, nil
}

// calculateArmorAC computes AC for body armor + DEX bonus.
func (s *Service) calculateArmorAC(char refdata.Character, armor refdata.Armor) int32 {
	ac := armor.AcBase
	dexMod := int32(getDexMod(char))

	if armor.AcDexBonus.Valid && armor.AcDexBonus.Bool {
		if armor.AcDexMax.Valid && armor.AcDexMax.Int32 > 0 {
			if dexMod > armor.AcDexMax.Int32 {
				dexMod = armor.AcDexMax.Int32
			}
		}
		ac += dexMod
	}

	// Add shield bonus if shield is equipped
	if char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
		// We'd need to check if off-hand is a shield, but for simplicity
		// we look up the armor record
		shieldArmor, err := s.store.GetArmor(context.Background(), char.EquippedOffHand.String)
		if err == nil && shieldArmor.ArmorType == "shield" {
			ac += 2
		}
	}

	return ac
}

// calculateBaseAC computes base AC (10 + DEX mod) or uses ac_formula if set.
func (s *Service) calculateBaseAC(char refdata.Character) int32 {
	return 10 + int32(getDexMod(char))
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

// getDexMod extracts the DEX modifier from ability scores.
func getDexMod(char refdata.Character) int {
	scores := parseAbilityScores(char.AbilityScores)
	return AbilityModifier(scores.Dex)
}

// parseAbilityScores parses the JSON ability scores.
func parseAbilityScores(raw json.RawMessage) AbilityScores {
	var scores AbilityScores
	_ = json.Unmarshal(raw, &scores)
	return scores
}
