package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPassthroughMiddleware_ForwardsRequestUnchanged verifies the no-op
// middleware used by Phase 104b to mount inventory API routes without the
// global auth stack (matching Phase 104's combat handler wiring).
func TestPassthroughMiddleware_ForwardsRequestUnchanged(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := passthroughMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	require.True(t, called, "passthroughMiddleware must call the inner handler")
	assert.Equal(t, http.StatusTeapot, w.Code)
}
