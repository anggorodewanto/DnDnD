package portal

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/sqlc-dev/pqtype"
)

// rawFeatures marshals features into the nullable JSONB shape the character row
// stores them in.
func rawFeatures(t *testing.T, feats []character.Feature) pqtype.NullRawMessage {
	t.Helper()
	b, err := json.Marshal(feats)
	if err != nil {
		t.Fatalf("marshaling features: %v", err)
	}
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}
}

// decodeFeatures unmarshals a preserved result back to names for assertions.
func decodeFeatures(t *testing.T, msg pqtype.NullRawMessage) map[string]bool {
	t.Helper()
	if !msg.Valid {
		return nil
	}
	var feats []character.Feature
	if err := json.Unmarshal(msg.RawMessage, &feats); err != nil {
		t.Fatalf("unmarshaling features: %v", err)
	}
	return featureNames(feats)
}

func mustClassesJSON(t *testing.T, entries []character.ClassEntry) []byte {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshaling classes: %v", err)
	}
	return b
}

// TestPreservePersistedFeats_OutOfBandFeatureSurvivesAdditiveEdit pins the
// Steady Aim case: a feature appended out of band (no Source stamped) that the
// fresh derivation does not reproduce must survive an edit that could only have
// added features.
func TestPreservePersistedFeats_OutOfBandFeatureSurvivesAdditiveEdit(t *testing.T) {
	existing := rawFeatures(t, []character.Feature{
		{Name: "Sneak Attack"},
		{Name: "Steady Aim", MechanicalEffect: "steady_aim"},
	})
	fresh := rawFeatures(t, []character.Feature{{Name: "Sneak Attack"}})

	got := decodeFeatures(t, preservePersistedFeats(existing, fresh, true))

	if !got["Steady Aim"] {
		t.Error("an out-of-band feature must survive a rebuild that could only add features")
	}
	if !got["Sneak Attack"] {
		t.Error("freshly derived features must be kept")
	}
}

// TestPreservePersistedFeats_RespecDropsUnsourcedFeature pins the other side of
// the tradeoff: once the build shape changed we cannot tell a stale class
// feature from an out-of-band grant, so unsourced features are dropped rather
// than resurrected onto the new build.
func TestPreservePersistedFeats_RespecDropsUnsourcedFeature(t *testing.T) {
	existing := rawFeatures(t, []character.Feature{
		{Name: "Rage"},
		{Name: "Steady Aim"},
	})
	fresh := rawFeatures(t, []character.Feature{{Name: "Arcane Recovery"}})

	got := decodeFeatures(t, preservePersistedFeats(existing, fresh, false))

	if got["Rage"] {
		t.Error("a respec must not resurrect the old class's features")
	}
	if got["Steady Aim"] {
		t.Error("unsourced features are indistinguishable from stale class features after a respec")
	}
	if !got["Arcane Recovery"] {
		t.Error("freshly derived features must be kept")
	}
}

// Feats carry an explicit Source, so they are preserved across a respec too —
// this is the pre-existing COV-17 S1 behaviour and must not regress.
func TestPreservePersistedFeats_FeatSurvivesRespec(t *testing.T) {
	existing := rawFeatures(t, []character.Feature{
		{Name: "Rage"},
		{Name: "Tough", Source: featFeatureSource},
	})
	fresh := rawFeatures(t, []character.Feature{{Name: "Arcane Recovery"}})

	got := decodeFeatures(t, preservePersistedFeats(existing, fresh, false))

	if !got["Tough"] {
		t.Error("feat-sourced features must survive a respec")
	}
	if got["Rage"] {
		t.Error("a respec must not resurrect the old class's features")
	}
}

func TestPreservePersistedFeats_NoDuplicateWhenFreshAlreadyHasIt(t *testing.T) {
	existing := rawFeatures(t, []character.Feature{{Name: "Steady Aim"}})
	fresh := rawFeatures(t, []character.Feature{{Name: "Steady Aim"}})

	result := preservePersistedFeats(existing, fresh, true)

	var feats []character.Feature
	if err := json.Unmarshal(result.RawMessage, &feats); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if len(feats) != 1 {
		t.Errorf("expected no duplicate, got %d features: %v", len(feats), feats)
	}
}

func TestPreservePersistedFeats_DegradesToFresh(t *testing.T) {
	fresh := rawFeatures(t, []character.Feature{{Name: "Rage"}})

	if got := preservePersistedFeats(pqtype.NullRawMessage{}, fresh, true); string(got.RawMessage) != string(fresh.RawMessage) {
		t.Error("absent existing features should fall back to fresh")
	}
	bad := pqtype.NullRawMessage{RawMessage: []byte("not json"), Valid: true}
	if got := preservePersistedFeats(bad, fresh, true); string(got.RawMessage) != string(fresh.RawMessage) {
		t.Error("unparseable existing features should fall back to fresh")
	}
}

// TestBuildIsAdditive pins the boundary between an edit that can only grow the
// derived feature set (safe to carry unsourced features across) and one that
// can legitimately retire features (must not resurrect them).
func TestBuildIsAdditive(t *testing.T) {
	base := []character.ClassEntry{{Class: "rogue", Subclass: "thief", Level: 4}}
	noSubclass := []character.ClassEntry{{Class: "rogue", Level: 2}}

	tests := []struct {
		name        string
		storedRace  string
		stored      []character.ClassEntry
		freshRace   string
		fresh       []character.ClassEntry
		want        bool
		explanation string
	}{
		{"identical", "Elf", base, "Elf", base, true, "nothing changed"},
		{"case insensitive", "Elf", base, "elf", []character.ClassEntry{{Class: "Rogue", Subclass: "Thief", Level: 4}}, true, "slug vs display name"},
		{"level raised", "Elf", base, "Elf", []character.ClassEntry{{Class: "rogue", Subclass: "thief", Level: 5}}, true, "level-up only adds"},
		{"subclass chosen", "Elf", noSubclass, "Elf", []character.ClassEntry{{Class: "rogue", Subclass: "thief", Level: 3}}, true, "answering the level-up subclass prompt only adds"},
		{"class added", "Elf", base, "Elf", append(append([]character.ClassEntry{}, base...), character.ClassEntry{Class: "wizard", Level: 1}), true, "multiclassing only adds"},
		{"level lowered", "Elf", base, "Elf", []character.ClassEntry{{Class: "rogue", Subclass: "thief", Level: 3}}, false, "de-levelling retires features"},
		{"subclass swapped", "Elf", base, "Elf", []character.ClassEntry{{Class: "rogue", Subclass: "assassin", Level: 4}}, false, "the old subclass's features must go"},
		{"class replaced", "Elf", base, "Elf", []character.ClassEntry{{Class: "wizard", Level: 4}}, false, "a respec retires the old class"},
		{"class removed", "Elf", append(append([]character.ClassEntry{}, base...), character.ClassEntry{Class: "wizard", Level: 1}), "Elf", base, false, "dropping a class retires its features"},
		{"race changed", "Elf", base, "Human", base, false, "racial traits change"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildIsAdditive(tt.storedRace, mustClassesJSON(t, tt.stored), tt.freshRace, mustClassesJSON(t, tt.fresh))
			if got != tt.want {
				t.Errorf("buildIsAdditive = %v, want %v (%s)", got, tt.want, tt.explanation)
			}
		})
	}
}

func TestBuildIsAdditive_UnparseableIsTreatedAsRespec(t *testing.T) {
	// Without a trustworthy comparison we must assume a respec, which keeps the
	// conservative feat-only preservation rather than resurrecting features.
	if buildIsAdditive("Elf", []byte("not json"), "Elf", mustClassesJSON(t, nil)) {
		t.Error("unparseable stored classes must not be reported as additive")
	}
}
