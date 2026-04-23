package open5e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test helpers ---

func newMockAPI(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.HandleFunc(path, handler)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("unexpected request: %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

// --- Cycle 1: NewClient defaults ---

func TestNewClient_DefaultsBaseURL(t *testing.T) {
	c := NewClient("", nil)
	assert.Equal(t, "https://api.open5e.com/v1/", c.baseURL)
	assert.NotNil(t, c.http)
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	c := NewClient("http://example.com/", http.DefaultClient)
	assert.Equal(t, "http://example.com/", c.baseURL)
}

func TestNewClient_TrailingSlashNormalized(t *testing.T) {
	c := NewClient("http://example.com", nil)
	assert.Equal(t, "http://example.com/", c.baseURL)
}

// --- Cycle 2: SearchMonsters ---

func TestClient_SearchMonsters_Success(t *testing.T) {
	var gotQuery string
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"aboleth","name":"Aboleth","size":"Large","type":"aberration","armor_class":17,"hit_points":135,"hit_dice":"18d10+36","challenge_rating":"10","document__slug":"wotc-srd"}]}`)
		},
	})
	defer server.Close()

	c := NewClient(server.URL+"/", nil)
	res, err := c.SearchMonsters(context.Background(), SearchQuery{Search: "abol", DocumentSlug: "wotc-srd", Limit: 5, Offset: 10})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Equal(t, "aboleth", res.Results[0].Slug)
	assert.Equal(t, "Aboleth", res.Results[0].Name)
	assert.Contains(t, gotQuery, "search=abol")
	assert.Contains(t, gotQuery, "document__slug=wotc-srd")
	assert.Contains(t, gotQuery, "limit=5")
	assert.Contains(t, gotQuery, "offset=10")
}

func TestClient_SearchMonsters_HTTPErrorStatus(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	_, err := c.SearchMonsters(context.Background(), SearchQuery{})
	require.Error(t, err)
}

func TestClient_SearchMonsters_BadJSON(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "not json")
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	_, err := c.SearchMonsters(context.Background(), SearchQuery{})
	require.Error(t, err)
}

// --- Cycle 3: GetMonster ---

func TestClient_GetMonster_Success(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/aboleth/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"slug":"aboleth",
				"name":"Aboleth",
				"size":"Large",
				"type":"aberration",
				"alignment":"lawful evil",
				"armor_class":17,
				"armor_desc":"natural armor",
				"hit_points":135,
				"hit_dice":"18d10+36",
				"speed":{"walk":10,"swim":40},
				"strength":21,"dexterity":9,"constitution":15,
				"intelligence":18,"wisdom":15,"charisma":18,
				"strength_save":null,
				"challenge_rating":"10",
				"actions":[{"name":"Tentacle","desc":"Reach 10 ft."}],
				"document__slug":"wotc-srd"
			}`)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	m, err := c.GetMonster(context.Background(), "aboleth")
	require.NoError(t, err)
	assert.Equal(t, "aboleth", m.Slug)
	assert.Equal(t, "Aboleth", m.Name)
	assert.Equal(t, 17, m.ArmorClass)
	assert.Equal(t, "10", m.ChallengeRating)
	assert.Equal(t, "wotc-srd", m.DocumentSlug)
}

func TestClient_GetMonster_NotFound(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/nope/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	_, err := c.GetMonster(context.Background(), "nope")
	require.Error(t, err)
}

func TestClient_GetMonster_EmptySlug(t *testing.T) {
	c := NewClient("http://example.com/", nil)
	_, err := c.GetMonster(context.Background(), "")
	require.Error(t, err)
}

// --- Cycle 4: SearchSpells / GetSpell ---

func TestClient_SearchSpells_Success(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"fireball","name":"Fireball","level_int":3,"school":"evocation","document__slug":"wotc-srd"}]}`)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	res, err := c.SearchSpells(context.Background(), SearchQuery{Search: "fire"})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Equal(t, "fireball", res.Results[0].Slug)
	assert.Equal(t, 3, res.Results[0].LevelInt)
}

func TestClient_GetSpell_Success(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/fireball/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"slug":"fireball","name":"Fireball",
				"level_int":3,"school":"evocation",
				"casting_time":"1 action","range":"150 feet","duration":"Instantaneous",
				"components":"V, S, M","material":"a tiny ball of bat guano",
				"ritual":false,"concentration":false,
				"desc":"Boom.","higher_level":"Extra d6 per level.",
				"dnd_class":"Sorcerer, Wizard",
				"document__slug":"wotc-srd"
			}`)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	s, err := c.GetSpell(context.Background(), "fireball")
	require.NoError(t, err)
	assert.Equal(t, "fireball", s.Slug)
	assert.Equal(t, 3, s.LevelInt)
	assert.Equal(t, "evocation", s.School)
	assert.Equal(t, "wotc-srd", s.DocumentSlug)
}

func TestClient_GetSpell_EmptySlug(t *testing.T) {
	c := NewClient("http://example.com/", nil)
	_, err := c.GetSpell(context.Background(), "")
	require.Error(t, err)
}

func TestClient_SearchSpells_HTTPError(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	_, err := c.SearchSpells(context.Background(), SearchQuery{})
	require.Error(t, err)
}

// --- Cycle 5: request uses context cancellation ---

func TestClient_RequestRespectsContext(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":0,"results":[]}`)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.SearchMonsters(ctx, SearchQuery{})
	require.Error(t, err)
}

// --- Cycle 6: query parameters are omitted when empty ---

func TestClient_SearchMonsters_OmitsEmptyParams(t *testing.T) {
	var gotQuery string
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			_, _ = json.Marshal(nil)
			fmt.Fprint(w, `{"count":0,"results":[]}`)
		},
	})
	defer server.Close()
	c := NewClient(server.URL+"/", nil)
	_, err := c.SearchMonsters(context.Background(), SearchQuery{})
	require.NoError(t, err)
	assert.NotContains(t, gotQuery, "search=")
	assert.NotContains(t, gotQuery, "document__slug=")
	assert.NotContains(t, gotQuery, "limit=")
	assert.NotContains(t, gotQuery, "offset=")
}

// --- Cycle 7: baseURL without trailing slash still builds correct URL ---

func TestClient_BuildURL_HandlesBaseWithoutTrailingSlash(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":0,"results":[]}`)
		},
	})
	defer server.Close()
	c := NewClient(strings.TrimSuffix(server.URL, "/"), nil)
	_, err := c.SearchMonsters(context.Background(), SearchQuery{})
	require.NoError(t, err)
}
