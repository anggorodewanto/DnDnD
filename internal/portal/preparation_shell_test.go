package portal_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestServePrepare_RendersShell(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+testCharID+"/prepare", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", testCharID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.ContextWithDiscordUserID(ctx, "user-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ServePrepare(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body := rec.Body.String()
	assert.Contains(t, body, `id="character-builder"`)
	assert.Contains(t, body, `id="prep-character-id"`)
	assert.Contains(t, body, testCharID)
	// Loads the same hashed bundle as the create page: a hashed JS module and CSS link.
	assert.Contains(t, body, "/portal/app/assets/index-")
	assert.Contains(t, body, `<script type="module"`)
	assert.Contains(t, body, `<link rel="stylesheet"`)
}

func TestServePrepare_Unauthenticated(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+testCharID+"/prepare", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", testCharID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ServePrepare(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterRoutes_SpellPreparation(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)

	svc := &mockPrepareService{}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	prepH := portal.NewPreparationHandler(slog.Default(), svc, store, &mockRefDataStore{})

	portal.RegisterRoutes(r, h, fakeAuthMiddleware, portal.WithSpellPreparation(prepH))

	// HTML shell route is mounted.
	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+testCharID+"/prepare", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `id="prep-character-id"`)
}
