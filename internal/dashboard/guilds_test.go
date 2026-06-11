package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
)

type stubGuildLister struct {
	guilds []Guild
	err    error
}

func (s stubGuildLister) ListGuilds(_ context.Context) ([]Guild, error) {
	return s.guilds, s.err
}

func guildsRequest(t *testing.T, handler *GuildsHandler) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1")))
		})
	}
	RegisterGuildsRoute(r, handler, mw)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/guilds", nil))
	return rec
}

func TestGuildsHandler_ReturnsBotGuilds(t *testing.T) {
	handler := NewGuildsHandler(nil, stubGuildLister{guilds: []Guild{
		{ID: "g1", Name: "Tavern"},
		{ID: "g2", Name: "Dungeon"},
	}})

	rec := guildsRequest(t, handler)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp guildsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Guilds) != 2 || resp.Guilds[0].Name != "Tavern" {
		t.Fatalf("guilds = %+v", resp.Guilds)
	}
}

func TestGuildsHandler_NilListerReturnsEmptyList(t *testing.T) {
	rec := guildsRequest(t, NewGuildsHandler(nil, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp guildsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Guilds == nil || len(resp.Guilds) != 0 {
		t.Fatalf("expected empty (non-null) guild list, got %+v", resp.Guilds)
	}
}

func TestGuildsHandler_ListErrorDegradesToEmptyList(t *testing.T) {
	handler := NewGuildsHandler(nil, stubGuildLister{err: errors.New("state unavailable")})

	rec := guildsRequest(t, handler)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (degraded)", rec.Code)
	}
	var resp guildsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Guilds) != 0 {
		t.Fatalf("expected empty guild list on error, got %+v", resp.Guilds)
	}
}
