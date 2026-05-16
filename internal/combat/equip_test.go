package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
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

// === Phase 75b: AC Recalculation & Enforcement ===

// TDD Cycle 75b-1: RecalculateAC with Barbarian unarmored defense formula (no armor)
func TestRecalculateAC_BarbarianUnarmoredDefense(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":16,"int":10,"wis":13,"cha":8}`),
		AcFormula:     sql.NullString{String: "10 + DEX + CON", Valid: true},
	}

	ac := RecalculateAC(char, nil, false)
	// Formula: 10 + DEX(+2) + CON(+3) = 15, base: 10 + DEX(+2) = 12, take max = 15
	assert.Equal(t, int32(15), ac)
}

// TDD Cycle 75b-2: RecalculateAC with Monk unarmored defense formula
func TestRecalculateAC_MonkUnarmoredDefense(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":16,"cha":8}`),
		AcFormula:     sql.NullString{String: "10 + DEX + WIS", Valid: true},
	}

	ac := RecalculateAC(char, nil, false)
	// Formula: 10 + DEX(+4) + WIS(+3) = 17
	assert.Equal(t, int32(17), ac)
}

// TDD Cycle 75b-3: RecalculateAC with armor ignores formula
func TestRecalculateAC_ArmorIgnoresFormula(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":16,"int":10,"wis":13,"cha":8}`),
		AcFormula:     sql.NullString{String: "10 + DEX + CON", Valid: true},
	}
	armor := &refdata.Armor{
		ID:         "chain-mail",
		AcBase:     16,
		AcDexBonus: sql.NullBool{Bool: false, Valid: true},
		ArmorType:  "heavy",
	}

	ac := RecalculateAC(char, armor, false)
	assert.Equal(t, int32(16), ac)
}

// TDD Cycle 75b-4: RecalculateAC with shield — monk WIS formula skips shield bonus
func TestRecalculateAC_WithShield(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":16,"cha":8}`),
		AcFormula:     sql.NullString{String: "10 + DEX + WIS", Valid: true},
	}

	ac := RecalculateAC(char, nil, true)
	// Formula: 10 + 4 + 3 = 17, shield skipped for monk (WIS formula)
	assert.Equal(t, int32(17), ac)
}

// TDD Cycle 75b-5: RecalculateAC medium armor caps DEX at +2
func TestRecalculateAC_MediumArmorDexCap(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":13,"cha":8}`),
	}
	armor := &refdata.Armor{
		ID:         "half-plate",
		AcBase:     15,
		AcDexBonus: sql.NullBool{Bool: true, Valid: true},
		AcDexMax:   sql.NullInt32{Int32: 2, Valid: true},
		ArmorType:  "medium",
	}

	ac := RecalculateAC(char, armor, false)
	// 15 + min(+4, 2) = 17
	assert.Equal(t, int32(17), ac)
}

// TDD Cycle 75b-6: RecalculateAC light armor full DEX
func TestRecalculateAC_LightArmorFullDex(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":13,"cha":8}`),
	}
	armor := &refdata.Armor{
		ID:         "leather",
		AcBase:     11,
		AcDexBonus: sql.NullBool{Bool: true, Valid: true},
		ArmorType:  "light",
	}

	ac := RecalculateAC(char, armor, false)
	// 11 + 4 = 15
	assert.Equal(t, int32(15), ac)
}

// TDD Cycle 75b-7: RecalculateAC no armor, no formula = base 10 + DEX
func TestRecalculateAC_BaseAC(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
	}

	ac := RecalculateAC(char, nil, false)
	// 10 + 2 = 12
	assert.Equal(t, int32(12), ac)
}

// TDD Cycle 75b-8: RecalculateAC Lizardfolk natural armor (13 + DEX)
func TestRecalculateAC_NaturalArmor(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
		AcFormula:     sql.NullString{String: "13 + DEX", Valid: true},
	}

	ac := RecalculateAC(char, nil, false)
	// Formula: 13 + 2 = 15, base: 10 + 2 = 12, take max = 15
	assert.Equal(t, int32(15), ac)
}

// TDD Cycle 75b-8b: RecalculateAC formula with all ability types
func TestRecalculateAC_FormulaAllAbilities(t *testing.T) {
	tests := []struct {
		name    string
		formula string
		scores  string
		want    int32
	}{
		{
			name:    "STR formula",
			formula: "10 + STR",
			scores:  `{"str":16,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`,
			want:    13, // 10 + 3
		},
		{
			name:    "INT formula",
			formula: "10 + INT",
			scores:  `{"str":10,"dex":10,"con":10,"int":18,"wis":10,"cha":10}`,
			want:    14, // 10 + 4
		},
		{
			name:    "CHA formula",
			formula: "10 + CHA",
			scores:  `{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":20}`,
			want:    15, // 10 + 5
		},
		{
			name:    "lowercase formula",
			formula: "10 + dex + wis",
			scores:  `{"str":10,"dex":14,"con":10,"int":10,"wis":16,"cha":10}`,
			want:    15, // 10 + 2 + 3
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			char := refdata.Character{
				AbilityScores: json.RawMessage(tt.scores),
				AcFormula:     sql.NullString{String: tt.formula, Valid: true},
			}
			ac := RecalculateAC(char, nil, false)
			assert.Equal(t, tt.want, ac)
		})
	}
}

// TDD Cycle 75b-9: CheckHeavyArmorPenalty — STR below requirement

// F-07: Defense fighting style adds +1 AC when wearing armor.
func TestRecalculateAC_F07_DefenseFightingStyle(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":12,"int":10,"wis":10,"cha":8}`),
		Features:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`[{"name":"Defense","mechanical_effect":"defense"}]`), Valid: true},
	}
	armor := &refdata.Armor{
		ID:         "chain-mail",
		AcBase:     16,
		AcDexBonus: sql.NullBool{Bool: false, Valid: true},
		ArmorType:  "heavy",
	}

	ac := RecalculateAC(char, armor, false)
	// Chain mail AC 16 + Defense +1 = 17
	assert.Equal(t, int32(17), ac)
}

func TestRecalculateAC_F07_DefenseNoArmorNoBonus(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":12,"int":10,"wis":10,"cha":8}`),
		Features:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`[{"name":"Defense","mechanical_effect":"defense"}]`), Valid: true},
	}

	ac := RecalculateAC(char, nil, false)
	// No armor: 10 + DEX(+2) = 12, Defense does NOT apply without armor
	assert.Equal(t, int32(12), ac)
}

func TestCheckHeavyArmorPenalty_BelowReq(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":13,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
	}
	armor := refdata.Armor{
		ID:          "plate",
		ArmorType:   "heavy",
		StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
	}

	penalty := CheckHeavyArmorPenalty(char, armor)
	assert.Equal(t, int32(10), penalty)
}

// TDD Cycle 75b-10: CheckHeavyArmorPenalty — STR meets requirement
func TestCheckHeavyArmorPenalty_MeetsReq(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":15,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
	}
	armor := refdata.Armor{
		ID:          "plate",
		ArmorType:   "heavy",
		StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
	}

	penalty := CheckHeavyArmorPenalty(char, armor)
	assert.Equal(t, int32(0), penalty)
}

// TDD Cycle 75b-11: CheckHeavyArmorPenalty — no STR requirement on armor
func TestCheckHeavyArmorPenalty_NoReq(t *testing.T) {
	char := refdata.Character{
		AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
	}
	armor := refdata.Armor{
		ID:        "chain-mail",
		ArmorType: "heavy",
	}

	penalty := CheckHeavyArmorPenalty(char, armor)
	assert.Equal(t, int32(0), penalty)
}

// TDD Cycle 75b-12: Equipping heavy armor with low STR reports speed penalty in EquipResult
func TestEquip_HeavyArmor_SpeedPenalty(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	char.AbilityScores = json.RawMessage(`{"str":13,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`)
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "plate" {
			return refdata.Armor{
				ID:          "plate",
				Name:        "Plate",
				ArmorType:   "heavy",
				AcBase:      18,
				AcDexBonus:  sql.NullBool{Bool: false, Valid: true},
				StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
			}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "plate",
		Armor:     true,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(18), result.NewAC)
	assert.Equal(t, int32(10), result.SpeedPenalty)
	assert.Contains(t, result.CombatLog, "speed reduced by 10ft")
}

// TDD Cycle 75b-13: Equipping heavy armor with sufficient STR — no penalty
func TestEquip_HeavyArmor_NoSpeedPenalty(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID) // STR 16
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		if id == "plate" {
			return refdata.Armor{
				ID:          "plate",
				Name:        "Plate",
				ArmorType:   "heavy",
				AcBase:      18,
				AcDexBonus:  sql.NullBool{Bool: false, Valid: true},
				StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
			}, nil
		}
		return refdata.Armor{}, fmt.Errorf("not found")
	}
	setupEquipMock(ms, char)

	svc := NewService(ms)
	result, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "plate",
		Armor:     true,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(18), result.NewAC)
	assert.Equal(t, int32(0), result.SpeedPenalty)
}

// TDD Cycle 75b-14: Doffing armor with ac_formula recalculates using formula
func TestEquip_DoffArmor_WithACFormula(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	// Barbarian: STR 16, DEX 14 (+2), CON 16 (+3)
	char := makeEquipChar(charID)
	char.AbilityScores = json.RawMessage(`{"str":16,"dex":14,"con":16,"int":10,"wis":13,"cha":8}`)
	char.AcFormula = sql.NullString{String: "10 + DEX + CON", Valid: true}
	char.Ac = 16 // chain mail
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}

	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
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
	// Unarmored defense: 10 + DEX(+2) + CON(+3) = 15
	assert.Equal(t, int32(15), result.NewAC)
	assert.True(t, result.ACChanged)
}

// TDD Cycle 75b-15: Equipping armor when barbarian has higher unarmored defense
func TestEquip_ArmorLowerThanFormula(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	// Barbarian: DEX 18 (+4), CON 18 (+4) => unarmored = 10+4+4=18
	char := makeEquipChar(charID)
	char.AbilityScores = json.RawMessage(`{"str":16,"dex":18,"con":18,"int":10,"wis":13,"cha":8}`)
	char.AcFormula = sql.NullString{String: "10 + DEX + CON", Valid: true}
	char.Ac = 18 // unarmored

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
	// When armor is equipped, formula is ignored. AC = 16 (armor AC)
	assert.Equal(t, int32(16), result.NewAC)
}

// TDD Cycle 75b-16: Shield equip uses RecalculateAC with formula
func TestEquip_Shield_WithACFormula(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	// Monk: DEX 18 (+4), WIS 16 (+3) => unarmored = 10+4+3=17
	char := makeEquipChar(charID)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":16,"cha":8}`)
	char.AcFormula = sql.NullString{String: "10 + DEX + WIS", Valid: true}
	char.Ac = 17 // unarmored monk

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
	// Monk WIS formula: shield bonus skipped. AC stays 17.
	assert.Equal(t, int32(17), result.NewAC)
}

// TDD Cycle 75b-17: Shield unequip uses RecalculateAC with formula
func TestEquip_DoffShield_WithACFormula(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	// Monk with shield: AC = 17 (shield bonus skipped for WIS formula)
	char := makeEquipChar(charID)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":16,"cha":8}`)
	char.AcFormula = sql.NullString{String: "10 + DEX + WIS", Valid: true}
	char.Ac = 17
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
	// 10 + DEX(+4) + WIS(+3) = 17 (no shield)
	assert.Equal(t, int32(17), result.NewAC)
}
