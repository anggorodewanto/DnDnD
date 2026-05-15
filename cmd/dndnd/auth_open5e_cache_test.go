package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/open5e"
)

// TestOpen5eCachePOST_F14_RequiresDMAuth proves that the POST cache
// endpoints (CacheMonster, CacheSpell) are gated by DM auth middleware
// and return 403 for unauthenticated/non-DM callers.
func TestOpen5eCachePOST_F14_RequiresDMAuth(t *testing.T) {
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"forbidden: DM only"}`, http.StatusForbidden)
		})
	}

	router := chi.NewRouter()
	// Recover panics from nil service on GET routes so we can assert
	// they are reachable (not blocked by middleware).
	router.Use(recoverAsInternalError)
	handler := open5e.NewHandler(nil)

	// Public GET routes — no middleware.
	handler.RegisterPublicRoutes(router)

	// Protected POST routes — behind deny-all middleware.
	router.Group(func(r chi.Router) {
		r.Use(denyAll)
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
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, tt.expect, rec.Code,
				"%s %s must require DM auth (F-14)", tt.method, tt.path)
		})
	}

	// GET search routes must NOT be blocked by DM auth.
	getRoutes := []string{
		"/api/open5e/monsters?search=goblin",
		"/api/open5e/spells?search=fireball",
	}
	for _, path := range getRoutes {
		t.Run("GET_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"GET %s must remain public (F-14)", path)
			assert.NotEqual(t, http.StatusNotFound, rec.Code,
				"GET %s must be registered (F-14)", path)
		})
	}
}
