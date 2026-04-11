package levelup

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
)

// fakePublisher records PublishEncounterSnapshot calls.
type fakePublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
	err       error
}

func (f *fakePublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, encounterID)
	return f.err
}

func (f *fakePublisher) calls() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.published))
	copy(out, f.published)
	return out
}

// fakeLookup returns a preset encounter for any character.
type fakeLookup struct {
	encID uuid.UUID
	err   error
}

func (f *fakeLookup) ActiveEncounterIDForCharacter(_ context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	if f.err != nil {
		return uuid.Nil, false, f.err
	}
	if f.encID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return f.encID, true, nil
}

// levelUpFixture builds a minimal Service wired to in-memory mocks so we can
// exercise ApplyLevelUp() without pulling in the full TestApplyLevelUp suite.
func levelUpFixture(t *testing.T) (*Service, uuid.UUID) {
	t.Helper()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()

	charID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 1}})
	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Hero",
		Level:            1,
		HPMax:            10,
		HPCurrent:        10,
		ProficiencyBonus: 2,
		Classes:          classes,
		AbilityScores:    scores,
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 2: 1, 5: 2},
		SubclassLevel:    3,
		FeaturesByLevel:  map[string][]character.Feature{},
	}

	svc := NewService(charStore, classStore, &mockNotifier{})
	return svc, charID
}

func TestService_ApplyLevelUp_PublishesSnapshot(t *testing.T) {
	svc, charID := levelUpFixture(t)
	encID := uuid.New()
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeLookup{encID: encID})

	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 2)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestService_ApplyLevelUp_NotInActiveEncounter_NoPublish(t *testing.T) {
	svc, charID := levelUpFixture(t)
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeLookup{}) // encID zero

	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 2)
	require.NoError(t, err)
	assert.Empty(t, pub.calls())
}

func TestService_ApplyLevelUp_NilPublisherTolerated(t *testing.T) {
	svc, charID := levelUpFixture(t)
	// Intentionally do NOT SetPublisher.
	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 2)
	require.NoError(t, err)
}

func TestService_ApplyLevelUp_PublishErrorSwallowed(t *testing.T) {
	svc, charID := levelUpFixture(t)
	encID := uuid.New()
	pub := &fakePublisher{err: errors.New("hub full")}
	svc.SetPublisher(pub, &fakeLookup{encID: encID})

	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 2)
	require.NoError(t, err)
	assert.Len(t, pub.calls(), 1)
}

func TestService_ApplyLevelUp_StoreError_NoPublish(t *testing.T) {
	svc, _ := levelUpFixture(t)
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeLookup{encID: uuid.New()})

	// Unknown character ID forces GetCharacterForLevelUp to fail.
	_, err := svc.ApplyLevelUp(context.Background(), uuid.New(), "fighter", 2)
	require.Error(t, err)
	assert.Empty(t, pub.calls())
}
