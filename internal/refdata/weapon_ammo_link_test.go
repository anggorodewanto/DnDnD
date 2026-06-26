package refdata

import "testing"

// TestWeaponSeeds_AmmunitionWeaponsLinkAmmoItem is the drift guard for
// ISSUE-017 phase 2: every seeded weapon with the "ammunition" property must
// carry an ammunition_id FK that resolves to an ammunition-category row in the
// canonical item catalog. A new ammo weapon added without the link (or a
// renamed ammo id) fails CI here instead of silently falling back to the
// combat name heuristic.
func TestWeaponSeeds_AmmunitionWeaponsLinkAmmoItem(t *testing.T) {
	catalog := ItemCatalogByID()
	var linked int
	for _, w := range weaponSeeds() {
		hasAmmoProp := false
		for _, p := range w.Properties {
			if p == "ammunition" {
				hasAmmoProp = true
				break
			}
		}
		if !hasAmmoProp {
			if w.AmmunitionID.Valid {
				t.Errorf("weapon %s has no ammunition property but sets ammunition_id=%q", w.ID, w.AmmunitionID.String)
			}
			continue
		}
		if !w.AmmunitionID.Valid || w.AmmunitionID.String == "" {
			t.Errorf("ammunition weapon %s has no ammunition_id link", w.ID)
			continue
		}
		entry, ok := catalog[w.AmmunitionID.String]
		if !ok {
			t.Errorf("weapon %s links ammunition_id=%q with no catalog row", w.ID, w.AmmunitionID.String)
			continue
		}
		if entry.Category != "ammunition" {
			t.Errorf("weapon %s links %q which is category %q, want ammunition", w.ID, w.AmmunitionID.String, entry.Category)
		}
		linked++
	}
	if linked == 0 {
		t.Fatal("expected at least one ammunition weapon to link an ammo item")
	}
}
