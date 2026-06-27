package characteroverview

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
)

// GetSlotsContext returns the authorization + persistence context for a spell/pact
// slot edit (and the current slot values), delegating to the Store.
func (s *Service) GetSlotsContext(ctx context.Context, characterID uuid.UUID) (SlotsContext, error) {
	if characterID == uuid.Nil {
		return SlotsContext{}, fmt.Errorf("%w: character_id required", ErrInvalidInput)
	}
	return s.store.GetCharacterSlotsContext(ctx, characterID)
}

// ApplySlots validates and persists an out-of-combat spell/pact slot edit. A nil
// pointer in the update leaves that store untouched; only provided stores are
// validated (ErrInvalidInput on failure), encoded, and persisted.
func (s *Service) ApplySlots(ctx context.Context, characterID uuid.UUID, in SlotsUpdate) error {
	params := PersistSlotsParams{CharacterID: characterID}

	if in.SpellSlots != nil {
		if err := character.ValidateSpellSlots(*in.SpellSlots); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		raw, err := character.MarshalSpellSlotsJSON(*in.SpellSlots)
		if err != nil {
			return fmt.Errorf("marshaling spell slots: %w", err)
		}
		params.SpellSlots = raw
	}

	if in.PactMagicSlots != nil {
		if err := character.ValidatePactSlots(*in.PactMagicSlots); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		raw, err := json.Marshal(*in.PactMagicSlots)
		if err != nil {
			return fmt.Errorf("marshaling pact slots: %w", err)
		}
		params.PactMagicSlots = raw
	}

	if err := s.store.UpdateCharacterSlots(ctx, params); err != nil {
		return fmt.Errorf("persisting slots: %w", err)
	}
	return nil
}
