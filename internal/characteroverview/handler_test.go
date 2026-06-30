package characteroverview

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

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/character"
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

// --- GetFeatureUses: read-only feature_uses for editor prefill (ISSUE-039) ---

func TestGetFeatureUses_HappyPath(t *testing.T) {
	raw := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	h := newTestHandler(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New(), FeatureUses: raw}})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+uuid.New().String()+"/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		FeatureUses map[string]character.FeatureUse `json:"feature_uses"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rage, ok := resp.FeatureUses["rage"]
	if !ok || rage.Current != 1 || rage.Max != 3 || rage.Recharge != "long" {
		t.Fatalf("rage = %+v (ok=%v)", rage, ok)
	}
}

func TestGetFeatureUses_EmptyWhenNoFeatures(t *testing.T) {
	// A character with no limited-use features (nil feature_uses) must render
	// an empty object, not null, so the editor seeds cleanly.
	h := newTestHandler(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New()}})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+uuid.New().String()+"/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"feature_uses":{}`) {
		t.Fatalf("expected empty feature_uses object, got %s", body)
	}
}

func TestGetFeatureUses_InvalidCharacterID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/not-uuid/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGetFeatureUses_NotFound(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: ErrCharacterNotFound})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+uuid.New().String()+"/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGetFeatureUses_LoadError(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: errors.New("db down")})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+uuid.New().String()+"/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGetFeatureUses_Forbidden(t *testing.T) {
	// Verifier owns a different campaign than the character's -> 403.
	verifier := &fakeCampaignVerifier{ownedCampaign: uuid.New().String()}
	h := NewHandler(NewService(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New()}}), WithCampaignVerifier(verifier))
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+uuid.New().String()+"/feature-uses", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

// --- UpdateFeatureUses: out-of-combat feature_uses editor (ISSUE-040) ---

func postFeatureUses(t *testing.T, h *Handler, charID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/character-overview/"+charID+"/feature-uses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestUpdateFeatureUses_HappyPath(t *testing.T) {
	raw := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	store := &fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New(), FeatureUses: raw}}
	h := newTestHandler(store)

	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}],"reason":"used one rage"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		FeatureUses map[string]character.FeatureUse `json:"feature_uses"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rage := resp.FeatureUses["rage"]; rage.Current != 2 || rage.Max != 3 {
		t.Fatalf("rage = %+v", rage)
	}
	if store.persistedFeatureUses == nil {
		t.Fatal("expected persistence")
	}
}

func TestUpdateFeatureUses_InvalidCharacterID(t *testing.T) {
	rr := postFeatureUses(t, newTestHandler(&fakeStore{}), "not-uuid", `{"changes":[]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_InvalidBody(t *testing.T) {
	rr := postFeatureUses(t, newTestHandler(&fakeStore{}), uuid.New().String(), `{not json`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_MissingFeatureInChange(t *testing.T) {
	rr := postFeatureUses(t, newTestHandler(&fakeStore{}), uuid.New().String(), `{"changes":[{"current":2}]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_MissingCurrentInChange(t *testing.T) {
	rr := postFeatureUses(t, newTestHandler(&fakeStore{}), uuid.New().String(), `{"changes":[{"feature":"rage"}]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_NotFound(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: ErrCharacterNotFound})
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}]}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_LoadError(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: errors.New("db down")})
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}]}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_Forbidden(t *testing.T) {
	verifier := &fakeCampaignVerifier{ownedCampaign: uuid.New().String()}
	h := NewHandler(NewService(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New()}}), WithCampaignVerifier(verifier))
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}]}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestUpdateFeatureUses_ConflictWhenInCombat(t *testing.T) {
	raw := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	h := newTestHandler(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New(), FeatureUses: raw, InActiveCombat: true}})
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}]}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdateFeatureUses_ValidationError400(t *testing.T) {
	// Unknown feature -> service returns ErrInvalidInput -> 400.
	raw := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	h := newTestHandler(&fakeStore{slotsCtx: SlotsContext{CampaignID: uuid.New(), FeatureUses: raw}})
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"ki","current":1}]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdateFeatureUses_PersistError500(t *testing.T) {
	raw := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	h := newTestHandler(&fakeStore{
		slotsCtx:              SlotsContext{CampaignID: uuid.New(), FeatureUses: raw},
		persistFeatureUsesErr: errors.New("db down"),
	})
	rr := postFeatureUses(t, h, uuid.New().String(), `{"changes":[{"feature":"rage","current":2}]}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

// silence unused import
var _ = context.Background
