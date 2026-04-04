package portal_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
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
