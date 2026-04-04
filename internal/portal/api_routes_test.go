package portal_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-Test-User-ID")
		if userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := auth.ContextWithDiscordUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestAPIRoutes_ListRaces(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	refStore := &mockRefDataStore{
		races: []portal.RaceInfo{{ID: "elf", Name: "Elf", SpeedFt: 30}},
	}
	apiH := portal.NewAPIHandler(slog.Default(), refStore, nil)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	req := httptest.NewRequest(http.MethodGet, "/portal/api/races", nil)
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var races []portal.RaceInfo
	err := json.NewDecoder(rec.Body).Decode(&races)
	require.NoError(t, err)
	assert.Len(t, races, 1)
}

func TestAPIRoutes_ListClasses(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	refStore := &mockRefDataStore{
		classes: []portal.ClassInfo{{ID: "fighter", Name: "Fighter", HitDie: "d10"}},
	}
	apiH := portal.NewAPIHandler(slog.Default(), refStore, nil)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	req := httptest.NewRequest(http.MethodGet, "/portal/api/classes", nil)
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIRoutes_ListSpells(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	refStore := &mockRefDataStore{
		spells: []portal.SpellInfo{{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, Classes: []string{"wizard"}}},
	}
	apiH := portal.NewAPIHandler(slog.Default(), refStore, nil)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells?class=wizard", nil)
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIRoutes_ListEquipment(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	refStore := &mockRefDataStore{
		equipment: []portal.EquipmentItem{{ID: "longsword", Name: "Longsword", Category: "weapon"}},
	}
	apiH := portal.NewAPIHandler(slog.Default(), refStore, nil)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	req := httptest.NewRequest(http.MethodGet, "/portal/api/equipment", nil)
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var items []portal.EquipmentItem
	err := json.NewDecoder(rec.Body).Decode(&items)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestAPIRoutes_GetStartingEquipment(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	apiH := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, nil)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	req := httptest.NewRequest(http.MethodGet, "/portal/api/starting-equipment?class=fighter", nil)
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIRoutes_SubmitCharacter(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	builderStore := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	builderSvc := portal.NewBuilderService(builderStore)
	apiH := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)
	portal.RegisterRoutes(r, h, authMiddleware, portal.WithAPI(apiH))

	body := `{
		"token":"t","campaign_id":"c",
		"name":"Test","race":"elf","background":"sage","class":"wizard",
		"ability_scores":{"str":8,"dex":14,"con":13,"int":15,"wis":12,"cha":10},
		"skills":["arcana"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", "user-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}
