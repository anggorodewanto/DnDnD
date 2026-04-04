package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDMCharCreateService implements the service interface for testing.
type mockDMCharCreateService struct {
	result portal.CreateCharacterResult
	err    error
	called bool
	lastSub DMCharacterSubmission
}

func (m *mockDMCharCreateService) CreateCharacter(ctx context.Context, campaignID string, sub DMCharacterSubmission) (portal.CreateCharacterResult, error) {
	m.called = true
	m.lastSub = sub
	return m.result, m.err
}

func TestCharCreateHandler_ServeCreatePage_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/characters/new", nil)
	rec := httptest.NewRecorder()

	h.ServeCreatePage(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_ServeCreatePage_ReturnsHTML(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/characters/new", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.ServeCreatePage(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "Create Character")
}

func TestCharCreateHandler_HandleCreate_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", nil)
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleCreate_InvalidJSON(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", bytes.NewBufferString("not json"))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCharCreateHandler_HandleCreate_Success(t *testing.T) {
	mockSvc := &mockDMCharCreateService{
		result: portal.CreateCharacterResult{
			CharacterID:       "char-123",
			PlayerCharacterID: "pc-456",
		},
	}
	h := NewCharCreateHandler(nil, mockSvc, nil)

	sub := dmCreateRequest{
		CampaignID: "campaign-1",
		DMCharacterSubmission: DMCharacterSubmission{
			Name: "Thorin",
			Race: "Dwarf",
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 5},
			},
			AbilityScores: character.AbilityScores{
				STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
			},
		},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp map[string]string
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "char-123", resp["character_id"])
	assert.Equal(t, "pc-456", resp["player_character_id"])
}

func TestCharCreateHandler_HandleCreate_ValidationError(t *testing.T) {
	mockSvc := &mockDMCharCreateService{
		err: errors.New("validation failed: name is required"),
	}
	h := NewCharCreateHandler(nil, mockSvc, nil)

	sub := dmCreateRequest{
		CampaignID: "campaign-1",
		DMCharacterSubmission: DMCharacterSubmission{
			Name: "Thorin",
			Race: "Dwarf",
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 1},
			},
			AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCharCreateHandler_HandleCreate_InternalError(t *testing.T) {
	mockSvc := &mockDMCharCreateService{
		err: errors.New("database exploded"),
	}
	h := NewCharCreateHandler(nil, mockSvc, nil)

	sub := dmCreateRequest{
		CampaignID: "campaign-1",
		DMCharacterSubmission: DMCharacterSubmission{
			Name: "Thorin",
			Race: "Dwarf",
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 1},
			},
			AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCharCreateHandler_HandlePreview_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", nil)
	rec := httptest.NewRecorder()

	h.HandlePreview(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandlePreview_InvalidJSON(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", bytes.NewBufferString("bad"))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandlePreview(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCharCreateHandler_HandlePreview_ReturnsDerivedStats(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)

	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandlePreview(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats DMDerivedStats
	err := json.NewDecoder(rec.Body).Decode(&stats)
	require.NoError(t, err)
	assert.Equal(t, 44, stats.HPMax)
	assert.Equal(t, 11, stats.AC)
	assert.Equal(t, 3, stats.ProficiencyBonus)
	assert.Equal(t, 5, stats.TotalLevel)
}

func TestCharCreateHandler_HandleListRefRaces_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/races", nil)
	rec := httptest.NewRecorder()

	h.HandleListRefRaces(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleListRefRaces_Success(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		races: []portal.RaceInfo{
			{ID: "dwarf", Name: "Dwarf", SpeedFt: 25},
			{ID: "elf", Name: "Elf", SpeedFt: 30},
		},
	}
	h := NewCharCreateHandler(nil, nil, mockRef)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/races", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefRaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var races []portal.RaceInfo
	err := json.NewDecoder(rec.Body).Decode(&races)
	require.NoError(t, err)
	assert.Len(t, races, 2)
}

func TestCharCreateHandler_HandleListRefClasses_Success(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		classes: []portal.ClassInfo{
			{ID: "fighter", Name: "Fighter", HitDie: "d10"},
		},
	}
	h := NewCharCreateHandler(nil, nil, mockRef)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/classes", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefClasses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var classes []portal.ClassInfo
	err := json.NewDecoder(rec.Body).Decode(&classes)
	require.NoError(t, err)
	assert.Len(t, classes, 1)
}

// mockRefDataForCreate implements RefDataForCreate for testing.
type mockRefDataForCreate struct {
	races    []portal.RaceInfo
	classes  []portal.ClassInfo
	raceErr  error
	classErr error
}

func (m *mockRefDataForCreate) ListRaces(ctx context.Context) ([]portal.RaceInfo, error) {
	return m.races, m.raceErr
}

func (m *mockRefDataForCreate) ListClasses(ctx context.Context) ([]portal.ClassInfo, error) {
	return m.classes, m.classErr
}

func TestCharCreateHandler_RegisterRoutes_CreatePageEndpoint(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	h := NewHandler(nil, hub)
	ch := NewCharCreateHandler(nil, nil, nil)
	r := chi.NewRouter()
	RegisterRoutes(r, h, mockAuthMiddleware)
	ch.RegisterCharCreateRoutes(r.With(mockAuthMiddleware))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/characters/new", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Create Character")
}

func TestCharCreateHandler_RegisterRoutes_PreviewEndpoint(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	h := NewHandler(nil, hub)
	ch := NewCharCreateHandler(nil, nil, nil)
	r := chi.NewRouter()
	RegisterRoutes(r, h, mockAuthMiddleware)
	ch.RegisterCharCreateRoutes(r.With(mockAuthMiddleware))

	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{{Class: "Fighter", Level: 1}},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCharCreateHandler_HandleListRefRaces_NilRefData(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/races", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefRaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefClasses_NilRefData(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/classes", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefClasses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefRaces_Error(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		raceErr: errors.New("db error"),
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/races", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefRaces(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCharCreateHandler_HandleListRefClasses_NullResponse(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		classes: nil,
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/classes", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefClasses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefRaces_NullResponse(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		races: nil,
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/races", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefRaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefClasses_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/classes", nil)
	rec := httptest.NewRecorder()

	h.HandleListRefClasses(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleListRefClasses_Error(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		classErr: errors.New("db error"),
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/classes", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefClasses(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
