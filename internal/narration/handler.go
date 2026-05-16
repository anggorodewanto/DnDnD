package narration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
)

// CampaignVerifier checks whether a user owns a specific campaign.
type CampaignVerifier interface {
	IsCampaignDM(ctx context.Context, discordUserID, campaignID string) (bool, error)
}

// HandlerOption configures optional Handler dependencies.
type HandlerOption func(*Handler)

// WithCampaignVerifier injects a campaign ownership verifier.
func WithCampaignVerifier(v CampaignVerifier) HandlerOption {
	return func(h *Handler) { h.campaignVerifier = v }
}

// Handler exposes the narration HTTP API over the dashboard router.
type Handler struct {
	svc              *Service
	campaignVerifier CampaignVerifier
}

// NewHandler constructs a Handler around the narration service.
func NewHandler(svc *Service, opts ...HandlerOption) *Handler {
	h := &Handler{svc: svc}
	for _, o := range opts {
		o(h)
	}
	return h
}

// RegisterRoutes mounts narration API routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/narration", func(r chi.Router) {
		r.Post("/preview", h.Preview)
		r.Post("/post", h.Post)
		r.Get("/history", h.History)
	})
}

// previewRequest is the JSON payload for the /preview endpoint.
type previewRequest struct {
	Body string `json:"body"`
}

// Preview renders the editor source to the Discord payload that would be
// posted, without sending anything. Intended for live preview in the UI.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, RenderDiscord(req.Body))
}

// postRequest is the JSON payload for the /post endpoint.
type postRequest struct {
	CampaignID         string   `json:"campaign_id"`
	AuthorUserID       string   `json:"author_user_id"`
	Body               string   `json:"body"`
	AttachmentAssetIDs []string `json:"attachment_asset_ids"`
}

// Post composes and sends a narration, returning the stored post row.
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

	attachmentIDs, err := parseUUIDs(req.AttachmentAssetIDs)
	if err != nil {
		http.Error(w, "invalid attachment_asset_ids", http.StatusBadRequest)
		return
	}

	post, err := h.svc.Post(r.Context(), PostInput{
		CampaignID:         campaignID,
		AuthorUserID:       req.AuthorUserID,
		Body:               req.Body,
		AttachmentAssetIDs: attachmentIDs,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrAttachmentNotFound) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

// History returns recent posts for a campaign, newest-first.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
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

	if h.campaignVerifier != nil {
		userID, _ := auth.DiscordUserIDFromContext(r.Context())
		owns, err := h.campaignVerifier.IsCampaignDM(r.Context(), userID, campaignID.String())
		if err != nil || !owns {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 20)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)

	posts, err := h.svc.History(r.Context(), campaignID, limit, offset)
	if err != nil {
		http.Error(w, "failed to list history", http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, http.StatusOK, posts)
}

// parseUUIDs parses a slice of stringified UUIDs.
func parseUUIDs(in []string) ([]uuid.UUID, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]uuid.UUID, 0, len(in))
	for _, s := range in {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// parseIntDefault parses s as an int, returning def on empty/invalid input.
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
