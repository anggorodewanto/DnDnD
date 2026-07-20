package portal

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

// seedShapedSubclasses mirrors the JSONB shape of the classes.subclasses column
// as written by seeding (refdata/seed_classes.go): subclass-slug → {"name": …,
// "features_by_level": {level-string → [feature, …]}}. Two subclasses are
// present so tests can prove only the chosen one is granted.
const seedShapedSubclasses = `{
  "berserker": {
    "name": "Path of the Berserker",
    "features_by_level": {
      "3": [{"name": "Frenzy", "description": "Extra damage on your first hit.", "mechanical_effect": "frenzy"}],
      "6": [{"name": "Mindless Rage", "description": "You can't be charmed or frightened while raging.", "mechanical_effect": "immune_charmed_frightened_while_raging"}]
    }
  },
  "wild-heart": {
    "name": "Path of the Wild Heart",
    "features_by_level": {
      "3": [{"name": "Animal Speaker", "description": "You can cast Beast Sense.", "mechanical_effect": "animal_speaker"}]
    }
  }
}`

// barbarianClassFeatures is the class-level (non-subclass) half of the fixture.
func barbarianClassFeatures() map[string]map[string][]character.Feature {
	return map[string]map[string][]character.Feature{
		"barbarian": {
			"1": {{Name: "Rage", Description: "Enter a rage."}},
			"3": {{Name: "Primal Path", Description: "Choose a path."}},
		},
		"rogue": {
			"1": {{Name: "Sneak Attack", Description: "Extra d6."}},
		},
	}
}

// subclassFixture parses the seed-shaped JSON into the provider's map shape,
// exercising the real parser rather than a hand-built map.
func subclassFixture(t *testing.T) map[string]map[string]map[string][]character.Feature {
	t.Helper()
	parsed := parseSubclassFeatures([]byte(seedShapedSubclasses))
	if parsed == nil {
		t.Fatal("parseSubclassFeatures returned nil for seed-shaped JSON")
	}
	return map[string]map[string]map[string][]character.Feature{
		"barbarian": parsed,
		"rogue":     parseSubclassFeatures([]byte(`{"thief": {"name": "Thief", "features_by_level": {"3": [{"name": "Fast Hands"}]}}}`)),
	}
}

// featureNames flattens a feature slice to names for set assertions.
func featureNames(feats []character.Feature) map[string]bool {
	names := make(map[string]bool, len(feats))
	for _, f := range feats {
		names[f.Name] = true
	}
	return names
}

func TestParseSubclassFeatures_SeedShape(t *testing.T) {
	got := parseSubclassFeatures([]byte(seedShapedSubclasses))
	if got == nil {
		t.Fatal("expected parsed subclasses, got nil")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 subclasses, got %d", len(got))
	}

	frenzy := got["berserker"]["3"]
	if len(frenzy) != 1 {
		t.Fatalf("berserker level 3: expected 1 feature, got %d", len(frenzy))
	}
	if frenzy[0].Name != "Frenzy" {
		t.Errorf("expected Frenzy, got %s", frenzy[0].Name)
	}
	// The mechanical_effect slug is what the combat riders gate on, so it must
	// survive the decode — a name-only parse would silently disable them.
	if frenzy[0].MechanicalEffect != "frenzy" {
		t.Errorf("MechanicalEffect = %q, want %q", frenzy[0].MechanicalEffect, "frenzy")
	}
	if len(got["berserker"]["6"]) != 1 {
		t.Errorf("expected Mindless Rage at berserker level 6")
	}
}

func TestParseSubclassFeatures_EmptyAndMalformed(t *testing.T) {
	if got := parseSubclassFeatures(nil); got != nil {
		t.Errorf("nil input should parse to nil, got %v", got)
	}
	if got := parseSubclassFeatures([]byte(`not json`)); got != nil {
		t.Errorf("malformed input should degrade to nil, got %v", got)
	}
	// A subclass with no features_by_level contributes nothing rather than an
	// empty inner map, so callers can't mistake it for a populated subclass.
	if got := parseSubclassFeatures([]byte(`{"berserker": {"name": "Berserker"}}`)); len(got) != 0 {
		t.Errorf("subclass without features_by_level should be skipped, got %v", got)
	}
}

func TestCollectFeatures_GrantsChosenSubclassAtOrBelowLevel(t *testing.T) {
	classes := []character.ClassEntry{{Class: "barbarian", Subclass: "berserker", Level: 3}}

	names := featureNames(CollectFeatures(classes, barbarianClassFeatures(), subclassFixture(t), nil))

	if !names["Rage"] {
		t.Error("expected class feature Rage")
	}
	if !names["Frenzy"] {
		t.Error("expected subclass feature Frenzy at level 3")
	}
	if names["Mindless Rage"] {
		t.Error("Mindless Rage is a level-6 feature and must not be granted at level 3")
	}
	if names["Animal Speaker"] {
		t.Error("Wild Heart features must not leak into a Berserker character")
	}
}

func TestCollectFeatures_GrantsHigherSubclassFeatureAtHigherLevel(t *testing.T) {
	classes := []character.ClassEntry{{Class: "barbarian", Subclass: "berserker", Level: 6}}

	names := featureNames(CollectFeatures(classes, barbarianClassFeatures(), subclassFixture(t), nil))

	if !names["Frenzy"] || !names["Mindless Rage"] {
		t.Errorf("level 6 Berserker should have both Frenzy and Mindless Rage, got %v", names)
	}
}

func TestCollectFeatures_NoSubclassChosen(t *testing.T) {
	classes := []character.ClassEntry{{Class: "barbarian", Level: 2}}

	feats := CollectFeatures(classes, barbarianClassFeatures(), subclassFixture(t), nil)
	names := featureNames(feats)

	if !names["Rage"] {
		t.Error("expected class feature Rage even with no subclass")
	}
	if names["Frenzy"] || names["Mindless Rage"] || names["Animal Speaker"] {
		t.Errorf("no subclass chosen must grant no subclass features, got %v", names)
	}
}

func TestCollectFeatures_MulticlassScopesSubclassLevelsPerClass(t *testing.T) {
	// Barbarian 3 / Rogue 6: total level 9, but Mindless Rage keys off the
	// barbarian level (3), so it must not appear.
	classes := []character.ClassEntry{
		{Class: "barbarian", Subclass: "berserker", Level: 3},
		{Class: "rogue", Subclass: "thief", Level: 6},
	}

	names := featureNames(CollectFeatures(classes, barbarianClassFeatures(), subclassFixture(t), nil))

	if !names["Frenzy"] {
		t.Error("expected Frenzy from barbarian 3")
	}
	if names["Mindless Rage"] {
		t.Error("Mindless Rage keys off barbarian level 3, not the level-9 total")
	}
	if !names["Fast Hands"] {
		t.Error("expected Fast Hands from rogue 6 (thief level 3 feature)")
	}
}
