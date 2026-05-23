package dashboard

import (
	"context"
	"encoding/json"
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
	createErr error
	listErr   error
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
