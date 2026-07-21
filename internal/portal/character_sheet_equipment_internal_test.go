package portal

import (
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
)

func TestExtractWeaponMasteries(t *testing.T) {
	got := extractWeaponMasteries(pqtype.NullRawMessage{
		Valid:      true,
		RawMessage: []byte(`{"weapon_masteries":["longsword","shortbow"]}`),
	})
	assert.Equal(t, []string{"longsword", "shortbow"}, got)

	assert.Nil(t, extractWeaponMasteries(pqtype.NullRawMessage{Valid: false}))
	assert.Nil(t, extractWeaponMasteries(pqtype.NullRawMessage{Valid: true, RawMessage: []byte(`{}`)}))
	assert.Nil(t, extractWeaponMasteries(pqtype.NullRawMessage{Valid: true, RawMessage: []byte(`not json`)}))
}

func TestResolveWeaponMasteries(t *testing.T) {
	weapons := map[string]*WeaponStats{
		"longsword": {Mastery: "Sap"},
		"shortbow":  {Mastery: "Vex"},
		"club":      {Mastery: ""}, // weapon without a mastery property → skipped
		// Homebrew weapon: absent from the static catalog but carries a DB Name.
		"hb_silent01": {Name: "Silent Blade", Mastery: "Vex"},
	}
	catalog := map[string]refdata.ItemCatalogEntry{
		"longsword": {ID: "longsword", Name: "Longsword"},
		"shortbow":  {ID: "shortbow", Name: "Shortbow"},
	}

	got := resolveWeaponMasteries([]string{"longsword", "shortbow", "club", "unknown"}, weapons, catalog)
	assert.Equal(t, []WeaponMasteryDisplay{
		{Weapon: "Longsword", Mastery: "Sap"},
		{Weapon: "Shortbow", Mastery: "Vex"},
	}, got)

	// id absent from the catalog falls back to the raw id as the display name.
	got = resolveWeaponMasteries([]string{"longsword"}, weapons, map[string]refdata.ItemCatalogEntry{})
	assert.Equal(t, []WeaponMasteryDisplay{{Weapon: "longsword", Mastery: "Sap"}}, got)

	// Homebrew weapon absent from the catalog uses the DB weapon Name, not the raw id.
	got = resolveWeaponMasteries([]string{"hb_silent01"}, weapons, catalog)
	assert.Equal(t, []WeaponMasteryDisplay{{Weapon: "Silent Blade", Mastery: "Vex"}}, got)

	assert.Nil(t, resolveWeaponMasteries(nil, weapons, catalog))
	assert.Nil(t, resolveWeaponMasteries([]string{"club", "unknown"}, weapons, catalog)) // all skipped → nil
}

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
	// Plain gear with only flavor text still gets an expandable detail body.
	assert.True(t, InventoryDisplayItem{InventoryItem: character.InventoryItem{Name: "Sealed Letter", Description: "A wax-sealed note from the guild."}}.HasDetail())
}
