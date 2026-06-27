package characteroverview

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/rest"
)

// persistedCondition mirrors combat.CombatCondition's JSON shape so a condition
// the DM sets out of combat is read back identically by the portal sheet and by
// the combatant seeding that happens at combat start.
type persistedCondition struct {
	Condition      string `json:"condition"`
	DurationRounds int    `json:"duration_rounds"`
	StartedRound   int    `json:"started_round"`
}

// GetStatusContext returns the authorization + persistence context for a status
// edit, delegating to the Store.
func (s *Service) GetStatusContext(ctx context.Context, characterID uuid.UUID) (CharacterStatusContext, error) {
	if characterID == uuid.Nil {
		return CharacterStatusContext{}, fmt.Errorf("%w: character_id required", ErrInvalidInput)
	}
	return s.store.GetCharacterStatusContext(ctx, characterID)
}

// ApplyStatus validates an out-of-combat status edit, encodes conditions and the
// exhaustion-bearing character_data, persists the change, and returns the
// resulting status. existingCharData is the character's current character_data
// JSON (from GetStatusContext) into which the new exhaustion level is merged.
func (s *Service) ApplyStatus(ctx context.Context, characterID uuid.UUID, existingCharData []byte, in StatusUpdate) (CharacterStatus, error) {
	conditions, err := normalizeConditions(in.Conditions)
	if err != nil {
		return CharacterStatus{}, err
	}
	if err := validateVitals(in); err != nil {
		return CharacterStatus{}, err
	}

	condJSON, err := encodeConditions(conditions)
	if err != nil {
		return CharacterStatus{}, fmt.Errorf("encoding conditions: %w", err)
	}
	charData := rest.CharacterDataWithExhaustion(existingCharData, int(in.ExhaustionLevel))

	if err := s.store.UpdateCharacterStatus(ctx, PersistStatusParams{
		CharacterID:   characterID,
		HPMax:         in.HPMax,
		HPCurrent:     in.HPCurrent,
		TempHP:        in.TempHP,
		Conditions:    condJSON,
		CharacterData: charData,
	}); err != nil {
		return CharacterStatus{}, fmt.Errorf("persisting status: %w", err)
	}

	return CharacterStatus{
		HPMax:           in.HPMax,
		HPCurrent:       in.HPCurrent,
		TempHP:          in.TempHP,
		ExhaustionLevel: in.ExhaustionLevel,
		Conditions:      conditions,
	}, nil
}

// validateVitals enforces sane HP and exhaustion bounds.
func validateVitals(in StatusUpdate) error {
	if in.HPMax < 1 {
		return fmt.Errorf("%w: hp_max must be at least 1", ErrInvalidInput)
	}
	if in.HPCurrent < 0 || in.HPCurrent > in.HPMax {
		return fmt.Errorf("%w: hp_current must be between 0 and hp_max", ErrInvalidInput)
	}
	if in.TempHP < 0 {
		return fmt.Errorf("%w: temp_hp must not be negative", ErrInvalidInput)
	}
	if in.ExhaustionLevel < 0 || in.ExhaustionLevel > maxExhaustionLevel {
		return fmt.Errorf("%w: exhaustion_level must be between 0 and %d", ErrInvalidInput, maxExhaustionLevel)
	}
	return nil
}

// normalizeConditions lowercases, trims, dedupes and sorts the requested
// conditions, rejecting any name outside editableConditions.
func normalizeConditions(names []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, raw := range names {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !editableConditions[name] {
			return nil, fmt.Errorf("%w: unknown condition %q", ErrInvalidInput, raw)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// encodeConditions marshals condition names into the JSONB array shape stored on
// the characters row (always a non-nil array, never null).
func encodeConditions(names []string) (json.RawMessage, error) {
	entries := make([]persistedCondition, 0, len(names))
	for _, n := range names {
		entries = append(entries, persistedCondition{Condition: n})
	}
	return json.Marshal(entries)
}

// conditionNamesFromJSON extracts condition names from a stored conditions JSONB
// array. Empty/invalid input yields an empty slice.
func conditionNamesFromJSON(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var entries []persistedCondition
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Condition != "" {
			out = append(out, e.Condition)
		}
	}
	return out
}
