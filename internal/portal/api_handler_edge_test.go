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
