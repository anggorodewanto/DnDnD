package narration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeTemplateRefdata struct {
	insertArg refdata.InsertNarrationTemplateParams
	insertErr error
	inserted  refdata.NarrationTemplate

	getID  uuid.UUID
	getErr error
	gotten refdata.NarrationTemplate

	listArg refdata.ListNarrationTemplatesByCampaignParams
	list    []refdata.NarrationTemplate
	listErr error

	updateArg refdata.UpdateNarrationTemplateParams
	updated   refdata.NarrationTemplate
	updateErr error

	deletedID uuid.UUID
	deleteErr error
}

func (f *fakeTemplateRefdata) InsertNarrationTemplate(ctx context.Context, arg refdata.InsertNarrationTemplateParams) (refdata.NarrationTemplate, error) {
	f.insertArg = arg
	if f.insertErr != nil {
		return refdata.NarrationTemplate{}, f.insertErr
	}
	return f.inserted, nil
}

func (f *fakeTemplateRefdata) GetNarrationTemplate(ctx context.Context, id uuid.UUID) (refdata.NarrationTemplate, error) {
	f.getID = id
	if f.getErr != nil {
		return refdata.NarrationTemplate{}, f.getErr
	}
	return f.gotten, nil
}

func (f *fakeTemplateRefdata) ListNarrationTemplatesByCampaign(ctx context.Context, arg refdata.ListNarrationTemplatesByCampaignParams) ([]refdata.NarrationTemplate, error) {
	f.listArg = arg
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func (f *fakeTemplateRefdata) UpdateNarrationTemplate(ctx context.Context, arg refdata.UpdateNarrationTemplateParams) (refdata.NarrationTemplate, error) {
	f.updateArg = arg
	if f.updateErr != nil {
		return refdata.NarrationTemplate{}, f.updateErr
	}
	return f.updated, nil
}

func (f *fakeTemplateRefdata) DeleteNarrationTemplate(ctx context.Context, id uuid.UUID) error {
	f.deletedID = id
	return f.deleteErr
}

func TestTemplateDBStore_Insert_MapsFieldsBothWays(t *testing.T) {
	now := time.Now()
	camp := uuid.New()
	id := uuid.New()
	fake := &fakeTemplateRefdata{inserted: refdata.NarrationTemplate{
		ID: id, CampaignID: camp, Name: "n", Category: "c", Body: "b",
		CreatedAt: now, UpdatedAt: now,
	}}
	store := NewTemplateDBStore(fake)

	tpl, err := store.InsertNarrationTemplate(context.Background(), InsertTemplateParams{
		CampaignID: camp, Name: "n", Category: "c", Body: "b",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.insertArg.CampaignID != camp || fake.insertArg.Name != "n" || fake.insertArg.Category != "c" {
		t.Fatalf("insert arg wrong: %+v", fake.insertArg)
	}
	if tpl.ID != id || tpl.Body != "b" || !tpl.CreatedAt.Equal(now) {
		t.Fatalf("template mismatch: %+v", tpl)
	}
}

func TestTemplateDBStore_Insert_ErrorPropagates(t *testing.T) {
	store := NewTemplateDBStore(&fakeTemplateRefdata{insertErr: errors.New("boom")})
	_, err := store.InsertNarrationTemplate(context.Background(), InsertTemplateParams{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTemplateDBStore_Get_MapsResult(t *testing.T) {
	id := uuid.New()
	fake := &fakeTemplateRefdata{gotten: refdata.NarrationTemplate{ID: id, Name: "n"}}
	store := NewTemplateDBStore(fake)
	tpl, err := store.GetNarrationTemplate(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.getID != id || tpl.Name != "n" {
		t.Fatalf("mismatch")
	}
}

func TestTemplateDBStore_Get_NotFoundError(t *testing.T) {
	store := NewTemplateDBStore(&fakeTemplateRefdata{getErr: errors.New("sql: no rows in result set")})
	_, err := store.GetNarrationTemplate(context.Background(), uuid.New())
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateDBStore_List_PassesFilters(t *testing.T) {
	camp := uuid.New()
	fake := &fakeTemplateRefdata{list: []refdata.NarrationTemplate{
		{ID: uuid.New(), CampaignID: camp, Name: "a"},
		{ID: uuid.New(), CampaignID: camp, Name: "b"},
	}}
	store := NewTemplateDBStore(fake)

	got, err := store.ListNarrationTemplates(context.Background(), TemplateFilter{
		CampaignID: camp, Category: "Combat", Search: "ambush",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.listArg.CampaignID != camp || fake.listArg.Category != "Combat" || fake.listArg.Search != "ambush" {
		t.Fatalf("list arg wrong: %+v", fake.listArg)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestTemplateDBStore_List_ErrorPropagates(t *testing.T) {
	store := NewTemplateDBStore(&fakeTemplateRefdata{listErr: errors.New("boom")})
	_, err := store.ListNarrationTemplates(context.Background(), TemplateFilter{CampaignID: uuid.New()})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTemplateDBStore_Update_MapsArgs(t *testing.T) {
	id := uuid.New()
	fake := &fakeTemplateRefdata{updated: refdata.NarrationTemplate{ID: id, Name: "n2"}}
	store := NewTemplateDBStore(fake)
	tpl, err := store.UpdateNarrationTemplate(context.Background(), id, UpdateTemplateParams{Name: "n2", Body: "b"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.updateArg.ID != id || fake.updateArg.Name != "n2" {
		t.Fatalf("update arg wrong: %+v", fake.updateArg)
	}
	if tpl.Name != "n2" {
		t.Fatalf("name mismatch")
	}
}

func TestTemplateDBStore_Update_NotFoundMapped(t *testing.T) {
	store := NewTemplateDBStore(&fakeTemplateRefdata{updateErr: errors.New("sql: no rows in result set")})
	_, err := store.UpdateNarrationTemplate(context.Background(), uuid.New(), UpdateTemplateParams{Name: "n", Body: "b"})
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateDBStore_Delete_PassesID(t *testing.T) {
	id := uuid.New()
	fake := &fakeTemplateRefdata{}
	store := NewTemplateDBStore(fake)
	if err := store.DeleteNarrationTemplate(context.Background(), id); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.deletedID != id {
		t.Fatalf("delete id mismatch")
	}
}

func TestTemplateDBStore_Delete_ErrorPropagates(t *testing.T) {
	store := NewTemplateDBStore(&fakeTemplateRefdata{deleteErr: errors.New("boom")})
	err := store.DeleteNarrationTemplate(context.Background(), uuid.New())
	if err == nil {
		t.Fatalf("expected error")
	}
}
