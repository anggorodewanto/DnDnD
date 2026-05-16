package characteroverview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
)

func newTestHandler(store Store) *Handler {
	return NewHandler(NewService(store))
}

func TestHandler_Get_Success(t *testing.T) {
	campID := uuid.New()
	sheets := []CharacterSheet{
		{Name: "Aria", Languages: []string{"Common", "Elvish"}},
		{Name: "Fenwick", Languages: []string{"Elvish", "Dwarvish"}},
	}
	h := newTestHandler(&fakeStore{sheets: sheets})

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+campID.String(), nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Characters     []CharacterSheet   `json:"characters"`
		PartyLanguages []LanguageCoverage `json:"party_languages"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Characters) != 2 {
		t.Fatalf("chars = %+v", resp.Characters)
	}
	if len(resp.PartyLanguages) != 3 {
		t.Fatalf("langs = %+v", resp.PartyLanguages)
	}
}

func TestHandler_Get_MissingCampaignID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Get_InvalidUUID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Get_StoreError(t *testing.T) {
	h := newTestHandler(&fakeStore{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Get_EmptyCharactersReturnedAsEmptyArray(t *testing.T) {
	h := newTestHandler(&fakeStore{sheets: nil})
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp struct {
		Characters     []CharacterSheet   `json:"characters"`
		PartyLanguages []LanguageCoverage `json:"party_languages"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Characters == nil {
		t.Fatal("expected empty array, got nil")
	}
	if resp.PartyLanguages == nil {
		t.Fatal("expected empty array, got nil")
	}
}

func TestHandler_RegisterRoutes_Mounts(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Get_ForbiddenWhenNotCampaignDM(t *testing.T) {
	campA := uuid.New() // DM owns this
	campB := uuid.New() // DM does NOT own this

	verifier := &fakeCampaignVerifier{ownedCampaign: campA.String()}
	h := NewHandler(NewService(&fakeStore{sheets: []CharacterSheet{{Name: "X"}}}), WithCampaignVerifier(verifier))

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+campB.String(), nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Get_AllowedWhenCampaignDM(t *testing.T) {
	campA := uuid.New()

	verifier := &fakeCampaignVerifier{ownedCampaign: campA.String()}
	h := NewHandler(NewService(&fakeStore{sheets: []CharacterSheet{{Name: "X"}}}), WithCampaignVerifier(verifier))

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview?campaign_id="+campA.String(), nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

type fakeCampaignVerifier struct {
	ownedCampaign string
}

func (f *fakeCampaignVerifier) IsCampaignDM(_ context.Context, _, campaignID string) (bool, error) {
	return f.ownedCampaign == campaignID, nil
}

// silence unused import
var _ = context.Background
