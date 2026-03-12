package encounter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	createFn       func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error)
	getFn          func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error)
	listFn         func(ctx context.Context, campaignID uuid.UUID) ([]refdata.EncounterTemplate, error)
	updateFn       func(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error)
	deleteFn       func(ctx context.Context, id uuid.UUID) error
	listCreaturesFn func(ctx context.Context) ([]refdata.Creature, error)
}

func (m *mockStore) CreateEncounterTemplate(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
	return m.createFn(ctx, arg)
}
func (m *mockStore) GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	return m.getFn(ctx, id)
}
func (m *mockStore) ListEncounterTemplatesByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.EncounterTemplate, error) {
	return m.listFn(ctx, campaignID)
}
func (m *mockStore) UpdateEncounterTemplate(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
	return m.updateFn(ctx, arg)
}
func (m *mockStore) DeleteEncounterTemplate(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}
func (m *mockStore) ListCreatures(ctx context.Context) ([]refdata.Creature, error) {
	if m.listCreaturesFn != nil {
		return m.listCreaturesFn(ctx)
	}
	return []refdata.Creature{}, nil
}

func successStore() *mockStore {
	return &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:          uuid.New(),
				CampaignID:  arg.CampaignID,
				MapID:       arg.MapID,
				Name:        arg.Name,
				DisplayName: arg.DisplayName,
				Creatures:   arg.Creatures,
			}, nil
		},
		getFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:         id,
				CampaignID: uuid.New(),
				Name:       "Test Encounter",
				Creatures:  json.RawMessage(`[]`),
			}, nil
		},
		listFn: func(ctx context.Context, campaignID uuid.UUID) ([]refdata.EncounterTemplate, error) {
			return []refdata.EncounterTemplate{}, nil
		},
		updateFn: func(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:          arg.ID,
				Name:        arg.Name,
				DisplayName: arg.DisplayName,
				MapID:       arg.MapID,
				Creatures:   arg.Creatures,
			}, nil
		},
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
}

// --- TDD Cycle 1: Service.Create validates name is not empty ---

func TestService_Create_RejectsEmptyName(t *testing.T) {
	svc := NewService(successStore())
	_, err := svc.Create(context.Background(), CreateInput{
		CampaignID: uuid.New(),
		Name:       "",
		Creatures:  json.RawMessage(`[]`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

// --- TDD Cycle 2: Service.Create success ---

func TestService_Create_Success(t *testing.T) {
	svc := NewService(successStore())
	campaignID := uuid.New()
	mapID := uuid.New()

	et, err := svc.Create(context.Background(), CreateInput{
		CampaignID:  campaignID,
		MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
		Name:        "Goblin Ambush",
		DisplayName: "The Dark Forest",
		Creatures:   json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","quantity":3}]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", et.Name)
	assert.True(t, et.DisplayName.Valid)
	assert.Equal(t, "The Dark Forest", et.DisplayName.String)
	assert.NotEmpty(t, et.ID)
}

// --- TDD Cycle 3: Service.Create defaults creatures to empty array ---

func TestService_Create_DefaultsCreatures(t *testing.T) {
	var captured refdata.CreateEncounterTemplateParams
	store := &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			captured = arg
			return refdata.EncounterTemplate{
				ID:        uuid.New(),
				Name:      arg.Name,
				Creatures: arg.Creatures,
			}, nil
		},
	}
	svc := NewService(store)
	_, err := svc.Create(context.Background(), CreateInput{
		CampaignID: uuid.New(),
		Name:       "Empty Encounter",
	})
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`[]`), captured.Creatures)
}

// --- TDD Cycle 4: Service.Create store error ---

func TestService_Create_StoreError(t *testing.T) {
	store := &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, err := svc.Create(context.Background(), CreateInput{
		CampaignID: uuid.New(),
		Name:       "Test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter template")
}

// --- TDD Cycle 5: Service.Create with no display name keeps it null ---

func TestService_Create_NoDisplayName(t *testing.T) {
	var captured refdata.CreateEncounterTemplateParams
	store := &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			captured = arg
			return refdata.EncounterTemplate{ID: uuid.New(), Name: arg.Name, Creatures: arg.Creatures}, nil
		},
	}
	svc := NewService(store)
	_, err := svc.Create(context.Background(), CreateInput{
		CampaignID: uuid.New(),
		Name:       "Internal Only",
	})
	require.NoError(t, err)
	assert.False(t, captured.DisplayName.Valid)
}

// --- TDD Cycle 6: Service.GetByID ---

func TestService_GetByID(t *testing.T) {
	id := uuid.New()
	svc := NewService(successStore())
	et, err := svc.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, et.ID)
}

func TestService_GetByID_NotFound(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
	}
	svc := NewService(store)
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 7: Service.ListByCampaignID ---

func TestService_ListByCampaignID(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		listFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.EncounterTemplate, error) {
			return []refdata.EncounterTemplate{
				{ID: uuid.New(), Name: "Enc 1", Creatures: json.RawMessage(`[]`)},
				{ID: uuid.New(), Name: "Enc 2", Creatures: json.RawMessage(`[]`)},
			}, nil
		},
	}
	svc := NewService(store)
	list, err := svc.ListByCampaignID(context.Background(), campaignID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

// --- TDD Cycle 8: Service.Update validates name ---

func TestService_Update_RejectsEmptyName(t *testing.T) {
	svc := NewService(successStore())
	_, err := svc.Update(context.Background(), UpdateInput{
		ID:   uuid.New(),
		Name: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestService_Update_Success(t *testing.T) {
	svc := NewService(successStore())
	et, err := svc.Update(context.Background(), UpdateInput{
		ID:          uuid.New(),
		Name:        "Updated Name",
		DisplayName: "Updated Display",
		Creatures:   json.RawMessage(`[{"creature_ref_id":"ogre","short_id":"O1","quantity":1}]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", et.Name)
}

func TestService_Update_StoreError(t *testing.T) {
	store := &mockStore{
		updateFn: func(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, err := svc.Update(context.Background(), UpdateInput{
		ID:   uuid.New(),
		Name: "Test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating encounter template")
}

// --- TDD Cycle 9: Service.Delete ---

func TestService_Delete(t *testing.T) {
	svc := NewService(successStore())
	err := svc.Delete(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestService_Delete_Error(t *testing.T) {
	store := &mockStore{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db error")
		},
	}
	svc := NewService(store)
	err := svc.Delete(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 10: Service.Duplicate ---

func TestService_Duplicate_Success(t *testing.T) {
	originalID := uuid.New()
	campaignID := uuid.New()
	var captured refdata.CreateEncounterTemplateParams
	store := &mockStore{
		getFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:          originalID,
				CampaignID:  campaignID,
				Name:        "Original",
				DisplayName: sql.NullString{String: "Display", Valid: true},
				Creatures:   json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","quantity":2}]`),
			}, nil
		},
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			captured = arg
			return refdata.EncounterTemplate{
				ID:          uuid.New(),
				CampaignID:  arg.CampaignID,
				Name:        arg.Name,
				DisplayName: arg.DisplayName,
				Creatures:   arg.Creatures,
			}, nil
		},
	}
	svc := NewService(store)
	et, err := svc.Duplicate(context.Background(), originalID)
	require.NoError(t, err)
	assert.Equal(t, "Original (copy)", et.Name)
	assert.Equal(t, "Display (copy)", et.DisplayName.String)
	assert.Equal(t, campaignID, captured.CampaignID)
}

func TestService_Duplicate_GetError(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("not found")
		},
	}
	svc := NewService(store)
	_, err := svc.Duplicate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting encounter template to duplicate")
}

func TestService_Duplicate_CreateError(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:        id,
				Name:      "Original",
				Creatures: json.RawMessage(`[]`),
			}, nil
		},
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, err := svc.Duplicate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicating encounter template")
}

// --- TDD Cycle 11: NewService returns non-nil ---

func TestNewService(t *testing.T) {
	svc := NewService(successStore())
	assert.NotNil(t, svc)
}

// --- TDD Cycle 12 (service): ListCreatures ---

func TestService_ListCreatures(t *testing.T) {
	store := successStore()
	store.listCreaturesFn = func(ctx context.Context) ([]refdata.Creature, error) {
		return []refdata.Creature{
			{ID: "goblin", Name: "Goblin"},
		}, nil
	}
	svc := NewService(store)
	creatures, err := svc.ListCreatures(context.Background())
	require.NoError(t, err)
	assert.Len(t, creatures, 1)
	assert.Equal(t, "Goblin", creatures[0].Name)
}
