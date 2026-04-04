package portal_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes_NoOAuth_LoginReturns404(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware) // no WithOAuth

	req := httptest.NewRequest(http.MethodGet, "/portal/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Without OAuth routes, login should 404 or 405
	assert.True(t, rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed,
		"expected 404 or 405 without OAuth, got %d", rec.Code)
}
