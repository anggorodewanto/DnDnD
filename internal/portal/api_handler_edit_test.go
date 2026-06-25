package portal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEditAPIHandler(store portal.BuilderStore) *portal.APIHandler {
	return portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, portal.NewBuilderService(store))
}

func withEditCtx(req *http.Request, characterID, userID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", characterID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if userID != "" {
		ctx = auth.ContextWithDiscordUserID(ctx, userID)
	}
	return req.WithContext(ctx)
}

func editGet(characterID, userID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/portal/api/characters/"+characterID+"/edit-data", nil)
	return withEditCtx(req, characterID, userID)
}

func editPut(characterID, userID string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/portal/api/characters/"+characterID, bytes.NewReader(body))
	return withEditCtx(req, characterID, userID)
}

func ownerEditStore(sub portal.CharacterSubmission) *mockBuilderStore {
	return &mockBuilderStore{
		editContext:    &portal.EditContext{CampaignID: "c1", OwnerID: "owner-1", DMUserID: "dm-1", PlayerCharacterID: "pc-1"},
		editSubmission: sub,
	}
}

func TestAPIHandler_GetCharacterEditData_OK(t *testing.T) {
	store := ownerEditStore(portal.CharacterSubmission{Name: "Thorin", Race: "dwarf"})
	h := newEditAPIHandler(store)

	rec := httptest.NewRecorder()
	h.GetCharacterEditData(rec, editGet("char-1", "owner-1"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got portal.CharacterSubmission
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Thorin", got.Name)
}

func TestAPIHandler_GetCharacterEditData_Forbidden(t *testing.T) {
	h := newEditAPIHandler(ownerEditStore(portal.CharacterSubmission{Name: "Thorin"}))

	rec := httptest.NewRecorder()
	h.GetCharacterEditData(rec, editGet("char-1", "stranger"))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAPIHandler_GetCharacterEditData_NotFound(t *testing.T) {
	h := newEditAPIHandler(&mockBuilderStore{editContextErr: portal.ErrCharacterNotFound})

	rec := httptest.NewRecorder()
	h.GetCharacterEditData(rec, editGet("missing", "owner-1"))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIHandler_GetCharacterEditData_Unauthenticated(t *testing.T) {
	h := newEditAPIHandler(ownerEditStore(portal.CharacterSubmission{}))

	rec := httptest.NewRecorder()
	h.GetCharacterEditData(rec, editGet("char-1", ""))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIHandler_UpdateCharacter_OK(t *testing.T) {
	store := ownerEditStore(portal.CharacterSubmission{})
	h := newEditAPIHandler(store)
	body, _ := json.Marshal(validSubmission())

	rec := httptest.NewRecorder()
	h.UpdateCharacter(rec, editPut("char-1", "owner-1", body))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, store.updateCalled)
	assert.True(t, store.setPendingCalled, "a player edit reverts to pending")
}

func TestAPIHandler_UpdateCharacter_InEncounter(t *testing.T) {
	store := ownerEditStore(portal.CharacterSubmission{})
	store.hasActiveEncounter = true
	h := newEditAPIHandler(store)
	body, _ := json.Marshal(validSubmission())

	rec := httptest.NewRecorder()
	h.UpdateCharacter(rec, editPut("char-1", "owner-1", body))

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestAPIHandler_UpdateCharacter_Forbidden(t *testing.T) {
	store := ownerEditStore(portal.CharacterSubmission{})
	h := newEditAPIHandler(store)
	body, _ := json.Marshal(validSubmission())

	rec := httptest.NewRecorder()
	h.UpdateCharacter(rec, editPut("char-1", "stranger", body))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
