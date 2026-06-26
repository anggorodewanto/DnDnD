package refdata

import "testing"

func TestItemCatalog_NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range ItemCatalog() {
		if seen[e.ID] {
			t.Errorf("duplicate catalog id: %s", e.ID)
		}
		seen[e.ID] = true
	}
}

func TestItemCatalog_EntriesWellFormed(t *testing.T) {
	valid := map[string]bool{"weapon": true, "armor": true, "ammunition": true, "gear": true}
	for _, e := range ItemCatalog() {
		if e.ID == "" {
			t.Error("catalog entry has empty id")
		}
		if e.Name == "" {
			t.Errorf("catalog entry %s has empty name", e.ID)
		}
		if !valid[e.Category] {
			t.Errorf("catalog entry %s has invalid category %q", e.ID, e.Category)
		}
		if e.DefaultQuantity < 1 {
			t.Errorf("catalog entry %s has non-positive default_quantity %d", e.ID, e.DefaultQuantity)
		}
	}
}

func TestItemCatalog_CountMatchesEntries(t *testing.T) {
	if ItemCount != len(ItemCatalog()) {
		t.Fatalf("ItemCount=%d but ItemCatalog has %d entries", ItemCount, len(ItemCatalog()))
	}
}

func TestItemCatalog_DerivesWeaponAndArmorNames(t *testing.T) {
	byID := ItemCatalogByID()
	// Weapon/armor catalog rows must reuse the seed display names, not the slug.
	if got := byID["light-crossbow"]; got.Name != "Light crossbow" || got.Category != "weapon" {
		t.Errorf("light-crossbow = %+v; want name=\"Light crossbow\" category=weapon", got)
	}
	if got := byID["chain-mail"]; got.Name != "Chain mail" || got.Category != "armor" {
		t.Errorf("chain-mail = %+v; want name=\"Chain mail\" category=armor", got)
	}
}

func TestItemCatalog_CrossbowBoltAcceptanceShape(t *testing.T) {
	// ISSUE-017 acceptance: bolts resolve as typed, named, full-bundle
	// ammunition — not a bare "gear" slug.
	bolt, ok := ItemCatalogByID()["crossbow-bolt"]
	if !ok {
		t.Fatal("crossbow-bolt missing from catalog")
	}
	if bolt.Name != "Crossbow Bolts" || bolt.Category != "ammunition" || bolt.DefaultQuantity != 20 {
		t.Errorf("crossbow-bolt = %+v; want name=\"Crossbow Bolts\" category=ammunition qty=20", bolt)
	}
}
