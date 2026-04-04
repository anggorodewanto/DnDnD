package portal_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// mockTokenValidator implements portal.TokenValidator for handler tests.
type mockTokenValidator struct {
	token *portal.PortalToken
	err   error
}

func (m *mockTokenValidator) ValidateToken(_ context.Context, _ string) (*portal.PortalToken, error) {
	return m.token, m.err
}

func TestHandler_ServeLanding_Authenticated(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeLanding(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "DnDnD")
}

func TestHandler_ServeLanding_Unauthenticated(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	rec := httptest.NewRecorder()

	h.ServeLanding(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_ServeCreate_ValidToken(t *testing.T) {
	validator := &mockTokenValidator{
		token: &portal.PortalToken{
			ID:            uuid.New(),
			Token:         "valid-token",
			CampaignID:    uuid.New(),
			DiscordUserID: "user-123",
			Purpose:       "create_character",
			Used:          false,
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		},
	}
	h := portal.NewHandler(slog.Default(), validator)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=valid-token", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Character Builder")
}

func TestHandler_ServeCreate_MissingToken(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/create", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing token")
}

func TestHandler_ServeCreate_ExpiredToken(t *testing.T) {
	validator := &mockTokenValidator{err: portal.ErrTokenExpired}
	h := portal.NewHandler(slog.Default(), validator)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=expired", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusGone, rec.Code)
	assert.Contains(t, rec.Body.String(), "expired")
}

func TestHandler_ServeCreate_UsedToken(t *testing.T) {
	validator := &mockTokenValidator{err: portal.ErrTokenUsed}
	h := portal.NewHandler(slog.Default(), validator)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=used", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusGone, rec.Code)
	assert.Contains(t, rec.Body.String(), "already been used")
}

func TestHandler_ServeCreate_NotFoundToken(t *testing.T) {
	validator := &mockTokenValidator{err: portal.ErrTokenNotFound}
	h := portal.NewHandler(slog.Default(), validator)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=nope", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not found")
}

func TestHandler_ServeCreate_InternalError(t *testing.T) {
	validator := &mockTokenValidator{err: errors.New("db error")}
	h := portal.NewHandler(slog.Default(), validator)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=broken", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ServeCreate_Unauthenticated(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
