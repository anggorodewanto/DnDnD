package portal_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
)

// errorRefDataStore returns errors for all methods.
type errorRefDataStore struct{}

func (e *errorRefDataStore) ListRaces(_ context.Context) ([]portal.RaceInfo, error) {
	return nil, errors.New("db error")
}
func (e *errorRefDataStore) ListClasses(_ context.Context) ([]portal.ClassInfo, error) {
	return nil, errors.New("db error")
}
func (e *errorRefDataStore) ListSpellsByClass(_ context.Context, _, _ string) ([]portal.SpellInfo, error) {
	return nil, errors.New("db error")
}
func (e *errorRefDataStore) ListEquipment(_ context.Context, _ string) ([]portal.EquipmentItem, error) {
	return nil, errors.New("db error")
}

func TestAPIHandler_ListRaces_Error(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &errorRefDataStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/races", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListRaces(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIHandler_ListClasses_Error(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &errorRefDataStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/classes", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListClasses(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIHandler_ListSpells_Error(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &errorRefDataStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells?class=wizard", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListSpells(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIHandler_ListSpells_EmptyResult(t *testing.T) {
	store := &mockRefDataStore{spells: nil}
	h := portal.NewAPIHandler(slog.Default(), store, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells?class=barbarian", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListSpells(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var result []portal.SpellInfo
	json.NewDecoder(rec.Body).Decode(&result)
	assert.Empty(t, result)
}

func TestAPIHandler_SubmitCharacter_StoreError(t *testing.T) {
	builderStore := &mockBuilderStore{createCharErr: errors.New("db error")}
	builderSvc := portal.NewBuilderService(builderStore)
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)

	body := `{"token":"t","campaign_id":"c","name":"Test","race":"elf","background":"sage","class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// SR-013: a submission from a player who already has an approved character
// returns 409 Conflict (not 500), so the builder can surface a clear "retire
// first" message instead of a generic error.
func TestAPIHandler_SubmitCharacter_AlreadyActive_Returns409(t *testing.T) {
	builderStore := &mockBuilderStore{
		charID:   "c-1",
		activePC: &portal.ActivePlayerCharacter{ID: "approved-pc", Status: "approved"},
	}
	builderSvc := portal.NewBuilderService(builderStore)
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)

	body := `{"token":"t","campaign_id":"c","name":"Test","race":"elf","background":"sage","class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

// A submission with empty token/campaign_id (e.g. the builder failed to read
// its hidden inputs) must return 400, not a generic 500 from a token-lookup
// miss or a rejected campaign_id insert.
func TestAPIHandler_SubmitCharacter_EmptyTokenAndCampaign_Returns400(t *testing.T) {
	builderStore := &mockBuilderStore{charID: "c-1"}
	builderSvc := portal.NewBuilderService(builderStore)
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)

	body := `{"token":"","campaign_id":"","name":"Test","race":"elf","background":"sage","class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A submit whose token has expired / been used / is unknown / belongs to
// another account must map to a client-error status with a "request a new
// link" message — never a bare 500 that loses the build with no next step
// (usability T10 / Finding 4·a). The error arrives wrapped ("validating
// token: ..." or "redeeming token: ..."), so the mapping must see through the
// chain with errors.Is.
func TestAPIHandler_SubmitCharacter_TokenErrors(t *testing.T) {
	const validBody = `{"token":"t","campaign_id":"c","name":"Test","race":"elf","background":"sage","class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`

	cases := []struct {
		name       string
		store      *mockBuilderStore
		wantStatus int
	}{
		{"expired", &mockBuilderStore{charID: "c-1", validateTokenErr: portal.ErrTokenExpired}, http.StatusGone},
		{"used at validate", &mockBuilderStore{charID: "c-1", validateTokenErr: portal.ErrTokenUsed}, http.StatusGone},
		{"used at redeem", &mockBuilderStore{charID: "c-1", redeemTokenErr: portal.ErrTokenUsed}, http.StatusGone},
		{"not found", &mockBuilderStore{charID: "c-1", validateTokenErr: portal.ErrTokenNotFound}, http.StatusNotFound},
		{"ownership mismatch", &mockBuilderStore{charID: "c-1", validateToken: &portal.PortalToken{DiscordUserID: "someone-else"}}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builderSvc := portal.NewBuilderService(tc.store)
			h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)

			req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(validBody))
			req.Header.Set("Content-Type", "application/json")
			ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			h.SubmitCharacter(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			assert.NotContains(t, rec.Body.String(), "internal server error")
			assert.Contains(t, rec.Body.String(), "/create-character")
		})
	}
}

func TestAPIHandler_SubmitCharacter_ShortStoreError(t *testing.T) {
	// Regression: isValidationError must not panic on short error messages.
	builderStore := &mockBuilderStore{createCharErr: errors.New("fail")}
	builderSvc := portal.NewBuilderService(builderStore)
	h := portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, builderSvc)

	body := `{"token":"t","campaign_id":"c","name":"Test","race":"elf","background":"sage","class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Must not panic
	h.SubmitCharacter(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIHandler_ListEquipment_Error(t *testing.T) {
	h := portal.NewAPIHandler(slog.Default(), &errorRefDataStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/equipment", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListEquipment(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIHandler_ListEquipment_EmptyResult(t *testing.T) {
	store := &mockRefDataStore{equipment: nil}
	h := portal.NewAPIHandler(slog.Default(), store, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/api/equipment", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "u1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListEquipment(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var result []portal.EquipmentItem
	json.NewDecoder(rec.Body).Decode(&result)
	assert.Empty(t, result)
}

func TestAPIHandler_NewAPIHandler_NilLogger(t *testing.T) {
	h := portal.NewAPIHandler(nil, &mockRefDataStore{}, nil)
	assert.NotNil(t, h)
}
