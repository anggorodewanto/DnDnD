package characteroverview

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the character overview HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler constructs a character-overview HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the character overview routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/character-overview", h.Get)
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
