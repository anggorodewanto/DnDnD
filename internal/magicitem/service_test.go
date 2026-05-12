package magicitem

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records every PublishEncounterSnapshot call.
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

func TestService_PublishForCharacter_FiresWhenInCombat(t *testing.T) {
	encID := uuid.New()
	svc := NewService()
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	svc.PublishForCharacter(context.Background(), uuid.New())

	require.Equal(t, []uuid.UUID{encID}, pub.all())
}

func TestService_PublishForCharacter_NoOpWhenNotInCombat(t *testing.T) {
	svc := NewService()
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{}) // zero encID => not in combat

	svc.PublishForCharacter(context.Background(), uuid.New())

	assert.Empty(t, pub.all())
}

func TestService_PublishForCharacter_NoOpWhenPublisherUnset(t *testing.T) {
	svc := NewService()
	// Intentionally do NOT SetPublisher.
	svc.PublishForCharacter(context.Background(), uuid.New())
}

func TestService_PublishForCharacter_LookupErrorSwallowed(t *testing.T) {
	svc := NewService()
	pub := &fakePublisher{}
	svc.SetPublisher(pub, &fakeEncounterLookup{err: errors.New("db down")})

	svc.PublishForCharacter(context.Background(), uuid.New())

	assert.Empty(t, pub.all(), "lookup error must suppress publish fan-out")
}

func TestService_PublishForCharacter_PublishErrorDoesNotPanic(t *testing.T) {
	encID := uuid.New()
	svc := NewService()
	pub := &fakePublisher{err: errors.New("ws down")}
	svc.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	svc.PublishForCharacter(context.Background(), uuid.New())

	assert.Equal(t, []uuid.UUID{encID}, pub.all(), "fan-out attempt should still record")
}

func TestService_NilReceiver_DoesNotPanic(t *testing.T) {
	var svc *Service
	svc.PublishForCharacter(context.Background(), uuid.New())
}
