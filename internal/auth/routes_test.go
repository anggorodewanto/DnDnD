package auth_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes(t *testing.T) {
	tr := &mockTokenRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	r := chi.NewRouter()
	auth.RegisterRoutes(r, svc)

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/auth/login", http.StatusTemporaryRedirect},
		// callback without params returns 400 (missing state cookie)
		{http.MethodGet, "/auth/callback", http.StatusBadRequest},
		// logout without cookie still redirects
		{http.MethodPost, "/auth/logout", http.StatusTemporaryRedirect},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestRegisterRoutes_MethodNotAllowed(t *testing.T) {
	tr := &mockTokenRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	r := chi.NewRouter()
	auth.RegisterRoutes(r, svc)

	// POST to /auth/login should be 405
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
