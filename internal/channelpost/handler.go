package channelpost

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the channel-post HTTP API over the dashboard router. It is
// mounted behind the DM-only middleware (mountDMOnlyAPIs), so no per-handler
// auth is needed — same posture as the narration handler.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler around the channel-post service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the channel-post API routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/channel", func(r chi.Router) {
		r.Post("/post", h.Post)
		r.Get("/list", h.List)
	})
}

// postRequest is the JSON payload for POST /api/channel/post.
type postRequest struct {
	CampaignID string `json:"campaign_id"`
	Channel    string `json:"channel"`
	Body       string `json:"body"`
}

// Post broadcasts the body to the named campaign channel as the bot.
func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	res, err := h.svc.Post(r.Context(), PostInput{
		CampaignID: campaignID,
		Channel:    req.Channel,
		Body:       req.Body,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrUnknownChannel) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// List returns the campaign's configured channel name keys for the dropdown.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id required", http.StatusBadRequest)
		return
	}
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	channels, err := h.svc.Channels(r.Context(), campaignID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to list channels", http.StatusInternalServerError)
		return
	}
	if channels == nil {
		channels = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": channels})
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
