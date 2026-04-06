package statblocklibrary

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func newTestRouter(store Store) chi.Router {
	svc := NewService(store)
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestHandler_List_Success(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
	}}
	r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/statblocks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "Bat", resp[0].Name)
}

func TestHandler_List_WithAllFilters(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("hobgoblin", "Hobgoblin", "humanoid", "Medium", "1/2"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
		mkHomebrew("hb", "Homebrew Goblin", "humanoid", "Small", "1/4", campaign),
	}}
	r := newTestRouter(store)

	url := "/api/statblocks?campaign_id=" + campaign.String() +
		"&search=goblin&type=humanoid&cr_min=0.25&cr_max=1&size=Small&source=&limit=10&offset=0"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	names := []string{resp[0].Name, resp[1].Name}
	assert.ElementsMatch(t, []string{"Goblin", "Homebrew Goblin"}, names)
}

func TestHandler_List_MultipleTypesAndSizes(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
		mkCreature("ghoul", "Ghoul", "undead", "Medium", "1"),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?type=beast&type=undead&size=Tiny&size=Medium", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
}

func TestHandler_List_InvalidCampaignID(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?campaign_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidCRMin(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?cr_min=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidCRMax(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?cr_max=xyz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidLimit(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?limit=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidOffset(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?offset=xyz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidSource(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?source=bogus", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_StoreError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Get_Success(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
	}}
	r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/goblin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Goblin", resp.Name)
}

func TestHandler_Get_NotFound(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/nope", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Get_HomebrewOtherCampaignForbidden(t *testing.T) {
	other := uuid.New()
	mine := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkHomebrew("hb", "Other Homebrew", "humanoid", "Small", "1/4", other),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/hb?campaign_id="+mine.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Get_HomebrewForOwnCampaign(t *testing.T) {
	mine := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkHomebrew("hb", "My Homebrew", "humanoid", "Small", "1/4", mine),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/hb?campaign_id="+mine.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Get_InvalidCampaignID(t *testing.T) {
	r := newTestRouter(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/goblin?campaign_id=bad", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Get_StoreError(t *testing.T) {
	store := &fakeStore{getErr: errors.New("db down")}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks/goblin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_List_SourceSRD(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD", "humanoid", "Small", "1/4"),
		mkHomebrew("hb", "HB", "humanoid", "Small", "1/4", campaign),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?source=srd&campaign_id="+campaign.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "SRD", resp[0].Name)
}

func TestHandler_List_SourceHomebrew(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD", "humanoid", "Small", "1/4"),
		mkHomebrew("hb", "HB", "humanoid", "Small", "1/4", campaign),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?source=homebrew&campaign_id="+campaign.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "HB", resp[0].Name)
}

func TestHandler_List_CommaSeparatedTypes(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "undead", "Medium", "1"),
		mkCreature("c", "C", "humanoid", "Small", "1/4"),
	}}
	r := newTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/statblocks?type=beast,undead", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []refdata.Creature
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
}

// compile-time check
var _ = context.Background
