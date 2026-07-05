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

// durableFeat carries a fixed +1 CON ASI bonus (like the seeded Durable feat) —
// the CON-changing feat family that must resync max HP.
func durableFeat() FeatInfo {
	return FeatInfo{
		ID:       "durable",
		Name:     "Durable",
		ASIBonus: map[string]any{"con": 1},
	}
}

// strFeat carries a +1 STR ASI bonus — an ASI feat that must NOT touch HP,
// proving the HP resync is gated on the Constitution modifier, not "any ASI".
func strFeat() FeatInfo {
	return FeatInfo{
		ID:       "str-feat",
		Name:     "Brawny",
		ASIBonus: map[string]any{"str": 1},
	}
}

// TestConHPDelta — a CON-changing feat adds (Δ CON modifier × total level) hit
// points; an odd bump that leaves the modifier unchanged adds nothing.
func TestConHPDelta(t *testing.T) {
	tests := []struct {
		name           string
		oldCON, newCON int
		level          int32
		want           int32
	}{
		{"mod rises 1→2 at level 4", 13, 14, 4, 4},
		{"odd bump keeps modifier", 14, 15, 4, 0},
		{"con unchanged", 14, 14, 4, 0},
		{"mod rises at level 1", 13, 14, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := character.AbilityScores{CON: tc.oldCON}
			neu := character.AbilityScores{CON: tc.newCON}
			if got := conHPDelta(old, neu, tc.level); got != tc.want {
				t.Errorf("conHPDelta() = %d, want %d", got, tc.want)
			}
		})
	}
}

// seedFeatChar builds a level-4 fighter with the given HP and Constitution for
// the ApplyFeat tests.
func seedFeatChar(t *testing.T, store *mockCharacterStore, id uuid.UUID, hpMax, hpCurrent, con int32) {
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
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: int(con), INT: 10, WIS: 12, CHA: 8}),
		Features:         mustJSON(t, []character.Feature{{Name: "Second Wind", Source: "fighter", Level: 1}}),
	}
}

// Tough raises max AND current HP by 2×level. Seeded damaged (30/20) so the test
// proves both stores rise by +8 and current is NOT snapped to max (28 ≠ 38); the
// feat is also recorded on the character.
func TestService_ApplyFeat_Tough_RaisesMaxAndCurrentHP(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 20, 14) // level 4 → +8; 10 damage taken

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
	seedFeatChar(t, store, charID, 30, 30, 14)

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
	seedFeatChar(t, store, charID, 30, 30, 14)

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, alertFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	if got := store.chars[charID]; got.HPMax != 30 || got.HPCurrent != 30 {
		t.Errorf("HP = %d/%d, want 30/30 (non-HP feat)", got.HPCurrent, got.HPMax)
	}
}

// A CON-raising feat (Durable) that lifts the modifier grants +1 HP per level on
// BOTH stores. Seeded damaged (30/20 at CON 13, mod +1) so the +4 lands and the
// 10-point damage gap survives (24 ≠ 34); the new CON is persisted.
func TestService_ApplyFeat_Durable_RaisesMaxAndCurrentHP(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 20, 13) // CON 13 (mod +1); 10 damage taken

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, durableFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	got := store.chars[charID]
	if got.HPMax != 34 || got.HPCurrent != 24 {
		t.Errorf("HP = %d/%d, want 24/34 (both +4, gap preserved)", got.HPCurrent, got.HPMax)
	}
	var scores character.AbilityScores
	json.Unmarshal(got.AbilityScores, &scores)
	if scores.CON != 14 {
		t.Errorf("CON = %d, want 14", scores.CON)
	}
}

// A CON bump that does NOT cross a modifier boundary (14→15, mod stays +2) grants
// no HP — the resync keys off the modifier delta, not the raw score.
func TestService_ApplyFeat_ConFeat_OddBump_LeavesHPUnchanged(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 30, 14) // CON 14 (mod +2)

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, durableFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	if got := store.chars[charID]; got.HPMax != 30 || got.HPCurrent != 30 {
		t.Errorf("HP = %d/%d, want 30/30 (odd CON bump, no modifier change)", got.HPCurrent, got.HPMax)
	}
}

// An ASI feat that raises a non-CON ability never touches HP.
func TestService_ApplyFeat_NonConASIFeat_LeavesHPUnchanged(t *testing.T) {
	charID := uuid.New()
	store := newMockCharacterStore()
	seedFeatChar(t, store, charID, 30, 30, 13) // odd CON, but a STR feat won't touch it

	svc := NewService(store, newMockClassStore(), &mockNotifier{})
	if err := svc.ApplyFeat(context.Background(), charID, strFeat()); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	if got := store.chars[charID]; got.HPMax != 30 || got.HPCurrent != 30 {
		t.Errorf("HP = %d/%d, want 30/30 (non-CON ASI feat)", got.HPCurrent, got.HPMax)
	}
}
