package narration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeTemplateStore struct {
	templates  map[uuid.UUID]Template
	createErr  error
	getErr     error
	listErr    error
	updateErr  error
	deleteErr  error
	lastFilter TemplateFilter
}

func newFakeTemplateStore() *fakeTemplateStore {
	return &fakeTemplateStore{templates: map[uuid.UUID]Template{}}
}

func (f *fakeTemplateStore) InsertNarrationTemplate(ctx context.Context, p InsertTemplateParams) (Template, error) {
	if f.createErr != nil {
		return Template{}, f.createErr
	}
	t := Template{
		ID:         uuid.New(),
		CampaignID: p.CampaignID,
		Name:       p.Name,
		Category:   p.Category,
		Body:       p.Body,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	f.templates[t.ID] = t
	return t, nil
}

func (f *fakeTemplateStore) GetNarrationTemplate(ctx context.Context, id uuid.UUID) (Template, error) {
	if f.getErr != nil {
		return Template{}, f.getErr
	}
	t, ok := f.templates[id]
	if !ok {
		return Template{}, ErrTemplateNotFound
	}
	return t, nil
}

func (f *fakeTemplateStore) ListNarrationTemplates(ctx context.Context, filter TemplateFilter) ([]Template, error) {
	f.lastFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []Template{}
	for _, t := range f.templates {
		if t.CampaignID != filter.CampaignID {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeTemplateStore) UpdateNarrationTemplate(ctx context.Context, id uuid.UUID, p UpdateTemplateParams) (Template, error) {
	if f.updateErr != nil {
		return Template{}, f.updateErr
	}
	t, ok := f.templates[id]
	if !ok {
		return Template{}, ErrTemplateNotFound
	}
	t.Name = p.Name
	t.Category = p.Category
	t.Body = p.Body
	t.UpdatedAt = time.Now()
	f.templates[id] = t
	return t, nil
}

func (f *fakeTemplateStore) DeleteNarrationTemplate(ctx context.Context, id uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.templates, id)
	return nil
}

func TestTemplateService_Create_RejectsBlankName(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: uuid.New(),
		Name:       "   ",
		Body:       "hi",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTemplateService_Create_RejectsBlankBody(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: uuid.New(),
		Name:       "n",
		Body:       "  ",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTemplateService_Create_RejectsNilCampaign(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Create(context.Background(), CreateTemplateInput{Name: "n", Body: "b"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTemplateService_Create_Success(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()

	tpl, err := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp,
		Name:       "Tavern",
		Category:   "Locations",
		Body:       "Welcome to {tavern_name}.",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if tpl.Name != "Tavern" || tpl.Category != "Locations" || tpl.CampaignID != camp {
		t.Fatalf("unexpected template: %+v", tpl)
	}
	if len(store.templates) != 1 {
		t.Fatalf("expected stored template")
	}
}

func TestTemplateService_List_PassesFilter(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()

	_, err := svc.List(context.Background(), TemplateFilter{
		CampaignID: camp,
		Category:   "Combat",
		Search:     "ambush",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if store.lastFilter.CampaignID != camp || store.lastFilter.Category != "Combat" || store.lastFilter.Search != "ambush" {
		t.Fatalf("filter passthrough wrong: %+v", store.lastFilter)
	}
}

func TestTemplateService_List_RejectsNilCampaign(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.List(context.Background(), TemplateFilter{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTemplateService_Update_Success(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()

	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp, Name: "n", Body: "b",
	})

	updated, err := svc.Update(context.Background(), created.ID, camp, UpdateTemplateInput{
		Name:     "renamed",
		Category: "Cat",
		Body:     "new body",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if updated.Name != "renamed" || updated.Body != "new body" {
		t.Fatalf("update mismatch: %+v", updated)
	}
}

func TestTemplateService_Update_ValidatesInput(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateTemplateInput{Name: "", Body: "b"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTemplateService_Delete_Success(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp, Name: "n", Body: "b",
	})
	if err := svc.Delete(context.Background(), created.ID, camp); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, exists := store.templates[created.ID]; exists {
		t.Fatalf("expected delete to remove")
	}
}

func TestTemplateService_Get_Success(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp, Name: "n", Body: "b",
	})
	got, err := svc.Get(context.Background(), created.ID, camp)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("id mismatch")
	}
}

func TestTemplateService_Duplicate_AppendsCopySuffix(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp, Name: "Tavern", Category: "Loc", Body: "hi {x}",
	})
	dup, err := svc.Duplicate(context.Background(), created.ID, camp)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if dup.ID == created.ID {
		t.Fatalf("expected new id")
	}
	if dup.Name != "Tavern (copy)" {
		t.Fatalf("expected (copy) suffix, got %q", dup.Name)
	}
	if dup.Body != "hi {x}" || dup.Category != "Loc" || dup.CampaignID != camp {
		t.Fatalf("dup mismatch: %+v", dup)
	}
	if len(store.templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(store.templates))
	}
}

func TestTemplateService_Duplicate_NotFound(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Duplicate(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateService_ApplyTemplate_SubstitutesTokens(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	camp := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: camp,
		Name:       "Greeting",
		Body:       "Hello {player}, welcome to {place}.",
	})
	out, err := svc.Apply(context.Background(), created.ID, camp, map[string]string{
		"player": "Aragorn",
		"place":  "Bree",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out != "Hello Aragorn, welcome to Bree." {
		t.Fatalf("apply got %q", out)
	}
}

func TestTemplateService_ApplyTemplate_NotFound(t *testing.T) {
	svc := NewTemplateService(newFakeTemplateStore())
	_, err := svc.Apply(context.Background(), uuid.New(), uuid.New(), nil)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateService_Get_CrossCampaign_ReturnsNotFound(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	campA := uuid.New()
	campB := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: campA, Name: "n", Body: "b",
	})
	_, err := svc.Get(context.Background(), created.ID, campB)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound for cross-campaign Get, got %v", err)
	}
}

func TestTemplateService_Delete_CrossCampaign_ReturnsNotFound(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	campA := uuid.New()
	campB := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: campA, Name: "n", Body: "b",
	})
	err := svc.Delete(context.Background(), created.ID, campB)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound for cross-campaign Delete, got %v", err)
	}
}

func TestTemplateService_Update_CrossCampaign_ReturnsNotFound(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	campA := uuid.New()
	campB := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: campA, Name: "n", Body: "b",
	})
	_, err := svc.Update(context.Background(), created.ID, campB, UpdateTemplateInput{
		Name: "new", Body: "new body",
	})
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound for cross-campaign Update, got %v", err)
	}
}

func TestTemplateService_Duplicate_CrossCampaign_ReturnsNotFound(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	campA := uuid.New()
	campB := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: campA, Name: "n", Body: "b",
	})
	_, err := svc.Duplicate(context.Background(), created.ID, campB)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound for cross-campaign Duplicate, got %v", err)
	}
}

func TestTemplateService_Apply_CrossCampaign_ReturnsNotFound(t *testing.T) {
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	campA := uuid.New()
	campB := uuid.New()
	created, _ := svc.Create(context.Background(), CreateTemplateInput{
		CampaignID: campA, Name: "n", Body: "Hello {x}.",
	})
	_, err := svc.Apply(context.Background(), created.ID, campB, nil)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound for cross-campaign Apply, got %v", err)
	}
}