package homebrew

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// newTestRouter wires a Handler over a fake-store-backed Service.
func newTestRouter(store *fakeStore) chi.Router {
	svc := newSvc(store)
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// do issues an HTTP request to the handler and returns the recorder.
func do(t *testing.T, r chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// =============================================================================
// CREATURES (handler)
// =============================================================================

func TestHandler_CreateCreature_Success(t *testing.T) {
	store := newFakeStore()
	r := newTestRouter(store)
	cid := uuid.New()
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+cid.String(),
		map[string]any{"name": "Goblin"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Goblin", resp.Name)
	assert.True(t, resp.Homebrew.Bool)
}

func TestHandler_CreateCreature_MissingCampaign(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures", map[string]any{"name": "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCreature_InvalidCampaign(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id=not-a-uuid", map[string]any{"name": "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCreature_MalformedJSON(t *testing.T) {
	r := newTestRouter(newFakeStore())
	req := httptest.NewRequest(http.MethodPost, "/api/homebrew/creatures?campaign_id="+uuid.NewString(),
		strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCreature_UnknownField(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+uuid.NewString(),
		map[string]any{"name": "x", "extra_field": 1})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCreature_EmptyName(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+uuid.NewString(),
		map[string]any{"name": ""})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCreature_StoreError(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	r := newTestRouter(store)
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+uuid.NewString(),
		map[string]any{"name": "x"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_UpdateCreature_Success(t *testing.T) {
	store := newFakeStore()
	r := newTestRouter(store)
	cid := uuid.New()
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+cid.String(),
		map[string]any{"name": "Old"})
	require.Equal(t, http.StatusCreated, rec.Code)
	var created refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec2 := do(t, r, http.MethodPut, "/api/homebrew/creatures/"+created.ID+"?campaign_id="+cid.String(),
		map[string]any{"name": "New"})
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestHandler_UpdateCreature_NotFound(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPut, "/api/homebrew/creatures/missing?campaign_id="+uuid.NewString(),
		map[string]any{"name": "x"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_UpdateCreature_MalformedJSON(t *testing.T) {
	r := newTestRouter(newFakeStore())
	req := httptest.NewRequest(http.MethodPut, "/api/homebrew/creatures/abc?campaign_id="+uuid.NewString(),
		strings.NewReader("{"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateCreature_MissingCampaign(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodPut, "/api/homebrew/creatures/abc", map[string]any{"name": "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeleteCreature_Success(t *testing.T) {
	store := newFakeStore()
	r := newTestRouter(store)
	cid := uuid.New()
	rec := do(t, r, http.MethodPost, "/api/homebrew/creatures?campaign_id="+cid.String(),
		map[string]any{"name": "x"})
	require.Equal(t, http.StatusCreated, rec.Code)
	var created refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec2 := do(t, r, http.MethodDelete, "/api/homebrew/creatures/"+created.ID+"?campaign_id="+cid.String(), nil)
	assert.Equal(t, http.StatusNoContent, rec2.Code)
}

func TestHandler_DeleteCreature_NotFound(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodDelete, "/api/homebrew/creatures/missing?campaign_id="+uuid.NewString(), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_DeleteCreature_MissingCampaign(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodDelete, "/api/homebrew/creatures/abc", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// =============================================================================
// Smoke tests for the other six types — exercise every handler at least once
// to ensure routing wires up.
// =============================================================================

type typeRoute struct {
	name string
	path string // collection path, e.g. "/api/homebrew/spells"
}

var allRoutes = []typeRoute{
	{"spells", "/api/homebrew/spells"},
	{"weapons", "/api/homebrew/weapons"},
	{"magic-items", "/api/homebrew/magic-items"},
	{"races", "/api/homebrew/races"},
	{"feats", "/api/homebrew/feats"},
	{"classes", "/api/homebrew/classes"},
}

func TestHandler_AllOtherTypes_CRUD(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			r := newTestRouter(store)
			cid := uuid.New()

			// Create
			rec := do(t, r, http.MethodPost, tc.path+"?campaign_id="+cid.String(),
				bodyForType(tc.name, "Original"))
			require.Equal(t, http.StatusCreated, rec.Code, "create %s body=%s", tc.name, rec.Body.String())
			var created struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
			require.NotEmpty(t, created.ID)

			// Update
			rec2 := do(t, r, http.MethodPut, tc.path+"/"+created.ID+"?campaign_id="+cid.String(),
				bodyForType(tc.name, "Updated"))
			require.Equal(t, http.StatusOK, rec2.Code, "update %s body=%s", tc.name, rec2.Body.String())

			// Delete
			rec3 := do(t, r, http.MethodDelete, tc.path+"/"+created.ID+"?campaign_id="+cid.String(), nil)
			require.Equal(t, http.StatusNoContent, rec3.Code)
		})
	}
}

// bodyForType returns the minimal valid body for the given homebrew type.
func bodyForType(name, label string) map[string]any {
	switch name {
	case "spells":
		return map[string]any{"name": label, "level": 1, "school": "evocation",
			"casting_time": "1 action", "range_type": "self", "components": []string{"V"},
			"duration": "instant", "description": "x", "resolution_mode": "dm_required",
			"classes": []string{"wizard"}}
	case "weapons":
		return map[string]any{"name": label, "damage": "1d6", "damage_type": "slashing", "weapon_type": "simple-melee"}
	case "magic-items":
		return map[string]any{"name": label, "rarity": "rare", "description": "x"}
	case "races":
		return map[string]any{"name": label, "speed_ft": 30, "size": "Medium",
			"ability_bonuses": map[string]int{"str": 1}, "traits": []any{}}
	case "feats":
		return map[string]any{"name": label, "description": "x"}
	case "classes":
		return map[string]any{"name": label, "hit_die": "1d10", "primary_ability": "STR",
			"save_proficiencies": []string{"STR"},
			"features_by_level":  map[string]any{}, "attacks_per_action": map[string]any{},
			"subclass_level": 3, "subclasses": []any{}}
	}
	return nil
}

func TestHandler_AllOtherTypes_BadCampaignID(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name+"_create", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			rec := do(t, r, http.MethodPost, tc.path, bodyForType(tc.name, "x"))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
		t.Run(tc.name+"_update", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			rec := do(t, r, http.MethodPut, tc.path+"/abc", bodyForType(tc.name, "x"))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
		t.Run(tc.name+"_delete", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			rec := do(t, r, http.MethodDelete, tc.path+"/abc", nil)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestHandler_AllOtherTypes_MalformedJSON(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name+"_create", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			req := httptest.NewRequest(http.MethodPost, tc.path+"?campaign_id="+uuid.NewString(),
				strings.NewReader("{"))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
		t.Run(tc.name+"_update", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			req := httptest.NewRequest(http.MethodPut, tc.path+"/abc?campaign_id="+uuid.NewString(),
				strings.NewReader("{"))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestHandler_AllOtherTypes_NotFound(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name+"_update", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			rec := do(t, r, http.MethodPut, tc.path+"/missing?campaign_id="+uuid.NewString(),
				bodyForType(tc.name, "x"))
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
		t.Run(tc.name+"_delete", func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			rec := do(t, r, http.MethodDelete, tc.path+"/missing?campaign_id="+uuid.NewString(), nil)
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestHandler_AllOtherTypes_StoreError(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name+"_create", func(t *testing.T) {
			store := newFakeStore()
			store.upsertErr = errors.New("boom")
			r := newTestRouter(store)
			rec := do(t, r, http.MethodPost, tc.path+"?campaign_id="+uuid.NewString(),
				bodyForType(tc.name, "x"))
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

func TestHandler_DecodeBodyNilBody(t *testing.T) {
	// Cover the missing-body branch by directly calling decodeBody with a
	// request whose Body is nil.
	req := &http.Request{Body: nil}
	var dst struct{}
	err := decodeBody(req, &dst)
	require.Error(t, err)
}

func TestHandler_EmptyNameValidation_AllTypes(t *testing.T) {
	for _, tc := range allRoutes {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRouter(newFakeStore())
			body := bodyForType(tc.name, "")
			rec := do(t, r, http.MethodPost, tc.path+"?campaign_id="+uuid.NewString(), body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
