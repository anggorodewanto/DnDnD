package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeApprovalPage_ReturnsHTML(t *testing.T) {
	store := &mockApprovalStore{entries: []ApprovalEntry{}}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, &mockNotifier{}, hub, campaignID)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")

	body := rec.Body.String()
	require.Contains(t, body, "Character Approval Queue")
	require.Contains(t, body, "sidebar")
	require.Contains(t, body, "/dashboard/api/approvals")
}

func TestServeApprovalPage_RequiresAuth(t *testing.T) {
	store := &mockApprovalStore{}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, &mockNotifier{}, hub, campaignID)

	r := chi.NewRouter()
	// No auth middleware
	ah.RegisterApprovalRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServeApprovalPage_ContainsSidebarNav(t *testing.T) {
	store := &mockApprovalStore{entries: []ApprovalEntry{}}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, &mockNotifier{}, hub, campaignID)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, entry := range SidebarNav {
		assert.Contains(t, body, entry.Label, "sidebar should contain %q", entry.Label)
	}
}
