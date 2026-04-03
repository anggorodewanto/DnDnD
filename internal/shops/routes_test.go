package shops_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
)

func noopAuth(next http.Handler) http.Handler {
	return next
}

func TestRegisterRoutes_CreateAndListShops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	handler := shops.NewHandler(svc)

	r := chi.NewRouter()
	shops.RegisterRoutes(r, handler, noopAuth)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Create shop
	body, _ := json.Marshal(map[string]string{"name": "Route Test Shop"})
	resp, err := http.Post(ts.URL+"/api/campaigns/"+camp.ID.String()+"/shops", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created refdata.Shop
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "Route Test Shop", created.Name)

	// List shops
	resp, err = http.Get(ts.URL + "/api/campaigns/" + camp.ID.String() + "/shops")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var list []refdata.Shop
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	assert.Len(t, list, 1)
}

func TestRegisterRoutes_GetShop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	handler := shops.NewHandler(svc)

	r := chi.NewRouter()
	shops.RegisterRoutes(r, handler, noopAuth)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Create shop
	body, _ := json.Marshal(map[string]string{"name": "Get Test"})
	resp, err := http.Post(ts.URL+"/api/campaigns/"+camp.ID.String()+"/shops", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created refdata.Shop
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Get it via route
	resp, err = http.Get(ts.URL + "/api/campaigns/" + camp.ID.String() + "/shops/" + created.ID.String())
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result shops.ShopResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "Get Test", result.Shop.Name)
}
