package portal_test

import (
	"fmt"
	"html/template"
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

func TestNewHandler_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	h := portal.NewHandler(nil, nil)
	assert.NotNil(t, h)
}

func TestHandler_ServeLanding_EmptyUserID(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeLanding(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_ServeCreate_EmptyUserID(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=abc", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// brokenTemplate creates a template that always fails on Execute.
func brokenTemplate(name string) *template.Template {
	// A template that calls a function which doesn't exist triggers execute error
	return template.Must(template.New(name).Funcs(template.FuncMap{
		"fail": func() (string, error) { return "", fmt.Errorf("forced template error") },
	}).Parse(`{{fail}}`))
}

func TestHandler_ServeLanding_TemplateError(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)
	h.SetLandingTemplate(brokenTemplate("landing"))

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeLanding(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ServeCreate_TemplateError(t *testing.T) {
	validator := &mockTokenValidator{
		token: &portal.PortalToken{
			Token:         "valid-token",
			CampaignID:    uuid.New(),
			DiscordUserID: "user-123",
			Purpose:       "create_character",
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		},
	}
	h := portal.NewHandler(slog.Default(), validator)
	h.SetCreateTemplate(brokenTemplate("create"))

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=valid-token", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_RenderError_TemplateError(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)
	h.SetErrorTemplate(brokenTemplate("error"))

	// Trigger renderError by sending a request with missing token
	req := httptest.NewRequest(http.MethodGet, "/portal/create", nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.ServeCreate(rec, req)

	// When error template fails, it falls back to http.Error with 500
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
