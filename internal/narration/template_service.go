package narration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrTemplateNotFound is returned when a template lookup misses.
var ErrTemplateNotFound = errors.New("narration template not found")

// CreateTemplateInput is the caller-provided payload for Create.
type CreateTemplateInput struct {
	CampaignID uuid.UUID
	Name       string
	Category   string
	Body       string
}

// UpdateTemplateInput is the caller-provided payload for Update.
type UpdateTemplateInput struct {
	Name     string
	Category string
	Body     string
}

// InsertTemplateParams is the normalized payload the TemplateStore persists.
type InsertTemplateParams struct {
	CampaignID uuid.UUID
	Name       string
	Category   string
	Body       string
}

// UpdateTemplateParams is the normalized payload for an update call.
type UpdateTemplateParams struct {
	Name     string
	Category string
	Body     string
}

// TemplateFilter scopes a List call.
type TemplateFilter struct {
	CampaignID uuid.UUID
	Category   string
	Search     string
}

// TemplateStore is the persistence interface for narration templates.
type TemplateStore interface {
	InsertNarrationTemplate(ctx context.Context, p InsertTemplateParams) (Template, error)
	GetNarrationTemplate(ctx context.Context, id uuid.UUID) (Template, error)
	ListNarrationTemplates(ctx context.Context, filter TemplateFilter) ([]Template, error)
	UpdateNarrationTemplate(ctx context.Context, id uuid.UUID, p UpdateTemplateParams) (Template, error)
	DeleteNarrationTemplate(ctx context.Context, id uuid.UUID) error
}

// TemplateService orchestrates narration template CRUD and apply operations.
type TemplateService struct {
	store TemplateStore
}

// NewTemplateService constructs a TemplateService.
func NewTemplateService(store TemplateStore) *TemplateService {
	return &TemplateService{store: store}
}

// Create validates and persists a new template.
func (s *TemplateService) Create(ctx context.Context, in CreateTemplateInput) (Template, error) {
	if in.CampaignID == uuid.Nil {
		return Template{}, fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	name, category, err := validateNameBody(in.Name, in.Category, in.Body)
	if err != nil {
		return Template{}, err
	}
	return s.store.InsertNarrationTemplate(ctx, InsertTemplateParams{
		CampaignID: in.CampaignID,
		Name:       name,
		Category:   category,
		Body:       in.Body,
	})
}

// Get fetches a template by id.
func (s *TemplateService) Get(ctx context.Context, id uuid.UUID) (Template, error) {
	return s.store.GetNarrationTemplate(ctx, id)
}

// List returns templates for a campaign filtered by category/search.
func (s *TemplateService) List(ctx context.Context, filter TemplateFilter) ([]Template, error) {
	if filter.CampaignID == uuid.Nil {
		return nil, fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Search = strings.TrimSpace(filter.Search)
	return s.store.ListNarrationTemplates(ctx, filter)
}

// Update validates and applies changes to an existing template.
func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, in UpdateTemplateInput) (Template, error) {
	name, category, err := validateNameBody(in.Name, in.Category, in.Body)
	if err != nil {
		return Template{}, err
	}
	return s.store.UpdateNarrationTemplate(ctx, id, UpdateTemplateParams{
		Name:     name,
		Category: category,
		Body:     in.Body,
	})
}

// Delete removes a template by id.
func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteNarrationTemplate(ctx, id)
}

// Duplicate copies a template into a new row whose name has " (copy)" appended.
func (s *TemplateService) Duplicate(ctx context.Context, id uuid.UUID) (Template, error) {
	src, err := s.store.GetNarrationTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	return s.store.InsertNarrationTemplate(ctx, InsertTemplateParams{
		CampaignID: src.CampaignID,
		Name:       src.Name + " (copy)",
		Category:   src.Category,
		Body:       src.Body,
	})
}

// Apply fetches the template and substitutes its placeholders with values.
// Tokens missing from values are left as `{name}` so the caller can spot
// omissions in the rendered preview.
func (s *TemplateService) Apply(ctx context.Context, id uuid.UUID, values map[string]string) (string, error) {
	tpl, err := s.store.GetNarrationTemplate(ctx, id)
	if err != nil {
		return "", err
	}
	return SubstitutePlaceholders(tpl.Body, values), nil
}

// validateNameBody trims and validates the name/category/body trio shared by
// Create and Update. It returns the trimmed name and category on success.
func validateNameBody(name, category, body string) (string, string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", "", fmt.Errorf("%w: name required", ErrInvalidInput)
	}
	if strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("%w: body required", ErrInvalidInput)
	}
	return trimmedName, strings.TrimSpace(category), nil
}
