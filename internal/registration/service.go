package registration

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
)

// ResultStatus indicates the outcome of a registration attempt.
type ResultStatus int

const (
	ResultExactMatch ResultStatus = iota
	ResultFuzzyMatch
	ResultNoMatch
)

// RegisterResult is returned by Register to indicate the match outcome.
type RegisterResult struct {
	Status          ResultStatus
	PlayerCharacter *refdata.PlayerCharacter
	Suggestions     []string
}

// Service handles player character registration and status transitions.
type Service struct {
	queries *refdata.Queries
}

// NewService creates a new registration service.
func NewService(queries *refdata.Queries) *Service {
	return &Service{queries: queries}
}

// validTransitions defines which status transitions are allowed.
// Only transitions from "pending" are permitted.
var validTransitions = map[string]map[string]bool{
	"pending": {
		"approved":          true,
		"changes_requested": true,
		"rejected":          true,
		"retired":           true,
	},
}

// Register attempts to register a player for a character by name.
// It performs case-insensitive exact matching first, then fuzzy matching.
func (s *Service) Register(ctx context.Context, campaignID uuid.UUID, discordUserID, characterName string) (*RegisterResult, error) {
	// Try case-insensitive exact match
	char, err := s.queries.FindCharacterByNameCaseInsensitive(ctx, refdata.FindCharacterByNameCaseInsensitiveParams{
		CampaignID: campaignID,
		Lower:      characterName,
	})
	if err == nil {
		// Exact match found — create pending player_character
		pc, createErr := s.queries.CreatePlayerCharacter(ctx, refdata.CreatePlayerCharacterParams{
			CampaignID:    campaignID,
			CharacterID:   char.ID,
			DiscordUserID: discordUserID,
			Status:        "pending",
			CreatedVia:    "register",
		})
		if createErr != nil {
			return nil, fmt.Errorf("creating player character: %w", createErr)
		}
		return &RegisterResult{
			Status:          ResultExactMatch,
			PlayerCharacter: &pc,
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("finding character: %w", err)
	}

	// No exact match — try fuzzy matching
	names, err := s.queries.ListCharacterNamesByCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("listing character names: %w", err)
	}

	candidates := make([]string, len(names))
	for i, n := range names {
		candidates[i] = n.Name
	}

	matches := FindFuzzyMatches(characterName, candidates, 3)
	if len(matches) > 0 {
		suggestions := make([]string, len(matches))
		for i, m := range matches {
			suggestions[i] = m.Name
		}
		return &RegisterResult{
			Status:      ResultFuzzyMatch,
			Suggestions: suggestions,
		}, nil
	}

	return &RegisterResult{
		Status: ResultNoMatch,
	}, nil
}

// Import creates a pending player_character via import.
func (s *Service) Import(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error) {
	return s.createPC(ctx, campaignID, discordUserID, characterID, "import")
}

// Create creates a pending player_character via portal creation.
func (s *Service) Create(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error) {
	return s.createPC(ctx, campaignID, discordUserID, characterID, "create")
}

func (s *Service) createPC(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID, via string) (*refdata.PlayerCharacter, error) {
	pc, err := s.queries.CreatePlayerCharacter(ctx, refdata.CreatePlayerCharacterParams{
		CampaignID:    campaignID,
		CharacterID:   characterID,
		DiscordUserID: discordUserID,
		Status:        "pending",
		CreatedVia:    via,
	})
	if err != nil {
		return nil, fmt.Errorf("creating player character: %w", err)
	}
	return &pc, nil
}

// Approve transitions a player_character from pending to approved.
func (s *Service) Approve(ctx context.Context, id uuid.UUID) (*refdata.PlayerCharacter, error) {
	return s.transitionStatus(ctx, id, "approved", "")
}

// RequestChanges transitions a player_character from pending to changes_requested.
func (s *Service) RequestChanges(ctx context.Context, id uuid.UUID, feedback string) (*refdata.PlayerCharacter, error) {
	return s.transitionStatus(ctx, id, "changes_requested", feedback)
}

// Reject transitions a player_character from pending to rejected.
func (s *Service) Reject(ctx context.Context, id uuid.UUID, feedback string) (*refdata.PlayerCharacter, error) {
	return s.transitionStatus(ctx, id, "rejected", feedback)
}

// Retire transitions a player_character from pending to retired.
func (s *Service) Retire(ctx context.Context, id uuid.UUID) (*refdata.PlayerCharacter, error) {
	return s.transitionStatus(ctx, id, "retired", "")
}

// GetStatus returns the current player_character for a discord user in a campaign.
func (s *Service) GetStatus(ctx context.Context, campaignID uuid.UUID, discordUserID string) (*refdata.PlayerCharacter, error) {
	pc, err := s.queries.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaignID,
		DiscordUserID: discordUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("getting player character: %w", err)
	}
	return &pc, nil
}

func (s *Service) transitionStatus(ctx context.Context, id uuid.UUID, newStatus, feedback string) (*refdata.PlayerCharacter, error) {
	current, err := s.queries.GetPlayerCharacter(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting player character: %w", err)
	}

	allowed, ok := validTransitions[current.Status]
	if !ok || !allowed[newStatus] {
		return nil, fmt.Errorf("invalid status transition: %s -> %s", current.Status, newStatus)
	}

	feedbackNull := sql.NullString{}
	if feedback != "" {
		feedbackNull = sql.NullString{String: feedback, Valid: true}
	}

	pc, err := s.queries.UpdatePlayerCharacterStatus(ctx, refdata.UpdatePlayerCharacterStatusParams{
		ID:         id,
		Status:     newStatus,
		DmFeedback: feedbackNull,
	})
	if err != nil {
		return nil, fmt.Errorf("updating status: %w", err)
	}
	return &pc, nil
}
