package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
)

// ApprovalQueries defines the database operations needed by DBApprovalStore.
type ApprovalQueries interface {
	ListPlayerCharactersByStatus(ctx context.Context, arg refdata.ListPlayerCharactersByStatusParams) ([]refdata.ListPlayerCharactersByStatusRow, error)
	GetPlayerCharacterWithCharacter(ctx context.Context, id uuid.UUID) (refdata.GetPlayerCharacterWithCharacterRow, error)
	GetPlayerCharacter(ctx context.Context, id uuid.UUID) (refdata.PlayerCharacter, error)
	UpdatePlayerCharacterStatus(ctx context.Context, arg refdata.UpdatePlayerCharacterStatusParams) (refdata.PlayerCharacter, error)
}

// validApprovalTransitions defines which status transitions are allowed per current status.
var validApprovalTransitions = map[string]map[string]bool{
	"pending": {
		"approved":          true,
		"changes_requested": true,
		"rejected":          true,
		"retired":           true,
	},
}

// DBApprovalStore implements ApprovalStore using the database.
type DBApprovalStore struct {
	queries ApprovalQueries
}

// NewDBApprovalStore creates a new DBApprovalStore.
func NewDBApprovalStore(queries ApprovalQueries) *DBApprovalStore {
	return &DBApprovalStore{queries: queries}
}

// ListPendingApprovals returns all pending player characters for a campaign.
func (s *DBApprovalStore) ListPendingApprovals(ctx context.Context, campaignID uuid.UUID) ([]ApprovalEntry, error) {
	rows, err := s.queries.ListPlayerCharactersByStatus(ctx, refdata.ListPlayerCharactersByStatusParams{
		CampaignID: campaignID,
		Status:     "pending",
	})
	if err != nil {
		return nil, fmt.Errorf("listing pending approvals: %w", err)
	}

	entries := make([]ApprovalEntry, len(rows))
	for i, row := range rows {
		entries[i] = ApprovalEntry{
			ID:            row.ID,
			CharacterID:   row.CharacterID,
			CharacterName: row.CharacterName,
			DiscordUserID: row.DiscordUserID,
			Status:        row.Status,
			CreatedVia:    row.CreatedVia,
			DmFeedback:    row.DmFeedback.String,
		}
	}
	return entries, nil
}

// GetApprovalDetail returns the full character sheet for a pending player character.
func (s *DBApprovalStore) GetApprovalDetail(ctx context.Context, id uuid.UUID) (*ApprovalDetail, error) {
	row, err := s.queries.GetPlayerCharacterWithCharacter(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting approval detail: %w", err)
	}

	return &ApprovalDetail{
		ApprovalEntry: ApprovalEntry{
			ID:            row.ID,
			CharacterID:   row.CharacterID,
			CharacterName: row.CharacterName,
			DiscordUserID: row.DiscordUserID,
			Status:        row.Status,
			CreatedVia:    row.CreatedVia,
			DmFeedback:    row.DmFeedback.String,
		},
		Race:          row.Race,
		Level:         row.Level,
		Classes:       string(row.Classes),
		HpMax:         row.HpMax,
		HpCurrent:     row.HpCurrent,
		Ac:            row.Ac,
		SpeedFt:       row.SpeedFt,
		AbilityScores: string(row.AbilityScores),
		Languages:     strings.Join(row.Languages, ", "),
		DdbURL:        row.DdbUrl.String,
	}, nil
}

// ApproveCharacter transitions a player character from pending to approved.
func (s *DBApprovalStore) ApproveCharacter(ctx context.Context, id uuid.UUID) error {
	return s.transitionStatus(ctx, id, "approved", "")
}

// RequestChanges transitions a player character from pending to changes_requested.
func (s *DBApprovalStore) RequestChanges(ctx context.Context, id uuid.UUID, feedback string) error {
	return s.transitionStatus(ctx, id, "changes_requested", feedback)
}

// RetireCharacter transitions a player character from pending to retired.
func (s *DBApprovalStore) RetireCharacter(ctx context.Context, id uuid.UUID) error {
	return s.transitionStatus(ctx, id, "retired", "")
}

// RejectCharacter transitions a player character from pending to rejected.
func (s *DBApprovalStore) RejectCharacter(ctx context.Context, id uuid.UUID, feedback string) error {
	return s.transitionStatus(ctx, id, "rejected", feedback)
}

func (s *DBApprovalStore) transitionStatus(ctx context.Context, id uuid.UUID, newStatus, feedback string) error {
	current, err := s.queries.GetPlayerCharacter(ctx, id)
	if err != nil {
		return fmt.Errorf("getting player character: %w", err)
	}

	allowed, ok := validApprovalTransitions[current.Status]
	if !ok || !allowed[newStatus] {
		return fmt.Errorf("invalid status transition: %s -> %s", current.Status, newStatus)
	}

	_, err = s.queries.UpdatePlayerCharacterStatus(ctx, refdata.UpdatePlayerCharacterStatusParams{
		ID:         id,
		Status:     newStatus,
		DmFeedback: sql.NullString{String: feedback, Valid: feedback != ""},
	})
	if err != nil {
		return fmt.Errorf("updating status: %w", err)
	}
	return nil
}
