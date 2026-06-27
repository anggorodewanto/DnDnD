package characteroverview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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

// Handler exposes the character overview HTTP API.
type Handler struct {
	svc              *Service
	campaignVerifier CampaignVerifier
}

// NewHandler constructs a character-overview HTTP handler.
func NewHandler(svc *Service, opts ...HandlerOption) *Handler {
	h := &Handler{svc: svc}
	for _, o := range opts {
		o(h)
	}
	return h
}

// RegisterRoutes mounts the character overview routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/character-overview", h.Get)
	r.Post("/api/character-overview/{characterID}/status", h.UpdateStatus)
}

// statusRequest is the JSON body for an out-of-combat status edit.
type statusRequest struct {
	HPMax           int32    `json:"hp_max"`
	HPCurrent       int32    `json:"hp_current"`
	TempHP          int32    `json:"temp_hp"`
	ExhaustionLevel int32    `json:"exhaustion_level"`
	Conditions      []string `json:"conditions"`
	Reason          string   `json:"reason"`
}

// UpdateStatus applies a DM's out-of-combat edit to a character's HP, temp HP,
// exhaustion and conditions. Refused (409) while the character is in an active
// combat; the in-combat override controls own status during combat.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	var req statusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sctx, err := h.svc.GetStatusContext(ctx, characterID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load character", http.StatusInternalServerError)
		return
	}

	if h.campaignVerifier != nil {
		userID, _ := auth.DiscordUserIDFromContext(ctx)
		owns, err := h.campaignVerifier.IsCampaignDM(ctx, userID, sctx.CampaignID.String())
		if err != nil || !owns {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	if sctx.InActiveCombat {
		http.Error(w, "character is in an active combat; use the in-combat controls", http.StatusConflict)
		return
	}

	status, err := h.svc.ApplyStatus(ctx, characterID, sctx.CharacterData, StatusUpdate{
		HPMax:           req.HPMax,
		HPCurrent:       req.HPCurrent,
		TempHP:          req.TempHP,
		ExhaustionLevel: req.ExhaustionLevel,
		Conditions:      req.Conditions,
		Reason:          req.Reason,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to update status", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// overviewResponse is the JSON envelope returned by Get.
type overviewResponse struct {
	Characters     []CharacterSheet   `json:"characters"`
	PartyLanguages []LanguageCoverage `json:"party_languages"`
}

// Get returns the approved party characters plus the Party Languages
// aggregation for the given campaign.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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

	sheets, err := h.svc.ListPartyCharacters(r.Context(), campaignID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to list party characters", http.StatusInternalServerError)
		return
	}
	if sheets == nil {
		sheets = []CharacterSheet{}
	}

	resp := overviewResponse{
		Characters:     sheets,
		PartyLanguages: h.svc.PartyLanguages(sheets),
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
