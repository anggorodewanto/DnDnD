package itempicker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/itempicker"
	"github.com/ab/dndnd/internal/refdata"
)

func TestCatalogAmmunition_SourcesFromCatalog(t *testing.T) {
	ammo := itempicker.CatalogAmmunition()

	byID := map[string]string{}
	for _, a := range ammo {
		byID[a.ID] = a.Name
	}

	// The canonical bolt id players already store resolves to its display name.
	assert.Equal(t, "Crossbow Bolts", byID["crossbow-bolt"])
	assert.Contains(t, byID, "arrow")
	assert.Contains(t, byID, "sling-bullet")
	assert.Contains(t, byID, "blowgun-needle")

	// Exactly the catalog's ammunition rows, no more.
	var want int
	for _, e := range refdata.ItemCatalog() {
		if e.Category == "ammunition" {
			want++
		}
	}
	assert.Len(t, ammo, want)
}

func TestMergedGear_IncludesStaticAndCatalog(t *testing.T) {
	gear := itempicker.MergedGear()

	byID := map[string]itempicker.GearItem{}
	for _, g := range gear {
		byID[g.ID] = g
	}

	// Static descriptive rows survive.
	require.Contains(t, byID, "backpack")
	assert.NotEmpty(t, byID["backpack"].Description)
	assert.Contains(t, byID, "torch")

	// Catalog-only gear (packs / foci / clothing) becomes pickable.
	assert.Equal(t, "Arcane Focus", byID["arcane-focus"].Name)
	assert.Contains(t, byID, "dungeoneers-pack")
	assert.Contains(t, byID, "common-clothes")
}

func TestMergedGear_DedupesByID_StaticWins(t *testing.T) {
	gear := itempicker.MergedGear()

	var crowbars []itempicker.GearItem
	for _, g := range gear {
		if g.ID == "crowbar" {
			crowbars = append(crowbars, g)
		}
	}

	// "crowbar" is in both static and catalog; it must appear once, keeping the
	// static row's description.
	require.Len(t, crowbars, 1)
	assert.NotEmpty(t, crowbars[0].Description)
}
