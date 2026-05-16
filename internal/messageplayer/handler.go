package messageplayer

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

// Handler exposes the message-player HTTP API on the dashboard router.
type Handler struct {
	svc              *Service
	campaignVerifier CampaignVerifier
}

// NewHandler constructs a Handler wrapping the given service.
func NewHandler(svc *Service, opts ...HandlerOption) *Handler {
	h := &Handler{svc: svc}
	for _, o := range opts {
		o(h)
	}
	return h
}

// RegisterRoutes mounts the message-player routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/message-player", func(r chi.Router) {
		r.Post("/", h.Send)
		r.Get("/history", h.History)
	})
}

// sendRequest is the JSON payload for POST /api/message-player.
type sendRequest struct {
	CampaignID        string `json:"campaign_id"`
	PlayerCharacterID string `json:"player_character_id"`
	AuthorUserID      string `json:"author_user_id"`
	Body              string `json:"body"`
}

// Send handles DM-initiated messages to a specific player.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	playerCharacterID, err := uuid.Parse(req.PlayerCharacterID)
	if err != nil {
		http.Error(w, "invalid player_character_id", http.StatusBadRequest)
		return
	}

	authorUserID, _ := auth.DiscordUserIDFromContext(r.Context())

	msg, err := h.svc.SendMessage(r.Context(), SendMessageInput{
		CampaignID:        campaignID,
		PlayerCharacterID: playerCharacterID,
		AuthorUserID:      authorUserID,
		Body:              req.Body,
	})
	if err != nil {
		status := statusForError(err)
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// History handles GET /api/message-player/history.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id required", http.StatusBadRequest)
		return
	}
	playerCharacterIDStr := r.URL.Query().Get("player_character_id")
	if playerCharacterIDStr == "" {
		http.Error(w, "player_character_id required", http.StatusBadRequest)
		return
	}
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	playerCharacterID, err := uuid.Parse(playerCharacterIDStr)
	if err != nil {
		http.Error(w, "invalid player_character_id", http.StatusBadRequest)
		return
	}

	if h.campaignVerifier != nil {
		userID, _ := auth.DiscordUserIDFromContext(r.Context())
		owns, verErr := h.campaignVerifier.IsCampaignDM(r.Context(), userID, campaignID.String())
		if verErr != nil || !owns {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 20)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)

	msgs, err := h.svc.History(r.Context(), campaignID, playerCharacterID, limit, offset)
	if err != nil {
		http.Error(w, "failed to list history", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// statusForError maps domain errors to HTTP statuses.
func statusForError(err error) int {
	if errors.Is(err, ErrInvalidInput) {
		return http.StatusBadRequest
	}
	if errors.Is(err, ErrPlayerNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, ErrMessengerUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
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
