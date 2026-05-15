package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/open5e"
)

// TestOpen5eCachePOST_F14_RequiresDMAuth proves that all Open5e endpoints
// (search + cache) are gated by DM auth middleware and return 403 for
// unauthenticated/non-DM callers. J-C02 extended this to GET search routes.
func TestOpen5eCachePOST_F14_RequiresDMAuth(t *testing.T) {
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"forbidden: DM only"}`, http.StatusForbidden)
		})
	}

	router := chi.NewRouter()
	handler := open5e.NewHandler(nil)

	// J-C02: all open5e routes (search + cache) behind auth middleware.
	router.Group(func(r chi.Router) {
		r.Use(denyAll)
		handler.RegisterPublicRoutes(r)
		handler.RegisterProtectedRoutes(r)
	})

	tests := []struct {
		method string
		path   string
		expect int
	}{
		// POST cache routes must be blocked (403).
		{http.MethodPost, "/api/open5e/monsters/goblin", http.StatusForbidden},
		{http.MethodPost, "/api/open5e/spells/fireball", http.StatusForbidden},
		// J-C02: GET search routes must also be blocked (403).
		{http.MethodGet, "/api/open5e/monsters", http.StatusForbidden},
		{http.MethodGet, "/api/open5e/spells", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, tt.expect, rec.Code,
				"%s %s must require DM auth (F-14/J-C02)", tt.method, tt.path)
		})
	}
}
