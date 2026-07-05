package levelup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
)

// COV-9 — the Tough feat raises the hit-point maximum by 2 per character level
// the moment it is gained (and grants those hit points, so current HP rises with
// max). ApplyFeat is the feat-acquisition seam; CalculateHP is not re-run on a
// feat pick, so the bump is applied there. Detection is by the seeded
// mechanical-effect slug "hp_plus_2_per_level", not the feat name.

func toughFeat() FeatInfo {
	return FeatInfo{
		ID:               "tough",
		Name:             "Tough",
		MechanicalEffect: []map[string]string{{"effect_type": "hp_plus_2_per_level"}},
	}
}

func alertFeat() FeatInfo {
	return FeatInfo{
		ID:               "alert",
		Name:             "Alert",
		MechanicalEffect: []map[string]string{{"effect_type": "bonus_initiative", "value": "5"}},
	}
}

func TestFeatMaxHPBonus(t *testing.T) {
	tests := []struct {
		name  string
		feat  FeatInfo
		level int32
		want  int32
	}{
		{"tough scales with level", toughFeat(), 4, 8},
		{"non-hp effect in list", alertFeat(), 4, 0},
		{"no mechanical effect", FeatInfo{ID: "x", Name: "X"}, 4, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := featMaxHPBonus(tc.feat, tc.level); got != tc.want {
				t.Errorf("featMaxHPBonus() = %d, want %d", got, tc.want)
			}
		})
	}
}

// seedFeatChar builds a level-4 fighter with the given HP for the ApplyFeat tests.
func seedFeatChar(t *testing.T, store *mockCharacterStore, id uuid.UUID, hpMax, hpCurrent int32) {
	t.Helper()
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 4}})
	store.chars[id] = &StoredCharacter{
		ID:               id,
		Name:             "Aria",
		DiscordUserID:    "user123",
		Level:            4,
		HPMax:            hpMax,
		HPCurrent:        hpCurrent,
		ProficiencyBonus: 2,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
		Features:         mustJSON(t, []character.Feature{{Name: "Second Wind", Source: "fighter", Level: 1}}),
	}
}

// Tough raises max AND current HP by 2×level. Seeded damaged (30/20) so the test
// proves both stores rise by +8 and current is NOT snapped to max (28 ≠ 38); the
// feat is also recorded on the character.
func TestService_ApplyFeat_Tough_RaisesMaxAndCurrentHP(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 20) // level 4 → +8; 10 damage taken

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, toughFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	got := store.chars[charID]
	if got.HPMax != 38 || got.HPCurrent != 28 {
		t.Errorf("HP = %d/%d, want 28/38 (both +8, gap preserved)", got.HPCurrent, got.HPMax)
	}
	var feats []character.Feature
	json.Unmarshal(got.Features, &feats)
	if feats[len(feats)-1].Name != "Tough" {
		t.Errorf("last feature = %s, want Tough", feats[len(feats)-1].Name)
	}
}

// Re-applying Tough is a no-op (idempotency guard) — HP bumps only once.
func TestService_ApplyFeat_Tough_Idempotent(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 30)

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	for i := 0; i < 2; i++ {
		if err := svc.ApplyFeat(context.Background(), charID, toughFeat()); err != nil {
			t.Fatalf("ApplyFeat #%d error: %v", i+1, err)
		}
	}

	if got := store.chars[charID].HPMax; got != 38 {
		t.Errorf("HPMax after double-apply = %d, want 38 (bumped once)", got)
	}
}

// A feat without the HP effect never touches HP.
func TestService_ApplyFeat_NonHPFeat_LeavesHPUnchanged(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 30)

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, alertFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	if got := store.chars[charID]; got.HPMax != 30 || got.HPCurrent != 30 {
		t.Errorf("HP = %d/%d, want 30/30 (non-HP feat)", got.HPCurrent, got.HPMax)
	}
}
