package levelup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/google/uuid"
)

// barbarianSubclasses mirrors the classes.subclasses JSONB shape as loaded by
// the store adapter: it arrives as map[string]any, keyed by subclass slug.
func barbarianSubclasses(t *testing.T) map[string]any {
	t.Helper()
	const raw = `{
      "berserker": {
        "name": "Path of the Berserker",
        "features_by_level": {
          "3": [{"name": "Frenzy", "description": "Extra damage on your first hit.", "mechanical_effect": "frenzy"}],
          "6": [{"name": "Mindless Rage", "description": "Immune to charm and fear while raging.", "mechanical_effect": "immune_charmed_frightened_while_raging"}]
        }
      },
      "wild-heart": {
        "name": "Path of the Wild Heart",
        "features_by_level": {
          "3": [{"name": "Animal Speaker", "description": "Cast Beast Sense.", "mechanical_effect": "animal_speaker"}]
        }
      }
    }`
	var subs map[string]any
	if err := json.Unmarshal([]byte(raw), &subs); err != nil {
		t.Fatalf("unmarshaling subclasses fixture: %v", err)
	}
	return subs
}

// barbarianRef builds the class ref data a level-up reads, with class features
// only at levels 1-3 so tests can prove a subclass feature still lands at a
// level where the class itself grants nothing (barbarian 6).
func barbarianRef(t *testing.T) *ClassRefData {
	t.Helper()
	return &ClassRefData{
		HitDie:           "d12",
		AttacksPerAction: map[int]int{1: 1, 5: 2},
		SubclassLevel:    3,
		Subclasses:       barbarianSubclasses(t),
		FeaturesByLevel: map[string][]character.Feature{
			"1": {{Name: "Rage"}},
			"3": {{Name: "Primal Path"}},
		},
	}
}

// barbarianLevelUpFixture wires a barbarian at the given level/subclass into the mocks.
func barbarianLevelUpFixture(t *testing.T, level int, subclass string) (uuid.UUID, *mockCharacterStore, *Service) {
	t.Helper()
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	classStore.classes["barbarian"] = barbarianRef(t)

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Grug",
		DiscordUserID:    "user123",
		Level:            int32(level),
		HPMax:            30,
		HPCurrent:        30,
		ProficiencyBonus: 2,
		Classes:          mustJSON(t, []character.ClassEntry{{Class: "barbarian", Subclass: subclass, Level: level}}),
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	return charID, charStore, NewService(charStore, classStore, &mockNotifier{})
}

// storedFeatureNames reads back the persisted feature names.
func storedFeatureNames(t *testing.T, store *mockCharacterStore, id uuid.UUID) map[string]int {
	t.Helper()
	var feats []character.Feature
	if len(store.chars[id].Features) > 0 {
		if err := json.Unmarshal(store.chars[id].Features, &feats); err != nil {
			t.Fatalf("unmarshaling stored features: %v", err)
		}
	}
	counts := make(map[string]int, len(feats))
	for _, f := range feats {
		counts[f.Name]++
	}
	return counts
}

func TestSubclassFeaturesForLevel(t *testing.T) {
	subs := barbarianSubclasses(t)

	got := subclassFeaturesForLevel(subs, "berserker", 3)
	if len(got) != 1 || got[0].Name != "Frenzy" {
		t.Fatalf("expected Frenzy at berserker 3, got %v", got)
	}
	if got[0].MechanicalEffect != "frenzy" {
		t.Errorf("MechanicalEffect = %q, want %q", got[0].MechanicalEffect, "frenzy")
	}

	if got := subclassFeaturesForLevel(subs, "berserker", 4); len(got) != 0 {
		t.Errorf("berserker has no level-4 feature, got %v", got)
	}
	if got := subclassFeaturesForLevel(subs, "", 3); len(got) != 0 {
		t.Errorf("no subclass chosen should yield nothing, got %v", got)
	}
	if got := subclassFeaturesForLevel(subs, "unknown-path", 3); len(got) != 0 {
		t.Errorf("unknown subclass should yield nothing, got %v", got)
	}
	if got := subclassFeaturesForLevel(nil, "berserker", 3); len(got) != 0 {
		t.Errorf("nil subclasses should yield nothing, got %v", got)
	}
}

func TestService_ApplyLevelUp_GrantsSubclassFeature(t *testing.T) {
	charID, charStore, svc := barbarianLevelUpFixture(t, 2, "berserker")

	if _, err := svc.ApplyLevelUp(context.Background(), charID, "barbarian", 3); err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	names := storedFeatureNames(t, charStore, charID)
	if names["Frenzy"] != 1 {
		t.Errorf("expected Frenzy granted once at barbarian 3, got %d (%v)", names["Frenzy"], names)
	}
	if names["Primal Path"] != 1 {
		t.Errorf("expected the class feature Primal Path too, got %v", names)
	}
	if names["Animal Speaker"] != 0 {
		t.Error("a Berserker must not receive Wild Heart features")
	}
}

// The class grants no feature at barbarian 6, so this fails if subclass
// features are only appended alongside a non-empty class-feature list.
func TestService_ApplyLevelUp_GrantsSubclassFeatureWhenClassGrantsNone(t *testing.T) {
	charID, charStore, svc := barbarianLevelUpFixture(t, 5, "berserker")

	if _, err := svc.ApplyLevelUp(context.Background(), charID, "barbarian", 6); err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	if names := storedFeatureNames(t, charStore, charID); names["Mindless Rage"] != 1 {
		t.Errorf("expected Mindless Rage at barbarian 6, got %v", names)
	}
}

func TestService_ApplyLevelUp_SubclassFeatureNotDuplicatedOnRepeat(t *testing.T) {
	charID, charStore, svc := barbarianLevelUpFixture(t, 2, "berserker")

	for i := 0; i < 2; i++ {
		if _, err := svc.ApplyLevelUp(context.Background(), charID, "barbarian", 3); err != nil {
			t.Fatalf("ApplyLevelUp %d error: %v", i, err)
		}
	}

	if names := storedFeatureNames(t, charStore, charID); names["Frenzy"] != 1 {
		t.Errorf("Frenzy should be granted exactly once, got %d", names["Frenzy"])
	}
}

func TestService_ApplyLevelUp_NoSubclassGrantsNoSubclassFeatures(t *testing.T) {
	charID, charStore, svc := barbarianLevelUpFixture(t, 2, "")

	if _, err := svc.ApplyLevelUp(context.Background(), charID, "barbarian", 3); err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	names := storedFeatureNames(t, charStore, charID)
	if names["Frenzy"] != 0 || names["Animal Speaker"] != 0 {
		t.Errorf("a character without a subclass must get no subclass features, got %v", names)
	}
	if names["Primal Path"] != 1 {
		t.Errorf("class features still apply, got %v", names)
	}
}

// A multiclass level-up must key subclass features off the leveled class's own
// level, not the character's total level.
func TestService_ApplyLevelUp_MulticlassScopesSubclassLevel(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	classStore.classes["barbarian"] = barbarianRef(t)
	// buildRefMaps resolves every class on the character, so the second class
	// needs ref data even though it is not the one being leveled.
	classStore.classes["rogue"] = &ClassRefData{HitDie: "d8", AttacksPerAction: map[int]int{1: 1}, SubclassLevel: 3}

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Grug",
		DiscordUserID:    "user123",
		Level:            6,
		HPMax:            50,
		HPCurrent:        50,
		ProficiencyBonus: 3,
		Classes: mustJSON(t, []character.ClassEntry{
			{Class: "barbarian", Subclass: "berserker", Level: 2},
			{Class: "rogue", Level: 4},
		}),
		AbilityScores: mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}
	svc := NewService(charStore, classStore, &mockNotifier{})

	// Barbarian 2 -> 3 brings the character to total level 7, but only the
	// barbarian level 3 subclass feature may be granted.
	if _, err := svc.ApplyLevelUp(context.Background(), charID, "barbarian", 3); err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	names := storedFeatureNames(t, charStore, charID)
	if names["Frenzy"] != 1 {
		t.Errorf("expected Frenzy from barbarian level 3, got %v", names)
	}
	if names["Mindless Rage"] != 0 {
		t.Error("Mindless Rage keys off barbarian level 6, not the level-7 total")
	}
}
