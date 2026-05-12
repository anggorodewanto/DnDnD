package levelup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
)

// recordingCardUpdater records OnCharacterUpdated calls so SR-007 tests can
// assert each level-up / ASI / feat path fires the persistent
// #character-cards refresh on success.
type recordingCardUpdater struct {
	calls []uuid.UUID
}

func (r *recordingCardUpdater) OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error {
	r.calls = append(r.calls, characterID)
	return nil
}

// SR-007: ApplyLevelUp MUST fire OnCharacterUpdated on a successful write so
// the persistent #character-cards message picks up the new level / HP / etc.
func TestService_ApplyLevelUp_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)
	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Aria",
		DiscordUserID:    "user123",
		Level:            5,
		HPMax:            44,
		HPCurrent:        44,
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}
	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}

	svc := NewService(charStore, classStore, notifier)
	rec := &recordingCardUpdater{}
	svc.SetCardUpdater(rec)

	if _, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 6); err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("OnCharacterUpdated calls = %d, want 1", len(rec.calls))
	}
	if rec.calls[0] != charID {
		t.Errorf("OnCharacterUpdated arg = %s, want %s", rec.calls[0], charID)
	}
}

// SR-007: ApproveASI MUST fire the card update so the new ability score line
// reaches the #character-cards message.
func TestService_ApproveASI_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Brom",
		AbilityScores: mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	svc := NewService(charStore, classStore, notifier)
	rec := &recordingCardUpdater{}
	svc.SetCardUpdater(rec)

	choice := ASIChoice{Type: ASIPlus2, Ability: "str"}
	if err := svc.ApproveASI(context.Background(), charID, choice); err != nil {
		t.Fatalf("ApproveASI error: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != charID {
		t.Errorf("expected one OnCharacterUpdated(%s), got %v", charID, rec.calls)
	}
}

// SR-007: ApplyFeat (no ASI bonus path) MUST fire the card update so the
// new feature line reaches the #character-cards message.
func TestService_ApplyFeat_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Cira",
		AbilityScores: mustJSON(t, character.AbilityScores{STR: 14, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 10}),
		Features:      json.RawMessage("[]"),
	}

	svc := NewService(charStore, classStore, notifier)
	rec := &recordingCardUpdater{}
	svc.SetCardUpdater(rec)

	feat := FeatInfo{Name: "Tough"}
	if err := svc.ApplyFeat(context.Background(), charID, feat); err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != charID {
		t.Errorf("expected one OnCharacterUpdated(%s), got %v", charID, rec.calls)
	}
}
