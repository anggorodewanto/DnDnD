package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/refdata"
)

type stubCampaignStore struct {
	campaigns []refdata.Campaign
	created   []refdata.CreateCampaignParams
	activated []refdata.SetActiveCampaignParams
	createErr error
	listErr   error
	activeErr error
}

func (s *stubCampaignStore) SetActiveCampaign(_ context.Context, arg refdata.SetActiveCampaignParams) error {
	if s.activeErr != nil {
		return s.activeErr
	}
	s.activated = append(s.activated, arg)
	return nil
}

func (s *stubCampaignStore) CreateCampaign(_ context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
	s.created = append(s.created, arg)
	if s.createErr != nil {
		return refdata.Campaign{}, s.createErr
	}
	return refdata.Campaign{
		ID:        uuid.New(),
		GuildID:   arg.GuildID,
		DmUserID:  arg.DmUserID,
		Name:      arg.Name,
		Settings:  arg.Settings,
		Status:    "active",
		CreatedAt: time.Date(2026, 5, 23, 1, 2, 3, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 23, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (s *stubCampaignStore) ListCampaigns(_ context.Context) ([]refdata.Campaign, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.campaigns, nil
}

func TestCampaignsHandler_ListFiltersToAuthenticatedDM(t *testing.T) {
	store := &stubCampaignStore{campaigns: []refdata.Campaign{
		{ID: uuid.New(), GuildID: "guild-1", DmUserID: "dm-1", Name: "Mine", Status: "active"},
		{ID: uuid.New(), GuildID: "guild-2", DmUserID: "dm-2", Name: "Theirs", Status: "active"},
	}}
	handler := NewCampaignsHandler(nil, store)

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp campaignsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Campaigns) != 1 {
		t.Fatalf("campaign count = %d, want 1", len(resp.Campaigns))
	}
	if resp.Campaigns[0].Name != "Mine" {
		t.Fatalf("campaign name = %q, want Mine", resp.Campaigns[0].Name)
	}
}

func TestCampaignsHandler_CreateUsesAuthenticatedDM(t *testing.T) {
	store := &stubCampaignStore{}
	handler := NewCampaignsHandler(nil, store)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(`{"guild_id":"guild-local","name":"Local Campaign"}`))
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-local"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if len(store.created) != 1 {
		t.Fatalf("created calls = %d, want 1", len(store.created))
	}
	got := store.created[0]
	if got.GuildID != "guild-local" || got.DmUserID != "dm-local" || got.Name != "Local Campaign" {
		t.Fatalf("created params = %+v", got)
	}
	if !got.Settings.Valid || len(got.Settings.RawMessage) == 0 {
		t.Fatalf("expected default settings JSON, got %+v", got.Settings)
	}

	var settings map[string]any
	if err := json.Unmarshal(got.Settings.RawMessage, &settings); err != nil {
		t.Fatalf("settings JSON invalid: %v", err)
	}
	if settings["diagonal_rule"] != "standard" {
		t.Fatalf("diagonal_rule = %v, want standard", settings["diagonal_rule"])
	}
}

func TestCampaignsHandler_CreateWarnsInPassthroughMode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := &stubCampaignStore{}
	handler := NewCampaignsHandler(logger, store)
	handler.Passthrough = true

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(`{"guild_id":"guild-local","name":"Local Campaign"}`))
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "local-dev"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(buf.String()), "passthrough") {
		t.Fatalf("expected a passthrough ownership warning, got logs: %q", buf.String())
	}
}

func TestCampaignsHandler_CreateNoWarnWithOAuth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := &stubCampaignStore{}
	handler := NewCampaignsHandler(logger, store) // Passthrough defaults to false

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(`{"guild_id":"guild-1","name":"Real Campaign"}`))
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(buf.String()), "passthrough") {
		t.Fatalf("did not expect a passthrough warning with OAuth, got: %q", buf.String())
	}
}

func TestCampaignsHandler_CreateRequiresNameAndGuild(t *testing.T) {
	handler := NewCampaignsHandler(nil, &stubCampaignStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(`{"guild_id":"","name":""}`))
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-local"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRegisterCampaignsRoutes_BehindAuthMiddleware(t *testing.T) {
	r := chi.NewRouter()
	store := &stubCampaignStore{}
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.ContextWithDiscordUserID(r.Context(), "dm-1")))
		})
	}
	RegisterCampaignsRoutes(r, NewCampaignsHandler(nil, store), mw)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/campaigns", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestResolveActiveCampaign_HonorsPreferredWhenValid(t *testing.T) {
	newer := uuid.New()
	older := uuid.New()
	campaigns := []refdata.Campaign{ // ListCampaigns order: created_at DESC
		{ID: newer, DmUserID: "dm-1", Status: "active"},
		{ID: older, DmUserID: "dm-1", Status: "active"},
	}

	id, status := ResolveActiveCampaign(campaigns, "dm-1", older)
	if id != older.String() {
		t.Fatalf("expected preferred (older) campaign %s, got %s", older, id)
	}
	if status != "active" {
		t.Fatalf("status = %q, want active", status)
	}
}

func TestResolveActiveCampaign_FallsBackToMostRecentWhenNoPreference(t *testing.T) {
	newer := uuid.New()
	older := uuid.New()
	campaigns := []refdata.Campaign{
		{ID: newer, DmUserID: "dm-1", Status: "active"},
		{ID: older, DmUserID: "dm-1", Status: "active"},
	}

	id, _ := ResolveActiveCampaign(campaigns, "dm-1", uuid.Nil)
	if id != newer.String() {
		t.Fatalf("expected most-recent campaign %s, got %s", newer, id)
	}
}

func TestResolveActiveCampaign_IgnoresArchivedOrUnownedPreference(t *testing.T) {
	archived := uuid.New()
	fallback := uuid.New()
	other := uuid.New()
	campaigns := []refdata.Campaign{
		{ID: fallback, DmUserID: "dm-1", Status: "active"},
		{ID: archived, DmUserID: "dm-1", Status: "archived"},
		{ID: other, DmUserID: "dm-2", Status: "active"},
	}

	// Preferred is archived -> fall back to dm-1's most-recent non-archived.
	if id, _ := ResolveActiveCampaign(campaigns, "dm-1", archived); id != fallback.String() {
		t.Fatalf("archived preference: expected fallback %s, got %s", fallback, id)
	}
	// Preferred belongs to another DM -> ignored, fall back.
	if id, _ := ResolveActiveCampaign(campaigns, "dm-1", other); id != fallback.String() {
		t.Fatalf("unowned preference: expected fallback %s, got %s", fallback, id)
	}
}

func setActiveRequest(t *testing.T, handler *CampaignsHandler, userID, campaignID string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID)))
		})
	}
	RegisterCampaignsRoutes(r, handler, mw)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/campaigns/"+campaignID+"/set-active", nil))
	return rec
}

func TestCampaignsHandler_SetActivePersistsOwnedCampaign(t *testing.T) {
	target := uuid.New()
	store := &stubCampaignStore{campaigns: []refdata.Campaign{
		{ID: target, DmUserID: "dm-1", Status: "active"},
	}}
	handler := NewCampaignsHandler(nil, store)

	rec := setActiveRequest(t, handler, "dm-1", target.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(store.activated) != 1 {
		t.Fatalf("SetActiveCampaign calls = %d, want 1", len(store.activated))
	}
	if store.activated[0].DmUserID != "dm-1" || store.activated[0].ActiveCampaignID != target {
		t.Fatalf("activated params = %+v", store.activated[0])
	}
}

func TestCampaignsHandler_SetActiveRejectsUnownedCampaign(t *testing.T) {
	target := uuid.New()
	store := &stubCampaignStore{campaigns: []refdata.Campaign{
		{ID: target, DmUserID: "dm-2", Status: "active"}, // owned by someone else
	}}
	handler := NewCampaignsHandler(nil, store)

	rec := setActiveRequest(t, handler, "dm-1", target.String())

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if len(store.activated) != 0 {
		t.Fatalf("expected no SetActiveCampaign call, got %d", len(store.activated))
	}
}

func TestCampaignsHandler_SetActiveRejectsArchivedCampaign(t *testing.T) {
	target := uuid.New()
	store := &stubCampaignStore{campaigns: []refdata.Campaign{
		{ID: target, DmUserID: "dm-1", Status: "archived"},
	}}
	handler := NewCampaignsHandler(nil, store)

	rec := setActiveRequest(t, handler, "dm-1", target.String())

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if len(store.activated) != 0 {
		t.Fatalf("expected no SetActiveCampaign call, got %d", len(store.activated))
	}
}
