package combat

import (
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ammoWeapon(id, name, ammoID string) refdata.Weapon {
	return refdata.Weapon{
		ID:           id,
		Name:         name,
		Properties:   []string{"ammunition"},
		AmmunitionID: sql.NullString{String: ammoID, Valid: ammoID != ""},
	}
}

// TestDeductAmmunition_MatchesByAmmunitionID proves the FK item-id path
// (ISSUE-017 phase 2): a stack identified only by its canonical item_id is
// spent even when its display name shares no keyword with the conventional
// ammo name — the precise link no longer depends on a name heuristic.
func TestDeductAmmunition_MatchesByAmmunitionID(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "crossbow-bolt", Name: "Quarrels", Quantity: 12, Type: "gear"},
	}
	got, err := DeductAmmunition(items, "Bolts", "crossbow-bolt")
	require.NoError(t, err)
	assert.Equal(t, 11, got[0].Quantity)
}

// TestDeductAmmunition_IDFallsBackToKeyword confirms that when the FK id does
// not match any stack, the legacy keyword scan still finds ammo (back-compat
// for pre-FK inventory rows).
func TestDeductAmmunition_IDFallsBackToKeyword(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "crossbow-bolt", Name: "crossbow-bolt", Quantity: 5, Type: "gear"},
	}
	// ammoID points at a different id; keyword "bolt" must still match.
	got, err := DeductAmmunition(items, "Bolts", "some-other-ammo")
	require.NoError(t, err)
	assert.Equal(t, 4, got[0].Quantity)
}

func TestGetAmmunitionName_FromFK(t *testing.T) {
	cases := map[string]struct {
		weapon refdata.Weapon
		want   string
	}{
		"crossbow bolt": {ammoWeapon("light-crossbow", "Light crossbow", "crossbow-bolt"), "Bolts"},
		"arrow":         {ammoWeapon("longbow", "Longbow", "arrow"), "Arrows"},
		"sling bullet":  {ammoWeapon("sling", "Sling", "sling-bullet"), "Bullets"},
		"blowgun":       {ammoWeapon("blowgun", "Blowgun", "blowgun-needle"), "Needles"},
		// FK absent -> legacy substring fallback.
		"fallback crossbow": {ammoWeapon("homebrew-crossbow", "Repeating crossbow", ""), "Bolts"},
		"fallback other":    {ammoWeapon("homebrew-bow", "War bow", ""), "Arrows"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, GetAmmunitionName(tc.weapon))
		})
	}
}

func TestRecoverAmmunition_MatchesByAmmunitionID(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "arrow", Name: "Bodkins", Quantity: 4, Type: "gear"},
	}
	// Spent 6 (recover 3) — matched by FK id, not by the "Arrows" keyword.
	got := RecoverAmmunition(items, "Arrows", "arrow", 6)
	assert.Equal(t, 7, got[0].Quantity)
}
