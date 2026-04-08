// Package characteroverview exposes a read-only view of the party's
// approved player character sheets plus a Party Languages aggregation that
// helps the DM see language coverage across the party at a glance.
package characteroverview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidInput is returned when a request to the service fails validation
// (e.g. missing campaign ID).
var ErrInvalidInput = errors.New("invalid character overview input")

// CharacterSheet is the read-only view of a single approved player character
// that the DM dashboard displays in the character overview. Field set mirrors
// what ListPlayerCharactersByStatus exposes from the joined characters row.
type CharacterSheet struct {
	PlayerCharacterID uuid.UUID       `json:"player_character_id"`
	CharacterID       uuid.UUID       `json:"character_id"`
	DiscordUserID     string          `json:"discord_user_id"`
	Name              string          `json:"name"`
	Race              string          `json:"race"`
	Level             int32           `json:"level"`
	Classes           json.RawMessage `json:"classes"`
	HPMax             int32           `json:"hp_max"`
	HPCurrent         int32           `json:"hp_current"`
	AC                int32           `json:"ac"`
	SpeedFt           int32           `json:"speed_ft"`
	AbilityScores     json.RawMessage `json:"ability_scores"`
	Languages         []string        `json:"languages"`
	DDBURL            string          `json:"ddb_url"`
}

// LanguageCoverage is a single (language -> characters who speak it) row
// in the Party Languages summary.
type LanguageCoverage struct {
	Language   string   `json:"language"`
	Characters []string `json:"characters"`
}

// Store is the persistence interface for the character overview.
type Store interface {
	ListApprovedPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]CharacterSheet, error)
}

// Service exposes character-overview read queries and the party-languages
// aggregation helper.
type Service struct {
	store Store
}

// NewService constructs a character-overview service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// ListPartyCharacters returns the approved player characters of a campaign
// for display in the DM dashboard character overview.
func (s *Service) ListPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]CharacterSheet, error) {
	if campaignID == uuid.Nil {
		return nil, fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	return s.store.ListApprovedPartyCharacters(ctx, campaignID)
}

// PartyLanguages aggregates the union of languages spoken across a set of
// character sheets into an alphabetically-sorted list, with the character
// names who speak each language (also sorted). Languages are deduplicated
// case-insensitively, preserving the canonical casing of the first occurrence
// encountered. Blank entries are ignored. Deterministic ordering makes the
// result safe to diff in tests and cache in the UI.
func (s *Service) PartyLanguages(sheets []CharacterSheet) []LanguageCoverage {
	if len(sheets) == 0 {
		return []LanguageCoverage{}
	}

	// canonical[lowerLang] = canonical casing used in the output
	canonical := map[string]string{}
	// speakers[lowerLang] = set of character names (dedup per language)
	speakers := map[string]map[string]struct{}{}

	for _, sheet := range sheets {
		name := strings.TrimSpace(sheet.Name)
		if name == "" {
			continue
		}
		for _, lang := range sheet.Languages {
			trimmed := strings.TrimSpace(lang)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := canonical[key]; !ok {
				canonical[key] = trimmed
				speakers[key] = map[string]struct{}{}
			}
			speakers[key][name] = struct{}{}
		}
	}

	out := make([]LanguageCoverage, 0, len(canonical))
	for key, label := range canonical {
		names := make([]string, 0, len(speakers[key]))
		for n := range speakers[key] {
			names = append(names, n)
		}
		sort.Strings(names)
		out = append(out, LanguageCoverage{Language: label, Characters: names})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Language) < strings.ToLower(out[j].Language)
	})
	return out
}
