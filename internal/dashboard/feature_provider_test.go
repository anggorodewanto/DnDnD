package dashboard

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestRefDataFeatureProvider_ImplementsInterface(t *testing.T) {
	// Verify the type satisfies the FeatureProvider interface at compile time.
	var _ FeatureProvider = (*RefDataFeatureProvider)(nil)
}

func TestRefDataFeatureProvider_EmptyMaps(t *testing.T) {
	fp := &RefDataFeatureProvider{
		classFeatures:    make(map[string]map[string][]character.Feature),
		subclassFeatures: make(map[string]map[string]map[string][]character.Feature),
		races:            make(map[string][]character.Feature),
	}
	if cf := fp.ClassFeatures(); cf == nil {
		t.Error("ClassFeatures() should not be nil")
	}
	if sf := fp.SubclassFeatures(); sf == nil {
		t.Error("SubclassFeatures() should not be nil")
	}
	if rt := fp.RacialTraits("elf"); rt != nil {
		t.Errorf("RacialTraits for unknown race should be nil, got %v", rt)
	}
}

func TestCollectFeatures_CaseInsensitiveLookup(t *testing.T) {
	// Map keyed by slug (lowercase) as the real provider builds it
	classFeatures := map[string]map[string][]character.Feature{
		"barbarian": {
			"1": {{Name: "Rage", Source: "Barbarian", Level: 1, Description: "Enter rage"}},
		},
	}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"barbarian": {
			"berserker": {
				"3": {{Name: "Frenzy", Source: "Barbarian (Berserker)", Level: 3, Description: "Bonus action attack"}},
			},
		},
	}

	// Form passes display name (capitalized)
	classes := []character.ClassEntry{
		{Class: "Barbarian", Subclass: "Berserker", Level: 3},
	}

	features := CollectFeatures(classes, classFeatures, subclassFeatures, nil)
	if len(features) == 0 {
		t.Fatal("CollectFeatures with capitalized class name should find features from lowercase-keyed map")
	}
	if features[0].Name != "Rage" {
		t.Errorf("expected first feature to be Rage, got %s", features[0].Name)
	}
}

func TestRacialTraits_CaseInsensitiveLookup(t *testing.T) {
	fp := &RefDataFeatureProvider{
		classFeatures:    make(map[string]map[string][]character.Feature),
		subclassFeatures: make(map[string]map[string]map[string][]character.Feature),
		races: map[string][]character.Feature{
			"elf": {{Name: "Darkvision", Source: "Elf", Level: 0, Description: "See in dark"}},
		},
	}

	// Form passes "Elf" but map is keyed by "elf"
	traits := fp.RacialTraits("Elf")
	if len(traits) == 0 {
		t.Fatal("RacialTraits with capitalized race name should find traits from lowercase-keyed map")
	}
	if traits[0].Name != "Darkvision" {
		t.Errorf("expected Darkvision, got %s", traits[0].Name)
	}
}
