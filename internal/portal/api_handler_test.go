package portal_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRefDataStore implements portal.RefDataStore for API handler tests.
type mockRefDataStore struct {
	races   []portal.RaceInfo
	classes []portal.ClassInfo
	spells  []portal.SpellInfo
}

func (m *mockRefDataStore) ListRaces(_ context.Context) ([]portal.RaceInfo, error) {
	return m.races, nil
}

func (m *mockRefDataStore) ListClasses(_ context.Context) ([]portal.ClassInfo, error) {
	return m.classes, nil
}

func (m *mockRefDataStore) ListSpellsByClass(_ context.Context, class string) ([]portal.SpellInfo, error) {
	var result []portal.SpellInfo
	for _, s := range m.spells {
		for _, c := range s.Classes {
			if c == class {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func TestAPIHandler_ListRaces(t *testing.T) {
	store := &mockRefDataStore{
		races: []portal.RaceInfo{
			{ID: "dwarf", Name: "Dwarf", SpeedFt: 25},
			{ID: "elf", Name: "Elf", SpeedFt: 30},
		},
	}
	h := portal.NewAPIHandler(slog.Default(), store, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/api/races", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ListRaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var races []portal.RaceInfo
	err := json.NewDecoder(rec.Body).Decode(&races)
	require.NoError(t, err)
	assert.Len(t, races, 2)
	assert.Equal(t, "Dwarf", races[0].Name)
}

func TestAPIHandler_ListClasses(t *testing.T) {
	store := &mockRefDataStore{
		classes: []portal.ClassInfo{
			{ID: "fighter", Name: "Fighter", HitDie: "d10"},
		},
	}
	h := portal.NewAPIHandler(slog.Default(), store, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/api/classes", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ListClasses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var classes []portal.ClassInfo
	err := json.NewDecoder(rec.Body).Decode(&classes)
	require.NoError(t, err)
	assert.Len(t, classes, 1)
	assert.Equal(t, "Fighter", classes[0].Name)
}

func TestAPIHandler_ListSpells(t *testing.T) {
	store := &mockRefDataStore{
		spells: []portal.SpellInfo{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, Classes: []string{"wizard", "sorcerer"}},
			{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, Classes: []string{"cleric"}},
		},
	}
	h := portal.NewAPIHandler(slog.Default(), store, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells?class=wizard", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ListSpells(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var spells []portal.SpellInfo
	err := json.NewDecoder(rec.Body).Decode(&spells)
	require.NoError(t, err)
	assert.Len(t, spells, 1)
	assert.Equal(t, "Fire Bolt", spells[0].Name)
}

func TestAPIHandler_ListSpells_MissingClass(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ListSpells(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIHandler_SubmitCharacter_Valid(t *testing.T) {
	refStore := &mockRefDataStore{}
	builderStore := &mockBuilderStore{
		charID: "char-1",
		pcID:   "pc-1",
	}
	builderSvc := portal.NewBuilderService(builderStore)
	h := portal.NewAPIHandler(slog.Default(), refStore, builderSvc)

	body := `{
		"token": "tok-abc",
		"campaign_id": "campaign-1",
		"name": "Thorin",
		"race": "dwarf",
		"background": "soldier",
		"class": "fighter",
		"ability_scores": {"str":15,"dex":14,"con":13,"int":12,"wis":10,"cha":8},
		"skills": ["athletics","perception"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.SubmitCharacter(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var result map[string]string
	err := json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "char-1", result["character_id"])
	assert.Equal(t, "pc-1", result["player_character_id"])
}

func TestAPIHandler_SubmitCharacter_Unauthenticated(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	h.SubmitCharacter(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIHandler_SubmitCharacter_InvalidJSON(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, portal.NewBuilderService(&mockBuilderStore{}))

	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.SubmitCharacter(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIHandler_SubmitCharacter_ValidationError(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, portal.NewBuilderService(&mockBuilderStore{}))

	body := `{"token":"tok","campaign_id":"c1","name":"","race":"","class":"","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8}}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.SubmitCharacter(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
