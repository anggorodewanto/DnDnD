package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeSettingsStore is an in-memory SourceStore for HTTP tests.
type fakeSettingsStore struct {
	getErr     error
	updateErr  error
	campaigns  map[uuid.UUID]refdata.Campaign
	lastUpdate *refdata.UpdateCampaignSettingsParams

	// custom-source backing store.
	custom        map[string]refdata.Open5eCustomSource
	listCustomErr error
	upsertErr     error
	deleteErr     error
}

func newFakeSettingsStore() *fakeSettingsStore {
	return &fakeSettingsStore{
		campaigns: make(map[uuid.UUID]refdata.Campaign),
		custom:    make(map[string]refdata.Open5eCustomSource),
	}
}

func (f *fakeSettingsStore) ListOpen5eCustomSources(ctx context.Context) ([]refdata.Open5eCustomSource, error) {
	if f.listCustomErr != nil {
		return nil, f.listCustomErr
	}
	out := make([]refdata.Open5eCustomSource, 0, len(f.custom))
	for _, c := range f.custom {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

func (f *fakeSettingsStore) UpsertOpen5eCustomSource(ctx context.Context, arg refdata.UpsertOpen5eCustomSourceParams) (refdata.Open5eCustomSource, error) {
	if f.upsertErr != nil {
		return refdata.Open5eCustomSource{}, f.upsertErr
	}
	rec := refdata.Open5eCustomSource{
		Slug:        arg.Slug,
		Title:       arg.Title,
		Publisher:   arg.Publisher,
		Description: arg.Description,
	}
	f.custom[arg.Slug] = rec
	return rec, nil
}

func (f *fakeSettingsStore) DeleteOpen5eCustomSource(ctx context.Context, slug string) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	if _, ok := f.custom[slug]; !ok {
		return 0, nil
	}
	delete(f.custom, slug)
	return 1, nil
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

func newSourcesRouter(store SourceStore) chi.Router {
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

// --- ListCatalog: built-in ∪ custom ---

func TestSourcesHandler_ListCatalog_MergesCustomSourcesAndFlagsBuiltin(t *testing.T) {
	store := newFakeSettingsStore()
	store.custom["warlock-grimoire"] = refdata.Open5eCustomSource{
		Slug: "warlock-grimoire", Title: "Warlock Grimoire", Publisher: "Kort'thalis",
	}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Sources []Source `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	bySlug := make(map[string]Source, len(resp.Sources))
	for _, s := range resp.Sources {
		bySlug[s.Slug] = s
	}
	// Built-in present and flagged.
	builtin, ok := bySlug["tome-of-beasts"]
	require.True(t, ok)
	assert.True(t, builtin.Builtin)
	// Custom present and not flagged.
	custom, ok := bySlug["warlock-grimoire"]
	require.True(t, ok, "custom source must appear in the merged catalog")
	assert.False(t, custom.Builtin)
	assert.Equal(t, "Warlock Grimoire", custom.Title)
}

func TestSourcesHandler_ListCatalog_StoreError_Returns500(t *testing.T) {
	store := newFakeSettingsStore()
	store.listCustomErr = errors.New("db down")
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/open5e/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- AddCustomSource ---

func TestSourcesHandler_AddCustomSource_HappyPath(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"slug":"warlock-grimoire","title":"Warlock Grimoire","publisher":"Kort'thalis","description":"Eldritch lore."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	var got Source
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "warlock-grimoire", got.Slug)
	assert.Equal(t, "Warlock Grimoire", got.Title)
	assert.False(t, got.Builtin)
	// Persisted to the store.
	_, ok := store.custom["warlock-grimoire"]
	assert.True(t, ok)
}

func TestSourcesHandler_AddCustomSource_TrimsAndPersistsTrimmed(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"slug":"  deep-cuts  ","title":"  Deep Cuts  "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	stored, ok := store.custom["deep-cuts"]
	require.True(t, ok, "slug must be stored trimmed")
	assert.Equal(t, "Deep Cuts", stored.Title)
}

func TestSourcesHandler_AddCustomSource_RejectsBadSlug(t *testing.T) {
	for _, bad := range []string{"", "Tome Of Beasts", "UPPER", "trailing-", "-leading", "double--hyphen", "under_score"} {
		store := newFakeSettingsStore()
		r := newSourcesRouter(store)
		body := strings.NewReader(`{"slug":"` + bad + `","title":"X"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", body)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "slug %q should be rejected", bad)
		assert.Empty(t, store.custom, "nothing should be stored for bad slug %q", bad)
	}
}

func TestSourcesHandler_AddCustomSource_RejectsMissingTitle(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"slug":"valid-slug","title":"   "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "title is required")
}

func TestSourcesHandler_AddCustomSource_RejectsBuiltinCollision(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	body := strings.NewReader(`{"slug":"tome-of-beasts","title":"Hijack"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Empty(t, store.custom)
}

func TestSourcesHandler_AddCustomSource_RejectsMalformedJSON(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodPost, "/api/open5e/sources", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- DeleteCustomSource ---

func TestSourcesHandler_DeleteCustomSource_HappyPath(t *testing.T) {
	store := newFakeSettingsStore()
	store.custom["warlock-grimoire"] = refdata.Open5eCustomSource{Slug: "warlock-grimoire", Title: "Warlock Grimoire"}
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/open5e/sources/warlock-grimoire", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, ok := store.custom["warlock-grimoire"]
	assert.False(t, ok, "custom source must be removed")
}

func TestSourcesHandler_DeleteCustomSource_BuiltinProtected(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/open5e/sources/tome-of-beasts", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSourcesHandler_DeleteCustomSource_NotFound(t *testing.T) {
	store := newFakeSettingsStore()
	r := newSourcesRouter(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/open5e/sources/never-added", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- enabling a custom source per campaign ---

func TestSourcesHandler_UpdateCampaignSources_AcceptsCustomSlug(t *testing.T) {
	store := newFakeSettingsStore()
	store.custom["warlock-grimoire"] = refdata.Open5eCustomSource{Slug: "warlock-grimoire", Title: "Warlock Grimoire"}
	id := uuid.New()
	store.campaigns[id] = refdata.Campaign{ID: id}
	r := newSourcesRouter(store)

	body := strings.NewReader(`{"enabled":["tome-of-beasts","warlock-grimoire"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/open5e/campaigns/"+id.String()+"/sources", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var resp campaignSourcesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.ElementsMatch(t, []string{"tome-of-beasts", "warlock-grimoire"}, resp.Enabled)
}
