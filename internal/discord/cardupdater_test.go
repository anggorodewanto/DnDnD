package discord

import (
	"context"

	"github.com/google/uuid"
)

// recordingCardUpdater records OnCharacterUpdated calls so SR-007 subscriber
// tests can assert that each non-combat mutator fires the persistent
// #character-cards refresh exactly once on the success path.
type recordingCardUpdater struct {
	calls []uuid.UUID
	err   error
}

func (r *recordingCardUpdater) OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error {
	r.calls = append(r.calls, characterID)
	return r.err
}
