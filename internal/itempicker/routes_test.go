package itempicker_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/itempicker"
	"github.com/ab/dndnd/internal/refdata"
)

func noopAuth(next http.Handler) http.Handler {
	return next
}

func TestRegisterRoutes_Search(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{
			{ID: "dagger", Name: "Dagger", Damage: "1d4", DamageType: "piercing", WeaponType: "simple_melee"},
		},
	}
	h := itempicker.NewHandler(store)

	r := chi.NewRouter()
	itempicker.RegisterRoutes(r, h, noopAuth)

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/campaigns/test-camp/items/search?q=dagger")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "Dagger", results[0].Name)
}

func TestRegisterRoutes_CreatureInventories(t *testing.T) {
	store := &stubStore{}
	h := itempicker.NewHandler(store)

	r := chi.NewRouter()
	itempicker.RegisterRoutes(r, h, noopAuth)

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/campaigns/test-camp/encounters/00000000-0000-0000-0000-000000000001/creature-inventories/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result itempicker.CreatureInventoriesResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Len(t, result.Creatures, 0)
}
