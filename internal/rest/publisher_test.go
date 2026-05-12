package rest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records every PublishEncounterSnapshot call so tests can
// verify rest.Service invokes the publisher after rest writes.
type fakePublisher struct {
	mu    sync.Mutex
	calls []uuid.UUID
	err   error
}

func (f *fakePublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, encounterID)
	return f.err
}

func (f *fakePublisher) all() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeEncounterLookup returns a preset encounter ID (or zero / error) for
// any character.
type fakeEncounterLookup struct {
	encID uuid.UUID
	err   error
}

func (f *fakeEncounterLookup) ActiveEncounterIDForCharacter(_ context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	if f.err != nil {
		return uuid.Nil, false, f.err
	}
	if f.encID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return f.encID, true, nil
}

func TestPublishForCharacter_FiresWhenInCombat(t *testing.T) {
	encID := uuid.New()
	charID := uuid.New()
	svc := NewService(nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	svc.PublishForCharacter(context.Background(), charID)

	require.Equal(t, []uuid.UUID{encID}, pub.all())
}

func TestPublishForCharacter_NoOpWhenNotInCombat(t *testing.T) {
	charID := uuid.New()
	svc := NewService(nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{}) // zero encID → not in combat

	svc.PublishForCharacter(context.Background(), charID)

	assert.Empty(t, pub.all())
}

func TestPublishForCharacter_NoOpWhenPublisherUnset(t *testing.T) {
	svc := NewService(nil)
	// Intentionally do NOT SetPublisher; should not panic.
	svc.PublishForCharacter(context.Background(), uuid.New())
}

func TestPublishForCharacter_LookupErrorSwallowed(t *testing.T) {
	svc := NewService(nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{err: errors.New("db down")})

	svc.PublishForCharacter(context.Background(), uuid.New())

	assert.Empty(t, pub.all(), "lookup error should suppress fan-out")
}

func TestPublishForCharacter_PublishErrorDoesNotPanic(t *testing.T) {
	encID := uuid.New()
	svc := NewService(nil)
	pub := &fakePublisher{err: errors.New("ws down")}
	svc.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	// Should swallow the error silently.
	svc.PublishForCharacter(context.Background(), uuid.New())

	assert.Equal(t, []uuid.UUID{encID}, pub.all(), "fan-out attempt should still record")
}
