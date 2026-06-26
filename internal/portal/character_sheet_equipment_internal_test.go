package portal

import (
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
)

func TestFormatArmorAC(t *testing.T) {
	tests := []struct {
		name  string
		armor refdata.Armor
		want  string
	}{
		{
			name:  "heavy fixed AC",
			armor: refdata.Armor{AcBase: 16, AcDexBonus: sql.NullBool{Bool: false, Valid: true}, ArmorType: "heavy"},
			want:  "16",
		},
		{
			name:  "light adds full dex",
			armor: refdata.Armor{AcBase: 11, AcDexBonus: sql.NullBool{Bool: true, Valid: true}, ArmorType: "light"},
			want:  "11 + DEX",
		},
		{
			name:  "medium caps dex",
			armor: refdata.Armor{AcBase: 14, AcDexBonus: sql.NullBool{Bool: true, Valid: true}, AcDexMax: sql.NullInt32{Int32: 2, Valid: true}, ArmorType: "medium"},
			want:  "14 + DEX (max 2)",
		},
		{
			name:  "shield is a bonus",
			armor: refdata.Armor{AcBase: 2, AcDexBonus: sql.NullBool{Bool: false, Valid: true}, ArmorType: "shield"},
			want:  "+2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatArmorAC(tt.armor))
		})
	}
}

func TestWeaponStatsFrom(t *testing.T) {
	w := refdata.Weapon{
		ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing",
		Properties:      []string{"versatile"},
		VersatileDamage: sql.NullString{String: "1d10", Valid: true},
		WeaponType:      "martial_melee", Mastery: "sap",
	}
	got := weaponStatsFrom(w)
	assert.Equal(t, "1d8 slashing", got.Damage)
	assert.Equal(t, "1d10", got.Versatile)
	assert.Equal(t, "Martial Melee", got.WeaponType)
	assert.Equal(t, "Sap", got.Mastery)
	assert.Equal(t, []string{"Versatile"}, got.Properties)
	assert.Empty(t, got.Range)
}

func TestWeaponStatsFrom_RangedShowsRange(t *testing.T) {
	w := refdata.Weapon{
		ID: "shortbow", Name: "Shortbow", Damage: "1d6", DamageType: "piercing",
		Properties:    []string{"ammunition", "two-handed"},
		RangeNormalFt: sql.NullInt32{Int32: 80, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 320, Valid: true},
		WeaponType:    "simple_ranged", Mastery: "vex",
	}
	got := weaponStatsFrom(w)
	assert.Equal(t, "80/320 ft", got.Range)
	assert.Equal(t, "Simple Ranged", got.WeaponType)
}

func TestArmorStatsFrom(t *testing.T) {
	a := refdata.Armor{
		ID: "chain-mail", Name: "Chain mail", AcBase: 16,
		AcDexBonus:    sql.NullBool{Bool: false, Valid: true},
		StrengthReq:   sql.NullInt32{Int32: 13, Valid: true},
		StealthDisadv: sql.NullBool{Bool: true, Valid: true},
		ArmorType:     "heavy",
	}
	got := armorStatsFrom(a)
	assert.Equal(t, "16", got.AC)
	assert.Equal(t, "Heavy", got.ArmorType)
	assert.Equal(t, 13, got.StrengthReq)
	assert.True(t, got.StealthDisadv)
}

func TestInventoryDisplayItem_HasDetail(t *testing.T) {
	assert.False(t, InventoryDisplayItem{InventoryItem: character.InventoryItem{Name: "Rope"}}.HasDetail())
	assert.True(t, InventoryDisplayItem{Weapon: &WeaponStats{}}.HasDetail())
	assert.True(t, InventoryDisplayItem{Armor: &ArmorStats{}}.HasDetail())
	assert.True(t, InventoryDisplayItem{InventoryItem: character.InventoryItem{IsMagic: true}}.HasDetail())
	assert.True(t, InventoryDisplayItem{InventoryItem: character.InventoryItem{MaxCharges: 3}}.HasDetail())
}
