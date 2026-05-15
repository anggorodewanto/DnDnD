package campaign

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the campaign HTTP API used by the DM dashboard.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler wrapping the campaign service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the campaign API routes on the given router.
//
// Phase 102 introduces pause/resume endpoints for the mobile-lite Quick
// Actions tab. The service methods and transitions are implemented in
// service.go; the handler only covers request parsing and status mapping.
//
// The optional campaignDMMw middleware is applied inside the {id} route group
// to enforce campaign-scoped DM authorization (F-01).
func (h *Handler) RegisterRoutes(r chi.Router, campaignDMMw ...func(http.Handler) http.Handler) {
	r.Route("/api/campaigns/{id}", func(r chi.Router) {
		for _, mw := range campaignDMMw {
			r.Use(mw)
		}
		r.Post("/pause", h.Pause)
		r.Post("/resume", h.Resume)
	})
}

// Pause transitions a campaign to paused status.
func (h *Handler) Pause(w http.ResponseWriter, r *http.Request) {
	id, ok := parseCampaignID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.PauseCampaign(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// Resume transitions a paused campaign back to active.
func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	id, ok := parseCampaignID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.ResumeCampaign(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func parseCampaignID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
