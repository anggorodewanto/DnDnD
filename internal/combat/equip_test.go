package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func makeEquipChar(charID uuid.UUID) refdata.Character {
	return refdata.Character{
		ID:            charID,
		Name:          "Tester",
		Ac:            10,
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
	}
}

func setupEquipMock(ms *mockStore, char refdata.Character) {
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		c := char
		c.EquippedMainHand = arg.EquippedMainHand
		c.EquippedOffHand = arg.EquippedOffHand
		c.EquippedArmor = arg.EquippedArmor
		c.Ac = arg.Ac
		return c, nil
	}
}

func longswordFn(ctx context.Context, id string) (refdata.Weapon, error) {
	if id == "longsword" {
		return refdata.Weapon{ID: "longsword", Name: "Longsword", Properties: []string{"versatile"}}, nil
	}
	return refdata.Weapon{}, fmt.Errorf("not found")
}

func shortswordFn(ctx context.Context, id string) (refdata.Weapon, error) {
	if id == "shortsword" {
		return refdata.Weapon{ID: "shortsword", Name: "Shortsword", Properties: []string{"light", "finesse"}}, nil
	}
	return refdata.Weapon{}, fmt.Errorf("not found")
}

func greatswordFn(ctx context.Context, id string) (refdata.Weapon, error) {
	if id == "greatsword" {
		return refdata.Weapon{ID: "greatsword", Name: "Greatsword", Properties: []string{"heavy", "two-handed"}}, nil
	}
	return refdata.Weapon{}, fmt.Errorf("not found")
}

// TDD Cycle 1: Equip weapon to main hand out of combat
func TestEquip_WeaponMainHand_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "longsword",
	})

	require.NoError(t, err)
	assert.Equal(t, "longsword", result.Character.EquippedMainHand.String)
	assert.Contains(t, result.CombatLog, "Longsword")
	assert.Contains(t, result.CombatLog, "main hand")
}

// TDD Cycle 2: Equip weapon to off-hand
func TestEquip_WeaponOffHand_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = shortswordFn
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shortsword",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.Equal(t, "shortsword", result.Character.EquippedOffHand.String)
	assert.Contains(t, result.CombatLog, "off-hand")
}

// TDD Cycle 3: Unequip main hand
func TestEquip_UnequipMainHand_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
	})

	require.NoError(t, err)
	assert.False(t, result.Character.EquippedMainHand.Valid)
	assert.Contains(t, result.CombatLog, "unarmed strike")
}

// TDD Cycle 4: Unequip off-hand
func TestEquip_UnequipOffHand_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shortsword", Valid: true}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.False(t, result.Character.EquippedOffHand.Valid)
	assert.Contains(t, result.CombatLog, "unequips off-hand")
}

// TDD Cycle 5: Equip weapon in combat costs free interact
func TestEquip_WeaponInCombat_CostsFreeInteract(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn
	setupEquipMock(ms, char)

	turn := makeBasicTurn()

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "longsword",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.FreeInteractUsed)
	assert.False(t, result.Turn.ActionUsed)
}

// TDD Cycle 6: Equip weapon in combat rejects if free interact already spent
func TestEquip_WeaponInCombat_FreeInteractSpent(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn

	turn := makeBasicTurn()
	turn.FreeInteractUsed = true

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Free object interaction already used")
}

// TDD Cycle 7: Equip shield out of combat
func TestEquip_Shield_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.Ac = 12
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shield",
	})

	require.NoError(t, err)
	assert.Equal(t, "shield", result.Character.EquippedOffHand.String)
	assert.True(t, result.ACChanged)
	assert.Equal(t, int32(12), result.OldAC)
	assert.Equal(t, int32(14), result.NewAC)
}

// TDD Cycle 8: Equip shield in combat costs action
func TestEquip_Shield_InCombat_CostsAction(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	turn := makeBasicTurn()

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "shield",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed, "donning shield should cost action")
}

// TDD Cycle 9: Shield in combat rejects if action spent
func TestEquip_Shield_InCombat_ActionSpent(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}

	turn := makeBasicTurn()
	turn.ActionUsed = true

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "shield",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Action already used")
	assert.Contains(t, err.Error(), "shield requires an action")
}

// TDD Cycle 10: Equip armor out of combat
func TestEquip_Armor_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "chain-mail" {
			return refdata.Armor{
				ID:        "chain-mail",
				Name:      "Chain Mail",
				ArmorType: "heavy",
				AcBase:    16,
				AcDexBonus: sql.NullBool{Bool: false, Valid: true},
			}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "chain-mail",
		Armor:     true,
	})

	require.NoError(t, err)
	assert.Equal(t, "chain-mail", result.Character.EquippedArmor.String)
	assert.True(t, result.ACChanged)
	assert.Equal(t, int32(16), result.NewAC)
}

// TDD Cycle 11: Equip armor blocked in combat
func TestEquip_Armor_BlockedInCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "chain-mail",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't don or doff armor during combat")
}

// TDD Cycle 12: Two-handed weapon with occupied off-hand
func TestEquip_TwoHandedWeapon_OffHandOccupied(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getWeaponFn = greatswordFn

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "greatsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "off-hand must be free for two-handed")
}

// TDD Cycle 13: HasFreeHand / BothHandsOccupied helpers
func TestHasFreeHand(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		char := refdata.Character{}
		assert.True(t, HasFreeHand(char))
		assert.False(t, BothHandsOccupied(char))
	})
	t.Run("main only", func(t *testing.T) {
		char := refdata.Character{
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
		}
		assert.True(t, HasFreeHand(char))
		assert.False(t, BothHandsOccupied(char))
	})
	t.Run("both occupied", func(t *testing.T) {
		char := refdata.Character{
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			EquippedOffHand:  sql.NullString{String: "shield", Valid: true},
		}
		assert.False(t, HasFreeHand(char))
		assert.True(t, BothHandsOccupied(char))
	})
}

// TDD Cycle 14: Unequip shield recalculates AC
func TestEquip_UnequipShield_ACRecalc(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.Ac = 14 // includes +2 from shield
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.True(t, result.ACChanged)
	assert.Equal(t, int32(14), result.OldAC)
	assert.Equal(t, int32(12), result.NewAC)
}

// TDD Cycle 15: Shield doff in combat costs action
func TestEquip_DoffShield_InCombat_CostsAction(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.Ac = 14
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	turn := makeBasicTurn()

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed, "doffing shield should cost action")
}

// TDD Cycle 16: Unequip armor out of combat
func TestEquip_UnequipArmor_OutOfCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.Ac = 16
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Armor:     true,
	})

	require.NoError(t, err)
	assert.False(t, result.Character.EquippedArmor.Valid)
	// 10 + DEX mod(14) = 12
	assert.Equal(t, int32(12), result.NewAC)
}

// TDD Cycle 17: Unequip armor blocked in combat
func TestEquip_UnequipArmor_BlockedInCombat(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}
	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't don or doff armor during combat")
}

// TDD Cycle 18: Equip shield auto-stows off-hand weapon
func TestEquip_Shield_AutoStowsOffHandWeapon(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shortsword", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		if id == "shortsword" {
			return refdata.Armor{}, fmt.Errorf("not found")
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shield",
	})

	require.NoError(t, err)
	assert.Equal(t, "shield", result.Character.EquippedOffHand.String)
}

// TDD Cycle 19: Off-hand weapon while shield equipped requires doffing first
func TestEquip_OffHandWeapon_ShieldEquipped_RequiresDoff(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getWeaponFn = shortswordFn
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shortsword",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "doff the shield first")
}

// TDD Cycle 20: Item not found
func TestEquip_ItemNotFound(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "nonexistent-weapon",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TDD Cycle 21: Unequip main hand in combat costs free interact
func TestEquip_UnequipMainHand_InCombat_CostsFreeInteract(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	setupEquipMock(ms, char)

	turn := makeBasicTurn()

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.FreeInteractUsed)
}

// TDD Cycle 22: Unequip main hand in combat rejects if free interact spent
func TestEquip_UnequipMainHand_InCombat_FreeInteractSpent(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}

	turn := makeBasicTurn()
	turn.FreeInteractUsed = true

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Free object interaction already used")
}

// TDD Cycle 23: Unequip off-hand weapon in combat costs free interact
func TestEquip_UnequipOffHandWeapon_InCombat_CostsFreeInteract(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shortsword", Valid: true}
	setupEquipMock(ms, char)
	// shortsword is not armor, so GetArmor will fail
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{}, fmt.Errorf("not found")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.FreeInteractUsed)
	assert.False(t, result.Turn.ActionUsed)
}

// TDD Cycle 24: Armor with DEX bonus (medium armor)
func TestEquip_MediumArmor_DexCapped(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID) // DEX 14 = +2 mod
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "half-plate" {
			return refdata.Armor{
				ID:         "half-plate",
				Name:       "Half Plate",
				ArmorType:  "medium",
				AcBase:     15,
				AcDexBonus: sql.NullBool{Bool: true, Valid: true},
				AcDexMax:   sql.NullInt32{Int32: 2, Valid: true},
			}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "half-plate",
		Armor:     true,
	})

	require.NoError(t, err)
	// AC = 15 + min(2, 2) = 17
	assert.Equal(t, int32(17), result.NewAC)
}

// TDD Cycle 25: Doff shield in combat with action already spent
func TestEquip_DoffShield_InCombat_ActionSpent(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}

	turn := makeBasicTurn()
	turn.ActionUsed = true

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Action already used")
}

// TDD Cycle 26: Off-hand already empty
func TestEquip_UnequipOffHand_AlreadyEmpty(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Offhand:   true,
	})

	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "already empty")
}

// TDD Cycle 27: DB error on UpdateCharacterEquipment
func TestEquip_UpdateCharacterEquipment_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 28: Light armor with full DEX bonus
func TestEquip_LightArmor_FullDex(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID) // DEX 14 = +2
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "leather" {
			return refdata.Armor{
				ID:         "leather",
				Name:       "Leather Armor",
				ArmorType:  "light",
				AcBase:     11,
				AcDexBonus: sql.NullBool{Bool: true, Valid: true},
			}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "leather",
		Armor:     true,
	})

	require.NoError(t, err)
	// AC = 11 + 2 = 13
	assert.Equal(t, int32(13), result.NewAC)
}

// TDD Cycle 29: Shield equip DB error
func TestEquip_Shield_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
	}
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shield",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 30: Armor equip DB error
func TestEquip_Armor_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "chain-mail", Name: "Chain Mail", ArmorType: "heavy", AcBase: 16, AcDexBonus: sql.NullBool{Bool: false, Valid: true}}, nil
	}
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "chain-mail",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 31: Armor not found
func TestEquip_Armor_NotFound(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "nonexistent",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TDD Cycle 32: Shield via --armor flag redirects
func TestEquip_ShieldViaArmorFlag(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "shield",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "without --armor")
}

// TDD Cycle 33: Unequip main hand DB error
func TestEquip_UnequipMainHand_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 34: Unequip off-hand with shield DB error
func TestEquip_UnequipOffHandShield_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	char.Ac = 14
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
	}
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 35: Unequip armor DB error
func TestEquip_UnequipArmor_DBError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}
	ms.updateCharacterEquipmentFn = func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Armor:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating character equipment")
}

// TDD Cycle 36: Equip weapon in combat with UpdateTurnActions error
func TestEquip_WeaponInCombat_UpdateTurnError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 37: Equip shield in combat with UpdateTurnActions error
func TestEquip_ShieldInCombat_UpdateTurnError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "shield",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 38: Armor with shield equipped includes shield bonus in AC calc
func TestEquip_ArmorWithShield_ACIncludesShieldBonus(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID) // DEX 14 = +2
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}

	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "chain-mail" {
			return refdata.Armor{
				ID:         "chain-mail",
				Name:       "Chain Mail",
				ArmorType:  "heavy",
				AcBase:     16,
				AcDexBonus: sql.NullBool{Bool: false, Valid: true},
			}, nil
		}
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "chain-mail",
		Armor:     true,
	})

	require.NoError(t, err)
	// AC = 16 (chain mail) + 2 (shield) = 18
	assert.Equal(t, int32(18), result.NewAC)
}

// TDD Cycle 39: Unequip armor with shield keeps shield bonus
func TestEquip_UnequipArmor_WithShield_KeepsShieldBonus(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.Ac = 18 // chain mail 16 + shield 2
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}

	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "shield" {
			return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
		Armor:     true,
	})

	require.NoError(t, err)
	// AC = 10 + 2 (DEX) + 2 (shield) = 14
	assert.Equal(t, int32(14), result.NewAC)
}

// TDD Cycle 40: Unequip main hand when main hand is already empty
func TestEquip_UnequipMainHand_AlreadyEmpty(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	// main hand is already empty (default)
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "none",
	})

	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "unarmed strike")
}

// TDD Cycle 41: Unequip off-hand weapon in combat with free interact used and action available
func TestEquip_UnequipOffHandWeapon_InCombat_FreeInteractSpent(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shortsword", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{}, fmt.Errorf("not found")
	}

	turn := makeBasicTurn()
	turn.FreeInteractUsed = true

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Free object interaction already used")
}

// TDD Cycle 42: Doff shield in combat - update turn error
func TestEquip_DoffShield_InCombat_UpdateTurnError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", Name: "Shield", ArmorType: "shield", AcBase: 2}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 43: Unequip main hand in combat - update turn error
func TestEquip_UnequipMainHand_InCombat_UpdateTurnError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 44: Unequip off-hand weapon in combat - update turn error
func TestEquip_UnequipOffHandWeapon_InCombat_UpdateTurnError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.EquippedOffHand = sql.NullString{String: "shortsword", Valid: true}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		Turn:      &turn,
		ItemName:  "none",
		Offhand:   true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 45: Somatic component check — free hand available
func TestCheckSomaticComponent_FreeHand(t *testing.T) {
	char := refdata.Character{
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
	}
	spell := refdata.Spell{Name: "Shield", Components: []string{"V", "S"}}
	err := CheckSomaticComponent(char, spell, false)
	assert.NoError(t, err)
}

// TDD Cycle 41: Somatic component check — both hands occupied
func TestCheckSomaticComponent_BothHandsOccupied(t *testing.T) {
	char := refdata.Character{
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
		EquippedOffHand:  sql.NullString{String: "shield", Valid: true},
	}
	spell := refdata.Spell{Name: "Shield", Components: []string{"V", "S"}}
	err := CheckSomaticComponent(char, spell, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "somatic component requires a free hand")
}

// TDD Cycle 42: Somatic component check — War Caster bypasses
func TestCheckSomaticComponent_WarCaster(t *testing.T) {
	char := refdata.Character{
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
		EquippedOffHand:  sql.NullString{String: "shield", Valid: true},
	}
	spell := refdata.Spell{Name: "Shield", Components: []string{"V", "S"}}
	err := CheckSomaticComponent(char, spell, true)
	assert.NoError(t, err)
}

// TDD Cycle 43: Somatic component check — no somatic component
func TestCheckSomaticComponent_NoSomatic(t *testing.T) {
	char := refdata.Character{
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
		EquippedOffHand:  sql.NullString{String: "shield", Valid: true},
	}
	spell := refdata.Spell{Name: "Healing Word", Components: []string{"V"}}
	err := CheckSomaticComponent(char, spell, false)
	assert.NoError(t, err)
}
