package open5e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHandlerRouter(t *testing.T, upstream *httptest.Server, store CacheStore) chi.Router {
	t.Helper()
	svc := NewService(NewClient(upstream.URL+"/", nil), NewCache(store))
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestHandler_SearchMonsters_OK(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			assert.Equal(t, "abol", q.Get("search"))
			assert.Equal(t, "tome-of-beasts", q.Get("document__slug"))
			assert.Equal(t, "5", q.Get("limit"))
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"aboleth","name":"Aboleth","document__slug":"tome-of-beasts"}]}`)
		},
	})
	defer upstream.Close()

	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/monsters?search=abol&document=tome-of-beasts&limit=5&offset=0", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp MonsterListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
}

func TestHandler_SearchMonsters_UpstreamError(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		},
	})
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/monsters", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestHandler_SearchMonsters_InvalidLimit(t *testing.T) {
	r := newHandlerRouter(t, newMockAPI(t, nil), &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/monsters?limit=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_SearchMonsters_InvalidOffset(t *testing.T) {
	r := newHandlerRouter(t, newMockAPI(t, nil), &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/monsters?offset=xyz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CacheMonster_OK(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/aboleth/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"slug":"aboleth","name":"Aboleth","size":"Large","type":"aberration","challenge_rating":"10","document__slug":"tome-of-beasts"}`)
		},
	})
	defer upstream.Close()
	store := &fakeCacheStore{}
	r := newHandlerRouter(t, upstream, store)

	req := httptest.NewRequest(http.MethodPost, "/api/open5e/monsters/aboleth", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "open5e_aboleth", resp["id"])
	require.Len(t, store.creatures, 1)
}

func TestHandler_CacheMonster_UpstreamError(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/unknown/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		},
	})
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/monsters/unknown", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

// Whitespace-only slugs (e.g. a URL-encoded space) must be rejected before
// hitting Open5e so we don't burn a network call on garbage input.
func TestHandler_CacheMonster_WhitespaceSlug(t *testing.T) {
	upstream := newMockAPI(t, nil)
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/monsters/%20", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_SearchSpells_OK(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"fireball","name":"Fireball","level_int":3,"document__slug":"deep-magic"}]}`)
		},
	})
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/spells?search=fire", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp SpellListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
}

func TestHandler_CacheSpell_OK(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/fireball/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"slug":"fireball","name":"Fireball","level_int":3,"school":"evocation","document__slug":"deep-magic"}`)
		},
	})
	defer upstream.Close()
	store := &fakeCacheStore{}
	r := newHandlerRouter(t, upstream, store)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/spells/fireball", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, store.spells, 1)
}

func TestHandler_CacheSpell_UpstreamError(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/nope/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		},
	})
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/spells/nope", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestHandler_SearchSpells_UpstreamError(t *testing.T) {
	upstream := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		},
	})
	defer upstream.Close()
	r := newHandlerRouter(t, upstream, &fakeCacheStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/spells", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}
