package characteroverview

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
)

// deleteChar issues DELETE /api/character-overview/{characterID} against the
// handler, optionally with an authed Discord user id in context.
func deleteChar(h *Handler, characterID, userID string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/api/character-overview/"+characterID, nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestHandler_Delete_Success(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: uuid.New()}}
	h := newTestHandler(store)

	rr := deleteChar(h, id.String(), "")

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.deletedID == nil || *store.deletedID != id {
		t.Fatalf("expected DeleteCharacter(%s), got %v", id, store.deletedID)
	}
}

func TestHandler_Delete_InvalidCharacterID(t *testing.T) {
	store := &fakeStore{}
	rr := deleteChar(newTestHandler(store), "not-a-uuid", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if store.deletedID != nil {
		t.Fatal("must not delete on a bad id")
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	store := &fakeStore{statusCtxErr: ErrCharacterNotFound}
	rr := deleteChar(newTestHandler(store), uuid.New().String(), "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
	if store.deletedID != nil {
		t.Fatal("must not delete a missing character")
	}
}

func TestHandler_Delete_ConflictWhenInCombat(t *testing.T) {
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: uuid.New(), InActiveCombat: true}}
	rr := deleteChar(newTestHandler(store), uuid.New().String(), "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if store.deletedID != nil {
		t.Fatal("must not delete a character in active combat")
	}
}

func TestHandler_Delete_ForbiddenWhenNotCampaignDM(t *testing.T) {
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: uuid.New()}}
	verifier := &fakeCampaignVerifier{ownedCampaign: uuid.New().String()} // owns a different campaign
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))

	rr := deleteChar(h, uuid.New().String(), "dm-1")

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if store.deletedID != nil {
		t.Fatal("must not delete when caller is not the campaign DM")
	}
}

func TestHandler_Delete_AllowedWhenCampaignDM(t *testing.T) {
	owned := uuid.New()
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: owned}}
	verifier := &fakeCampaignVerifier{ownedCampaign: owned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))

	rr := deleteChar(h, uuid.New().String(), "dm-1")

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.deletedID == nil {
		t.Fatal("expected delete for the owning DM")
	}
}

func TestHandler_Delete_StoreError(t *testing.T) {
	store := &fakeStore{
		statusCtx: CharacterStatusContext{CampaignID: uuid.New()},
		deleteErr: errors.New("boom"),
	}
	rr := deleteChar(newTestHandler(store), uuid.New().String(), "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}
