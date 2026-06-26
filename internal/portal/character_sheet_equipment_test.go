package portal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refWeapons / refArmor are the minimal reference rows the equipment-enrichment
// tests join against.
func refWeapons() []refdata.Weapon {
	return []refdata.Weapon{
		{
			ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing",
			Properties:      []string{"versatile"},
			VersatileDamage: sql.NullString{String: "1d10", Valid: true},
			WeaponType:      "martial_melee", Mastery: "sap",
		},
	}
}

func refArmor() []refdata.Armor {
	return []refdata.Armor{
		{
			ID: "chain-mail", Name: "Chain mail", AcBase: 16,
			AcDexBonus:    sql.NullBool{Bool: false, Valid: true},
			StrengthReq:   sql.NullInt32{Int32: 13, Valid: true},
			StealthDisadv: sql.NullBool{Bool: true, Valid: true},
			ArmorType:     "heavy",
		},
	}
}

func TestGetCharacterForSheet_EnrichesInventoryAndEquipment(t *testing.T) {
	charID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	inventory := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Equipped: true, Type: "weapon"},
		{ItemID: "chain-mail", Name: "Chain mail", Quantity: 1, Equipped: true, Type: "armor"},
		{ItemID: "rope", Name: "Rope", Quantity: 1, Type: "gear"},
		// Legacy row: name stored as the raw id; sheet should show catalog name.
		{ItemID: "crossbow-bolt", Name: "crossbow-bolt", Quantity: 1, Type: "gear"},
	}
	inventoryJSON, _ := json.Marshal(inventory)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			Name:             "Tank",
			Level:            1,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			EquippedArmor:    sql.NullString{String: "chain-mail", Valid: true},
			Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
		},
		weapons: refWeapons(),
		armor:   refArmor(),
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())
	require.NoError(t, err)

	// Inventory weapon stats joined by item id.
	require.Len(t, data.Inventory, 4)

	// Legacy raw-id name resolves to the catalog display name.
	assert.Equal(t, "Crossbow Bolts", data.Inventory[3].Name)
	require.NotNil(t, data.Inventory[0].Weapon)
	assert.Equal(t, "1d8 slashing", data.Inventory[0].Weapon.Damage)
	assert.Equal(t, "1d10", data.Inventory[0].Weapon.Versatile)
	assert.True(t, data.Inventory[0].HasDetail())

	// Inventory armor stats joined by item id.
	require.NotNil(t, data.Inventory[1].Armor)
	assert.Equal(t, "16", data.Inventory[1].Armor.AC)

	// Plain gear has no stat block and renders as a flat row.
	assert.Nil(t, data.Inventory[2].Weapon)
	assert.Nil(t, data.Inventory[2].Armor)
	assert.False(t, data.Inventory[2].HasDetail())

	// Equipped slots resolve id -> catalog name + stat block.
	assert.Equal(t, "Longsword", data.EquippedMainHand.Name)
	require.NotNil(t, data.EquippedMainHand.Weapon)
	assert.Equal(t, "1d8 slashing", data.EquippedMainHand.Weapon.Damage)

	assert.Equal(t, "Chain mail", data.EquippedArmor.Name)
	require.NotNil(t, data.EquippedArmor.Armor)
	assert.Equal(t, "16", data.EquippedArmor.Armor.AC)
	assert.True(t, data.EquippedArmor.Armor.StealthDisadv)
}

func TestGetCharacterForSheet_EquipmentLookupErrorDegradesGracefully(t *testing.T) {
	charID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})
	inventory := []character.InventoryItem{{ItemID: "longsword", Name: "Longsword", Quantity: 1}}
	inventoryJSON, _ := json.Marshal(inventory)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			Name:             "Tank",
			Level:            1,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
		},
		weaponsErr: sql.ErrConnDone,
		armorErr:   sql.ErrConnDone,
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	// Sheet still loads; equipment is name-only (id resolved via catalog).
	require.NoError(t, err)
	require.Len(t, data.Inventory, 1)
	assert.Nil(t, data.Inventory[0].Weapon)
	assert.Equal(t, "Longsword", data.EquippedMainHand.Name)
	assert.Nil(t, data.EquippedMainHand.Weapon)
}

func TestServeCharacterSheet_RendersItemAccordionAndEquipmentStats(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			ID:               "char-1",
			Name:             "Tank",
			Race:             "Human",
			Level:            1,
			ProficiencyBonus: 2,
			Classes:          []character.ClassEntry{{Class: "Fighter", Level: 1}},
			AbilityScores:    character.AbilityScores{STR: 16, DEX: 12, CON: 14, INT: 8, WIS: 10, CHA: 10},
			EquippedMainHand: portal.EquippedSlot{
				ItemID: "longsword", Name: "Longsword",
				Weapon: &portal.WeaponStats{Damage: "1d8 slashing", Versatile: "1d10", WeaponType: "Martial Melee", Mastery: "Sap", Properties: []string{"Versatile"}},
			},
			EquippedArmor: portal.EquippedSlot{
				ItemID: "chain-mail", Name: "Chain mail",
				Armor: &portal.ArmorStats{AC: "16", ArmorType: "Heavy", StrengthReq: 13, StealthDisadv: true},
			},
			Inventory: []portal.InventoryDisplayItem{
				{
					InventoryItem: character.InventoryItem{ItemID: "longsword", Name: "Longsword", Quantity: 1, Equipped: true},
					Weapon:        &portal.WeaponStats{Damage: "1d8 slashing", Versatile: "1d10", WeaponType: "Martial Melee", Mastery: "Sap"},
				},
				{InventoryItem: character.InventoryItem{ItemID: "arrow", Name: "Arrows", Quantity: 20, Type: "ammunition"}},
			},
			AbilityModifiers: map[string]int{"STR": 3, "DEX": 1, "CON": 2, "INT": -1, "WIS": 0, "CHA": 0},
			ClassSummary:     "Fighter 1",
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Inventory item with stats becomes an expandable accordion (details/summary).
	assert.Contains(t, body, "item-summary")
	assert.Contains(t, body, "<details")
	// Weapon stats are rendered.
	assert.Contains(t, body, "1d8 slashing")
	assert.Contains(t, body, "versatile 1d10")
	assert.Contains(t, body, "Martial Melee")
	// Equipment section shows armor stats.
	assert.Contains(t, body, "16")
	assert.Contains(t, body, "Disadvantage")
	// Stackable item shows its quantity.
	assert.Contains(t, body, "x20")
	// Plain ammunition row is not an accordion summary line.
	assert.Contains(t, body, "Arrows")
}
