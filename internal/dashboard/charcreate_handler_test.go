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
	// Verify new wizard steps exist
	assert.Contains(t, rec.Body.String(), "4. Equipment")
	assert.Contains(t, rec.Body.String(), "5. Spells")
	assert.Contains(t, rec.Body.String(), "6. Features")
	assert.Contains(t, rec.Body.String(), "7. Review")
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

func TestCharCreateHandler_HandleCreate_WithEquipmentAndSpells(t *testing.T) {
	mockSvc := &mockDMCharCreateService{
		result: portal.CreateCharacterResult{
			CharacterID:       "char-full",
			PlayerCharacterID: "pc-full",
		},
	}
	h := NewCharCreateHandler(nil, mockSvc, nil)

	sub := dmCreateRequest{
		CampaignID: "campaign-1",
		DMCharacterSubmission: DMCharacterSubmission{
			Name: "Elara",
			Race: "Elf",
			Classes: []character.ClassEntry{
				{Class: "Wizard", Level: 1},
			},
			AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 12, CHA: 10},
			Equipment:     []string{"quarterstaff"},
			Spells:        []string{"fire-bolt", "shield"},
			Languages:     []string{"Common", "Elvish"},
		},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleCreate(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, mockSvc.called)
	assert.Equal(t, []string{"quarterstaff"}, mockSvc.lastSub.Equipment)
	assert.Equal(t, []string{"fire-bolt", "shield"}, mockSvc.lastSub.Spells)
	assert.Equal(t, []string{"Common", "Elvish"}, mockSvc.lastSub.Languages)
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

func TestCharCreateHandler_HandlePreview_IncludesSpellSlots(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)

	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10},
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
	assert.NotNil(t, stats.SpellSlots)
	assert.Equal(t, 4, stats.SpellSlots[1])
	assert.Equal(t, 2, stats.SpellSlots[2])
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
	races        []portal.RaceInfo
	classes      []portal.ClassInfo
	equipment    []portal.EquipmentItem
	spells       []portal.SpellInfo
	raceErr      error
	classErr     error
	equipmentErr error
	spellsErr    error
}

func (m *mockRefDataForCreate) ListRaces(ctx context.Context) ([]portal.RaceInfo, error) {
	return m.races, m.raceErr
}

func (m *mockRefDataForCreate) ListClasses(ctx context.Context) ([]portal.ClassInfo, error) {
	return m.classes, m.classErr
}

func (m *mockRefDataForCreate) ListEquipment(ctx context.Context) ([]portal.EquipmentItem, error) {
	return m.equipment, m.equipmentErr
}

func (m *mockRefDataForCreate) ListSpellsByClass(ctx context.Context, class string) ([]portal.SpellInfo, error) {
	return m.spells, m.spellsErr
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

func TestCharCreateHandler_HandleListRefEquipment_Success(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		equipment: []portal.EquipmentItem{
			{ID: "longsword", Name: "Longsword", Category: "weapon"},
			{ID: "chain-mail", Name: "Chain Mail", Category: "armor"},
		},
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefEquipment(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var items []portal.EquipmentItem
	err := json.NewDecoder(rec.Body).Decode(&items)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestCharCreateHandler_HandleListRefEquipment_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	rec := httptest.NewRecorder()

	h.HandleListRefEquipment(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleListRefEquipment_NilRefData(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefEquipment(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefEquipment_Error(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		equipmentErr: errors.New("db error"),
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefEquipment(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCharCreateHandler_HandleListRefSpells_Success(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		spells: []portal.SpellInfo{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, Classes: []string{"Wizard"}},
			{ID: "shield", Name: "Shield", Level: 1, Classes: []string{"Wizard"}},
		},
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var spells []portal.SpellInfo
	err := json.NewDecoder(rec.Body).Decode(&spells)
	require.NoError(t, err)
	assert.Len(t, spells, 2)
}

func TestCharCreateHandler_HandleListRefSpells_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleListRefSpells_MissingClassParam(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCharCreateHandler_HandleListRefSpells_NilRefData(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_RegisterRoutes_EquipmentEndpoint(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	mockRef := &mockRefDataForCreate{
		equipment: []portal.EquipmentItem{
			{ID: "longsword", Name: "Longsword", Category: "weapon"},
		},
	}
	h := NewHandler(nil, hub)
	ch := NewCharCreateHandler(nil, nil, mockRef)
	r := chi.NewRouter()
	RegisterRoutes(r, h, mockAuthMiddleware)
	ch.RegisterCharCreateRoutes(r.With(mockAuthMiddleware))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCharCreateHandler_RegisterRoutes_SpellsEndpoint(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	mockRef := &mockRefDataForCreate{
		spells: []portal.SpellInfo{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, Classes: []string{"Wizard"}},
		},
	}
	h := NewHandler(nil, hub)
	ch := NewCharCreateHandler(nil, nil, mockRef)
	r := chi.NewRouter()
	RegisterRoutes(r, h, mockAuthMiddleware)
	ch.RegisterCharCreateRoutes(r.With(mockAuthMiddleware))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCharCreateHandler_HandleListRefEquipment_NullResponse(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		equipment: nil,
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/equipment", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefEquipment(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefSpells_NullResponse(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		spells: nil,
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandlePreview_IncludesFeatures(t *testing.T) {
	featureProvider := &mockFeatureProvider{
		classFeatures: map[string]map[string][]character.Feature{
			"Fighter": {
				"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
				"2": {{Name: "Action Surge", Source: "Fighter", Level: 2, Description: "Extra action"}},
			},
		},
	}
	h := NewCharCreateHandler(nil, nil, nil)
	h.SetFeatureProvider(featureProvider)

	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 2},
		},
		AbilityScores: character.AbilityScores{STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandlePreview(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]json.RawMessage
	err := json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)

	// Features should be in the preview response
	featuresRaw, ok := result["features"]
	require.True(t, ok, "preview should include features")

	var features []character.Feature
	err = json.Unmarshal(featuresRaw, &features)
	require.NoError(t, err)
	assert.Len(t, features, 2)
	assert.Equal(t, "Fighting Style", features[0].Name)
	assert.Equal(t, "Action Surge", features[1].Name)
}

func TestCharCreateHandler_HandlePreview_NoFeatureProvider(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)

	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	body, _ := json.Marshal(sub)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/characters/preview", bytes.NewBuffer(body))
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandlePreview(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCharCreateHandler_HandleListRefStartingEquipment_Success(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/starting-equipment?class=Fighter", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefStartingEquipment(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var packs []portal.EquipmentPack
	err := json.NewDecoder(rec.Body).Decode(&packs)
	require.NoError(t, err)
	assert.NotEmpty(t, packs)
}

func TestCharCreateHandler_HandleListRefStartingEquipment_MissingClass(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/starting-equipment", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefStartingEquipment(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCharCreateHandler_HandleListRefStartingEquipment_UnknownClass(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/starting-equipment?class=Unknown", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefStartingEquipment(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestCharCreateHandler_HandleListRefSpells_FiltersByMaxLevel(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		spells: []portal.SpellInfo{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, Classes: []string{"Wizard"}},
			{ID: "magic-missile", Name: "Magic Missile", Level: 1, Classes: []string{"Wizard"}},
			{ID: "fireball", Name: "Fireball", Level: 3, Classes: []string{"Wizard"}},
			{ID: "shield", Name: "Shield", Level: 1, Classes: []string{"Wizard"}},
			{ID: "scorching-ray", Name: "Scorching Ray", Level: 2, Classes: []string{"Wizard"}},
		},
	}
	h := NewCharCreateHandler(nil, nil, mockRef)

	// Request spells with max_level=1 (cantrips + 1st level only)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard&max_level=1", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var spells []portal.SpellInfo
	err := json.NewDecoder(rec.Body).Decode(&spells)
	require.NoError(t, err)
	// Should only return cantrips (level 0) and 1st level spells
	assert.Len(t, spells, 3) // fire-bolt(0), magic-missile(1), shield(1)
	for _, sp := range spells {
		assert.LessOrEqual(t, sp.Level, 1)
	}
}

func TestCharCreateHandler_HandleListRefStartingEquipment_RequiresAuth(t *testing.T) {
	h := NewCharCreateHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/starting-equipment?class=Fighter", nil)
	rec := httptest.NewRecorder()

	h.HandleListRefStartingEquipment(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCharCreateHandler_HandleListRefSpells_Error(t *testing.T) {
	mockRef := &mockRefDataForCreate{
		spellsErr: errors.New("db error"),
	}
	h := NewCharCreateHandler(nil, nil, mockRef)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/characters/ref/spells?class=Wizard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-user"))
	rec := httptest.NewRecorder()

	h.HandleListRefSpells(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
