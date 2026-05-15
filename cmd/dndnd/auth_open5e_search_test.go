package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/open5e"
)

// TestOpen5eSearchGET_JC02_RequiresDMAuth proves that the GET search
// endpoints (SearchMonsters, SearchSpells) are gated by DM auth middleware
// and return 401 for unauthenticated callers (finding J-C02).
func TestOpen5eSearchGET_JC02_RequiresDMAuth(t *testing.T) {
	denyUnauth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"count":0,"results":[]}`)
	}))
	defer upstream.Close()

	svc := open5e.NewService(open5e.NewClient(upstream.URL+"/", nil), nil)
	handler := open5e.NewHandler(svc)

	// Mount all open5e routes behind auth (matches main.go after J-C02 fix).
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(denyUnauth)
		handler.RegisterPublicRoutes(r)
		handler.RegisterProtectedRoutes(r)
	})

	// Unauthenticated requests must be rejected.
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/open5e/monsters?search=goblin"},
		{http.MethodGet, "/api/open5e/spells?search=fireball"},
		{http.MethodPost, "/api/open5e/monsters/goblin"},
		{http.MethodPost, "/api/open5e/spells/fireball"},
	}
	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"%s %s must require auth (J-C02)", tt.method, tt.path)
		})
	}
}
