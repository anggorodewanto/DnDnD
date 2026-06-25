package portal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// builderBackgroundSlugs mirrors the BACKGROUNDS list in CharacterBuilder.svelte.
// The Go maps keyed by background slug (skills, equipment) must cover every one
// of these kebab-case slugs, or the background silently grants nothing.
var builderBackgroundSlugs = []string{
	"acolyte", "charlatan", "criminal", "entertainer", "folk-hero",
	"guild-artisan", "hermit", "noble", "outlander", "sage",
	"sailor", "soldier", "urchin",
}

// TestBackgroundEquipmentPack_AllBuilderBackgrounds guards the equipment map
// against slug drift (the space-vs-hyphen "guild artisan"/"folk hero" bug):
// every builder background must resolve to a non-empty pack.
func TestBackgroundEquipmentPack_AllBuilderBackgrounds(t *testing.T) {
	for _, slug := range builderBackgroundSlugs {
		t.Run(slug, func(t *testing.T) {
			assert.NotEmpty(t, BackgroundEquipmentPack(slug),
				"background %q must resolve to a starting-equipment pack", slug)
		})
	}
}

func TestBackgroundEquipmentPack_Unknown(t *testing.T) {
	assert.Nil(t, BackgroundEquipmentPack("not-a-background"))
}
