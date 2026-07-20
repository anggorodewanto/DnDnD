package refdata

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// seededFeature is the decode target for seeded feature JSON.
type seededFeature struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	MechanicalEffect string `json:"mechanical_effect"`
}

// findSeededJSONArg returns the first captured json.RawMessage argument
// containing marker. It matches on content rather than column position so the
// test does not break when the classes table gains a column.
func findSeededJSONArg(t *testing.T, calls [][]any, marker string) []byte {
	t.Helper()
	for _, args := range calls {
		for _, a := range args {
			raw, ok := a.(json.RawMessage)
			if ok && bytes.Contains(raw, []byte(marker)) {
				return raw
			}
		}
	}
	t.Fatalf("no seeded JSON argument containing %q", marker)
	return nil
}

// TestSeedClasses_FrenzyIs2024 pins the Berserker Frenzy seed to the 2024 PHB
// wording and the `frenzy` mechanical-effect slug the combat rider gates on.
func TestSeedClasses_FrenzyIs2024(t *testing.T) {
	mock := &capturingDBTX{}
	if err := seedClasses(context.Background(), New(mock)); err != nil {
		t.Fatalf("seedClasses failed: %v", err)
	}

	raw := findSeededJSONArg(t, mock.calls, `"berserker"`)

	var decoded map[string]struct {
		FeaturesByLevel map[string][]seededFeature `json:"features_by_level"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshaling barbarian subclasses: %v", err)
	}

	feats := decoded["berserker"].FeaturesByLevel["3"]
	if len(feats) != 1 {
		t.Fatalf("expected one Berserker level-3 feature, got %d", len(feats))
	}
	frenzy := feats[0]
	if frenzy.Name != "Frenzy" {
		t.Fatalf("expected Frenzy, got %q", frenzy.Name)
	}
	if frenzy.MechanicalEffect != "frenzy" {
		t.Errorf("mechanical_effect = %q, want %q", frenzy.MechanicalEffect, "frenzy")
	}

	const want = "If you use Reckless Attack while your Rage is active, you deal extra damage to the first target you hit on your turn with a Strength-based attack. To determine the extra damage, roll a number of d6s equal to your Rage Damage bonus, and add them together. The damage has the same type as the weapon or Unarmed Strike used for the attack."
	if frenzy.Description != want {
		t.Errorf("Frenzy description is not the 2024 text.\n got: %s\nwant: %s", frenzy.Description, want)
	}
	// The 2014 wording granted a bonus-action attack; make sure it is gone.
	if strings.Contains(frenzy.Description, "bonus action") {
		t.Error("Frenzy still describes the 2014 bonus-action attack")
	}
}
