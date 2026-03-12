package encounter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Store defines the database operations needed by the encounter service.
type Store interface {
	CreateEncounterTemplate(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error)
	GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error)
	ListEncounterTemplatesByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.EncounterTemplate, error)
	UpdateEncounterTemplate(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error)
	DeleteEncounterTemplate(ctx context.Context, id uuid.UUID) error
	ListCreatures(ctx context.Context) ([]refdata.Creature, error)
}

// Service manages encounter template CRUD and validation.
type Service struct {
	store Store
}

// NewService creates a new encounter Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateInput holds the parameters for creating an encounter template.
type CreateInput struct {
	CampaignID  uuid.UUID
	MapID       uuid.NullUUID
	Name        string
	DisplayName string
	Creatures   json.RawMessage
}

// Create validates input and creates a new encounter template.
func (s *Service) Create(ctx context.Context, input CreateInput) (refdata.EncounterTemplate, error) {
	if input.Name == "" {
		return refdata.EncounterTemplate{}, errors.New("name must not be empty")
	}

	creatures := input.Creatures
	if len(creatures) == 0 {
		creatures = json.RawMessage(`[]`)
	}

	displayName := sql.NullString{}
	if input.DisplayName != "" {
		displayName = sql.NullString{String: input.DisplayName, Valid: true}
	}

	et, err := s.store.CreateEncounterTemplate(ctx, refdata.CreateEncounterTemplateParams{
		CampaignID:  input.CampaignID,
		MapID:       input.MapID,
		Name:        input.Name,
		DisplayName: displayName,
		Creatures:   creatures,
	})
	if err != nil {
		return refdata.EncounterTemplate{}, fmt.Errorf("creating encounter template: %w", err)
	}

	return et, nil
}

// GetByID retrieves an encounter template by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	return s.store.GetEncounterTemplate(ctx, id)
}

// ListByCampaignID lists all encounter templates for a campaign.
func (s *Service) ListByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.EncounterTemplate, error) {
	return s.store.ListEncounterTemplatesByCampaignID(ctx, campaignID)
}

// UpdateInput holds the parameters for updating an encounter template.
type UpdateInput struct {
	ID          uuid.UUID
	MapID       uuid.NullUUID
	Name        string
	DisplayName string
	Creatures   json.RawMessage
}

// Update validates input and updates an existing encounter template.
func (s *Service) Update(ctx context.Context, input UpdateInput) (refdata.EncounterTemplate, error) {
	if input.Name == "" {
		return refdata.EncounterTemplate{}, errors.New("name must not be empty")
	}

	creatures := input.Creatures
	if len(creatures) == 0 {
		creatures = json.RawMessage(`[]`)
	}

	displayName := sql.NullString{}
	if input.DisplayName != "" {
		displayName = sql.NullString{String: input.DisplayName, Valid: true}
	}

	et, err := s.store.UpdateEncounterTemplate(ctx, refdata.UpdateEncounterTemplateParams{
		ID:          input.ID,
		MapID:       input.MapID,
		Name:        input.Name,
		DisplayName: displayName,
		Creatures:   creatures,
	})
	if err != nil {
		return refdata.EncounterTemplate{}, fmt.Errorf("updating encounter template: %w", err)
	}

	return et, nil
}

// Delete deletes an encounter template by its ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteEncounterTemplate(ctx, id)
}

// ListCreatures returns all available creatures from the stat block library.
func (s *Service) ListCreatures(ctx context.Context) ([]refdata.Creature, error) {
	return s.store.ListCreatures(ctx)
}

// Duplicate creates a copy of an encounter template with a new name.
func (s *Service) Duplicate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	original, err := s.store.GetEncounterTemplate(ctx, id)
	if err != nil {
		return refdata.EncounterTemplate{}, fmt.Errorf("getting encounter template to duplicate: %w", err)
	}

	displayName := sql.NullString{}
	if original.DisplayName.Valid {
		displayName = sql.NullString{String: original.DisplayName.String + " (copy)", Valid: true}
	}

	et, err := s.store.CreateEncounterTemplate(ctx, refdata.CreateEncounterTemplateParams{
		CampaignID:  original.CampaignID,
		MapID:       original.MapID,
		Name:        original.Name + " (copy)",
		DisplayName: displayName,
		Creatures:   original.Creatures,
	})
	if err != nil {
		return refdata.EncounterTemplate{}, fmt.Errorf("duplicating encounter template: %w", err)
	}

	return et, nil
}
