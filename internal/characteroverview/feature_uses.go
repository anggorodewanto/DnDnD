package characteroverview

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
)

// ApplyFeatureUses validates and persists an out-of-combat feature-uses edit.
// existingFeatureUses is the character's current feature_uses JSON (carried in
// the SlotsContext). Each change sets that named feature's Current, preserving
// its Max + Recharge metadata; only features already present on the character
// may be edited. Returns the merged feature_uses map on success.
//
// ErrInvalidInput is returned for an empty change set, an unknown feature, a
// negative Current, or a Current above a bounded Max (a Max < 0 marks an
// unlimited pool, so only the lower bound is enforced).
func (s *Service) ApplyFeatureUses(ctx context.Context, characterID uuid.UUID, existingFeatureUses json.RawMessage, changes []FeatureUseChange) (map[string]character.FeatureUse, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("%w: no feature changes provided", ErrInvalidInput)
	}

	uses := map[string]character.FeatureUse{}
	if len(existingFeatureUses) > 0 {
		if err := json.Unmarshal(existingFeatureUses, &uses); err != nil {
			return nil, fmt.Errorf("parsing feature_uses: %w", err)
		}
	}

	for _, ch := range changes {
		fu, ok := uses[ch.Feature]
		if !ok {
			return nil, fmt.Errorf("%w: unknown feature %q", ErrInvalidInput, ch.Feature)
		}
		if ch.Current < 0 {
			return nil, fmt.Errorf("%w: current for %q must be >= 0", ErrInvalidInput, ch.Feature)
		}
		if fu.Max >= 0 && ch.Current > fu.Max {
			return nil, fmt.Errorf("%w: current %d exceeds max %d for %q", ErrInvalidInput, ch.Current, fu.Max, ch.Feature)
		}
		fu.Current = ch.Current
		uses[ch.Feature] = fu
	}

	raw, err := json.Marshal(uses)
	if err != nil {
		return nil, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if err := s.store.UpdateCharacterFeatureUses(ctx, PersistFeatureUsesParams{
		CharacterID: characterID,
		FeatureUses: raw,
	}); err != nil {
		return nil, fmt.Errorf("persisting feature_uses: %w", err)
	}
	return uses, nil
}
