package open5e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_ReturnsCopy(t *testing.T) {
	c1 := Catalog()
	require.NotEmpty(t, c1)
	c1[0].Title = "mutated"
	c2 := Catalog()
	assert.NotEqual(t, "mutated", c2[0].Title, "Catalog must return a fresh copy each call")
}

func TestCatalog_ContainsCanonicalSRDAndKoboldEntries(t *testing.T) {
	slugs := CatalogSlugs()
	// Spot-check the canonical entries used elsewhere in tests and the spec.
	assert.Contains(t, slugs, "wotc-srd")
	assert.Contains(t, slugs, "tome-of-beasts")
	assert.Contains(t, slugs, "deep-magic")
	assert.Contains(t, slugs, "creature-codex")
}

func TestIsKnownSource(t *testing.T) {
	assert.True(t, IsKnownSource("tome-of-beasts"))
	assert.True(t, IsKnownSource("wotc-srd"))
	assert.False(t, IsKnownSource(""))
	assert.False(t, IsKnownSource("definitely-not-a-real-source"))
}
