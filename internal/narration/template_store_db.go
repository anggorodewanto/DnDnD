package narration

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// TemplateRefdataQueries is the minimal subset of *refdata.Queries the
// TemplateDBStore needs. Declared as an interface so unit tests can swap a
// fake.
type TemplateRefdataQueries interface {
	InsertNarrationTemplate(ctx context.Context, arg refdata.InsertNarrationTemplateParams) (refdata.NarrationTemplate, error)
	GetNarrationTemplate(ctx context.Context, id uuid.UUID) (refdata.NarrationTemplate, error)
	ListNarrationTemplatesByCampaign(ctx context.Context, arg refdata.ListNarrationTemplatesByCampaignParams) ([]refdata.NarrationTemplate, error)
	UpdateNarrationTemplate(ctx context.Context, arg refdata.UpdateNarrationTemplateParams) (refdata.NarrationTemplate, error)
	DeleteNarrationTemplate(ctx context.Context, id uuid.UUID) error
}

// TemplateDBStore implements TemplateStore on top of sqlc-generated queries.
type TemplateDBStore struct {
	q TemplateRefdataQueries
}

// NewTemplateDBStore constructs a TemplateDBStore.
func NewTemplateDBStore(q TemplateRefdataQueries) *TemplateDBStore {
	return &TemplateDBStore{q: q}
}

// InsertNarrationTemplate persists a new template row.
func (s *TemplateDBStore) InsertNarrationTemplate(ctx context.Context, p InsertTemplateParams) (Template, error) {
	row, err := s.q.InsertNarrationTemplate(ctx, refdata.InsertNarrationTemplateParams{
		CampaignID: p.CampaignID,
		Name:       p.Name,
		Category:   p.Category,
		Body:       p.Body,
	})
	if err != nil {
		return Template{}, err
	}
	return templateFromRefdata(row), nil
}

// GetNarrationTemplate fetches a template by id, mapping a missing row to
// ErrTemplateNotFound.
func (s *TemplateDBStore) GetNarrationTemplate(ctx context.Context, id uuid.UUID) (Template, error) {
	row, err := s.q.GetNarrationTemplate(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return Template{}, ErrTemplateNotFound
		}
		return Template{}, err
	}
	return templateFromRefdata(row), nil
}

// ListNarrationTemplates returns templates filtered by category/search.
func (s *TemplateDBStore) ListNarrationTemplates(ctx context.Context, filter TemplateFilter) ([]Template, error) {
	rows, err := s.q.ListNarrationTemplatesByCampaign(ctx, refdata.ListNarrationTemplatesByCampaignParams{
		CampaignID: filter.CampaignID,
		Category:   filter.Category,
		Search:     filter.Search,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateFromRefdata(r))
	}
	return out, nil
}

// UpdateNarrationTemplate updates an existing template by id.
func (s *TemplateDBStore) UpdateNarrationTemplate(ctx context.Context, id uuid.UUID, p UpdateTemplateParams) (Template, error) {
	row, err := s.q.UpdateNarrationTemplate(ctx, refdata.UpdateNarrationTemplateParams{
		ID:       id,
		Name:     p.Name,
		Category: p.Category,
		Body:     p.Body,
	})
	if err != nil {
		if isNoRows(err) {
			return Template{}, ErrTemplateNotFound
		}
		return Template{}, err
	}
	return templateFromRefdata(row), nil
}

// DeleteNarrationTemplate removes a template by id.
func (s *TemplateDBStore) DeleteNarrationTemplate(ctx context.Context, id uuid.UUID) error {
	return s.q.DeleteNarrationTemplate(ctx, id)
}

func templateFromRefdata(r refdata.NarrationTemplate) Template {
	return Template{
		ID:         r.ID,
		CampaignID: r.CampaignID,
		Name:       r.Name,
		Category:   r.Category,
		Body:       r.Body,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// isNoRows returns true if err is sql.ErrNoRows or its message says so. The
// string check covers wrapped errors from sqlc-generated code.
func isNoRows(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return err != nil && err.Error() == "sql: no rows in result set"
}
