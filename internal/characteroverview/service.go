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

	"github.com/ab/dndnd/internal/character"
)

// ErrInvalidInput is returned when a request to the service fails validation
// (e.g. missing campaign ID).
var ErrInvalidInput = errors.New("invalid character overview input")

// ErrCharacterNotFound is returned when a status edit targets a character that
// does not exist.
var ErrCharacterNotFound = errors.New("character not found")

// editableConditions is the set of persistent, non-exhaustion 5e conditions a
// DM may set on a character out of combat. Exhaustion is edited via its own
// numeric level, not as a named condition here.
var editableConditions = map[string]bool{
	"blinded":       true,
	"charmed":       true,
	"deafened":      true,
	"frightened":    true,
	"grappled":      true,
	"incapacitated": true,
	"invisible":     true,
	"paralyzed":     true,
	"petrified":     true,
	"poisoned":      true,
	"prone":         true,
	"restrained":    true,
	"stunned":       true,
	"unconscious":   true,
}

// maxExhaustionLevel is the 5e cap (level 6 = death).
const maxExhaustionLevel = 6

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
	TempHP            int32           `json:"temp_hp"`
	AC                int32           `json:"ac"`
	SpeedFt           int32           `json:"speed_ft"`
	AbilityScores     json.RawMessage `json:"ability_scores"`
	Languages         []string        `json:"languages"`
	DDBURL            string          `json:"ddb_url"`
	// ExhaustionLevel and Conditions are the persistent (out-of-combat) status
	// the DM can edit from the overview. Conditions is a list of condition names.
	ExhaustionLevel int32    `json:"exhaustion_level"`
	Conditions      []string `json:"conditions"`
	// SpellSlots and PactMagicSlots are the persistent caster resources the DM can
	// edit out of combat. SpellSlots is kept string-keyed ({"1":{...}}) for the
	// wire; PactMagicSlots is nil when the character has no pact magic.
	SpellSlots     map[string]character.SlotInfo `json:"spell_slots"`
	PactMagicSlots *character.PactMagicSlots     `json:"pact_magic_slots"`
}

// CharacterStatus is the persistent live-status of a character returned after a
// successful out-of-combat DM edit.
type CharacterStatus struct {
	HPMax           int32    `json:"hp_max"`
	HPCurrent       int32    `json:"hp_current"`
	TempHP          int32    `json:"temp_hp"`
	ExhaustionLevel int32    `json:"exhaustion_level"`
	Conditions      []string `json:"conditions"`
}

// StatusUpdate is the validated input for an out-of-combat status edit.
type StatusUpdate struct {
	HPMax           int32
	HPCurrent       int32
	TempHP          int32
	ExhaustionLevel int32
	Conditions      []string
	Reason          string
}

// CharacterStatusContext carries the persistence facts the handler needs to
// authorize and apply a status edit without re-reading the character row.
type CharacterStatusContext struct {
	CampaignID     uuid.UUID
	CharacterData  []byte // raw character_data JSON (carries exhaustion_level); may be nil
	InActiveCombat bool
}

// PersistStatusParams is the fully-resolved row update handed to the Store. The
// Conditions and CharacterData are already encoded by the service.
type PersistStatusParams struct {
	CharacterID   uuid.UUID
	HPMax         int32
	HPCurrent     int32
	TempHP        int32
	Conditions    json.RawMessage
	CharacterData []byte
}

// SlotsContext carries the persistence facts the handler needs to authorize and
// apply an out-of-combat spell/pact slot edit (and to render the current values).
//
// FeatureUses carries the raw feature_uses JSON ({"rage":{"current":1,"max":3,
// "recharge":"long"}}); it backs the read-only GetFeatureUses endpoint that
// prefills the in-combat feature-use override editor. May be nil when the
// character has no limited-use features.
type SlotsContext struct {
	CampaignID     uuid.UUID
	InActiveCombat bool
	SpellSlots     map[int]character.SlotInfo
	PactMagicSlots character.PactMagicSlots
	FeatureUses    json.RawMessage
}

// SlotsUpdate is the validated input for an out-of-combat slot edit. A nil
// pointer means "leave that store untouched"; only the provided stores are
// validated and persisted.
type SlotsUpdate struct {
	SpellSlots     *map[int]character.SlotInfo
	PactMagicSlots *character.PactMagicSlots
}

// PersistSlotsParams is the resolved slot update handed to the Store. Each field
// is the already-encoded JSON for that store, or nil to leave it untouched.
type PersistSlotsParams struct {
	CharacterID    uuid.UUID
	SpellSlots     json.RawMessage
	PactMagicSlots json.RawMessage
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
	// GetCharacterStatusContext loads the authorization + persistence context for
	// a status edit. Returns ErrCharacterNotFound when the character is absent.
	GetCharacterStatusContext(ctx context.Context, characterID uuid.UUID) (CharacterStatusContext, error)
	// UpdateCharacterStatus persists a resolved status edit.
	UpdateCharacterStatus(ctx context.Context, params PersistStatusParams) error
	// GetCharacterSlotsContext loads the authorization + persistence context for a
	// spell/pact slot edit. Returns ErrCharacterNotFound when the character is absent.
	GetCharacterSlotsContext(ctx context.Context, characterID uuid.UUID) (SlotsContext, error)
	// UpdateCharacterSlots persists a resolved slot edit (only the provided stores).
	UpdateCharacterSlots(ctx context.Context, params PersistSlotsParams) error
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
