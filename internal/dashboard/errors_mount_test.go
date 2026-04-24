package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/errorlog"
)

func TestMountErrorsRoutes_WiresSidebarAndPanel(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	r := chi.NewRouter()
	h := NewHandler(nil, nil)

	MountErrorsRoutes(r, h, store, time.Now, mockAuthMiddleware)

	// Campaign home should now render the (count) via the shared reader.
	reqHome := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	reqHome = reqHome.WithContext(contextWithUser(reqHome.Context(), "dm-1"))
	recHome := httptest.NewRecorder()
	r.Get("/dashboard", h.ServeDashboard)
	r.ServeHTTP(recHome, reqHome)
	assert.Equal(t, http.StatusOK, recHome.Code)

	// Errors panel should be reachable.
	reqErr := httptest.NewRequest(http.MethodGet, "/dashboard/errors", nil)
	recErr := httptest.NewRecorder()
	r.ServeHTTP(recErr, reqErr)
	assert.Equal(t, http.StatusOK, recErr.Code)
}
