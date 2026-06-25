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

func TestServeEdit_RendersBuilderInEditMode(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+testCharID+"/edit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", testCharID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.ContextWithDiscordUserID(ctx, "user-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ServeEdit(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="character-builder"`)
	// The edit-mode flag carries the target character ID so the Svelte builder
	// switches to edit mode and prefills.
	assert.Contains(t, body, `id="edit-character-id"`)
	assert.Contains(t, body, testCharID)
}

func TestServeEdit_Unauthenticated(t *testing.T) {
	h := portal.NewHandler(slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+testCharID+"/edit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", testCharID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.ServeEdit(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
