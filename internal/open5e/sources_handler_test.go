package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeSettingsStore is an in-memory CampaignSettingsStore for HTTP tests.
type fakeSettingsStore struct {
	getErr     error
	updateErr  error
	campaigns  map[uuid.UUID]refdata.Campaign
	lastUpdate *refdata.UpdateCampaignSettingsParams
}

func newFakeSettingsStore() *fakeSettingsStore {
	return &fakeSettingsStore{campaigns: make(map[uuid.UUID]refdata.Campaign)}
}

func (f *fakeSettingsStore) GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	if f.getErr != nil {
		return refdata.Campaign{}, f.getErr
	}
	c, ok := f.campaigns[id]
	if !ok {
		return refdata.Campaign{}, errors.New("not found")
	}
	return c, nil
}

func (f *fakeSettingsStore) UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
	if f.updateErr != nil {
		return refdata.Campaign{}, f.updateErr
	}
	cp := arg
	f.lastUpdate = &cp
	c := f.campaigns[arg.ID]
	c.ID = arg.ID
	c.Settings = arg.Settings
	f.campaigns[arg.ID] = c
	return c, nil
}

func newSourcesRouter(store CampaignSettingsStore) chi.Router {
	r := chi.NewRouter()
	NewSourcesHandler(store).RegisterRoutes(r)
	return r
}

// --- ListCatalog ---

func TestSourcesHandler_ListCatalog_ReturnsCuratedEntries(t *testing.T) {
	r := newSourcesRouter(newFakeSettingsStore())
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Sources []Source `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Sources)
	slugs := make([]string, 0, len(resp.Sources))
	for _, s := range resp.Sources {
		slugs = append(slugs, s.Slug)
	}
	assert.Contains(t, slugs, "tome-of-beasts")
	assert.Contains(t, slugs, "wotc-srd")
}

// --- GetCampaignSources ---

func TestSourcesHandler_GetCampaignSources_ReadsEnabled(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	settings := []byte(`{"turn_timeout_hours":24,"open5e_sources":["tome-of-beasts","deep-magic"]}`)
	store.campaigns[id] = refdata.Campaign{
		ID:       id,
		Settings: pqtype.NullRawMessage{RawMessage: settings, Valid: true},
	}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/campaigns/"+id.String()+"/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, id.String(), resp.CampaignID)
	assert.ElementsMatch(t, []string{"tome-of-beasts", "deep-magic"}, resp.Enabled)
}

func TestSourcesHandler_GetCampaignSources_NoSettings_ReturnsEmptyList(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{ID: id}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/campaigns/"+id.String()+"/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Enabled, "must serialize as [] not null")
	assert.Empty(t, resp.Enabled)
}

func TestSourcesHandler_GetCampaignSources_InvalidCampaignID(t *testing.T) {
	r := newSourcesRouter(newFakeSettingsStore())
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/campaigns/not-a-uuid/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSourcesHandler_GetCampaignSources_NotFound(t *testing.T) {
	r := newSourcesRouter(newFakeSettingsStore())
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/campaigns/"+uuid.New().String()+"/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- UpdateCampaignSources ---

func TestSourcesHandler_UpdateCampaignSources_HappyPath_PreservesOtherFields(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	settings := []byte(`{"turn_timeout_hours":12,"diagonal_rule":"variant","open5e_sources":["deep-magic"],"channel_ids":{"the-story":"123"}}`)
	store.campaigns[id] = refdata.Campaign{
		ID:       id,
		Settings: pqtype.NullRawMessage{RawMessage: settings, Valid: true},
	}
	r := newSourcesRouter(store)

	body := strings.NewReader(`{"enabled":["tome-of-beasts","creature-codex"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.ElementsMatch(t, []string{"tome-of-beasts", "creature-codex"}, resp.Enabled)

	// Verify the merged JSONB preserves unrelated fields.
	require.NotNil(t, store.lastUpdate)
	var merged map[string]any
	require.NoError(t, json.Unmarshal(store.lastUpdate.Settings.RawMessage, &merged))
	assert.EqualValues(t, 12, merged["turn_timeout_hours"])
	assert.Equal(t, "variant", merged["diagonal_rule"])
	require.IsType(t, map[string]any{}, merged["channel_ids"])
	assert.Equal(t, "123", merged["channel_ids"].(map[string]any)["the-story"])
	require.IsType(t, []any{}, merged["open5e_sources"])
	got := merged["open5e_sources"].([]any)
	assert.Len(t, got, 2)
}

func TestSourcesHandler_UpdateCampaignSources_EmptyListDisablesAll(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{
		ID: id,
		Settings: pqtype.NullRawMessage{
			RawMessage: []byte(`{"turn_timeout_hours":24,"open5e_sources":["tome-of-beasts"]}`),
			Valid:      true,
		},
	}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", strings.NewReader(`{"enabled":[]}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Enabled)
	// Settings still serialises the empty list (canonical wire shape).
	var merged map[string]any
	require.NoError(t, json.Unmarshal(store.lastUpdate.Settings.RawMessage, &merged))
	require.Contains(t, merged, "open5e_sources")
	assert.Empty(t, merged["open5e_sources"])
}

func TestSourcesHandler_UpdateCampaignSources_RejectsUnknownSlug(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{ID: id}
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"enabled":["definitely-not-a-real-doc"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "unknown open5e source")
	assert.Nil(t, store.lastUpdate, "no write should happen when validation fails")
}

func TestSourcesHandler_UpdateCampaignSources_RejectsMalformedJSON(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{ID: id}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSourcesHandler_UpdateCampaignSources_DedupesAndIgnoresBlanks(t *testing.T) {
	store := newFakeSettingsStore()
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{ID: id}
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"enabled":["tome-of-beasts","","tome-of-beasts","deep-magic"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []string{"tome-of-beasts", "deep-magic"}, resp.Enabled)
}

func TestSourcesHandler_UpdateCampaignSources_NotFound(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+uuid.New().String()+"/sources", strings.NewReader(`{"enabled":["tome-of-beasts"]}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
