package levelup

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CampaignOwnershipChecker resolves the DM user ID for a character's campaign.
type CampaignOwnershipChecker interface {
	GetCampaignDMUserID(ctx context.Context, characterID uuid.UUID) (string, error)
}

// LevelUpRequest is the JSON body for the level-up API.
type LevelUpRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	ClassID     string    `json:"class_id"`
	NewLevel    int       `json:"new_level"`
}

// LevelUpResponse is the JSON response after a level-up.
type LevelUpResponse struct {
	NewLevel            int  `json:"new_level"`
	HPGained            int  `json:"hp_gained"`
	NewHPMax            int  `json:"new_hp_max"`
	NewProficiencyBonus int  `json:"new_proficiency_bonus"`
	NewAttacksPerAction int  `json:"new_attacks_per_action"`
	GrantsASI           bool `json:"grants_asi"`
	NeedsSubclass       bool `json:"needs_subclass"`
}

// ASIApprovalRequest is the JSON body for approving an ASI choice.
type ASIApprovalRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Choice      ASIChoice `json:"choice"`
}

// ASIDenyRequest is the JSON body for denying an ASI choice.
type ASIDenyRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Reason      string    `json:"reason"`
}

// FeatApplyRequest is the JSON body for applying a feat.
type FeatApplyRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Feat        FeatInfo  `json:"feat"`
}

// FeatPrereqCheckRequest is the JSON body for checking feat prerequisites.
type FeatPrereqCheckRequest struct {
	Prerequisites      FeatPrerequisites       `json:"prerequisites"`
	Scores             character.AbilityScores `json:"scores"`
	ArmorProficiencies []string                `json:"armor_proficiencies"`
	IsSpellcaster      bool                    `json:"is_spellcaster"`
}

// FeatPrereqCheckResponse is the JSON response for a feat prerequisite check.
type FeatPrereqCheckResponse struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
}

// Handler serves the level-up API endpoints.
type Handler struct {
	service          *Service
	logger           *slog.Logger
	hub              *dashboard.Hub
	ownershipChecker CampaignOwnershipChecker
}

// NewHandler creates a new level-up Handler.
func NewHandler(service *Service, hub *dashboard.Hub) *Handler {
	return &Handler{
		service: service,
		logger:  slog.Default(),
		hub:     hub,
	}
}

// SetOwnershipChecker wires a CampaignOwnershipChecker onto the handler.
func (h *Handler) SetOwnershipChecker(oc CampaignOwnershipChecker) {
	h.ownershipChecker = oc
}

// RegisterRoutes mounts level-up API routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/levelup", func(r chi.Router) {
		r.Post("/", h.HandleLevelUp)
		r.Post("/asi/approve", h.HandleApproveASI)
		r.Post("/asi/deny", h.HandleDenyASI)
		r.Post("/feat/apply", h.HandleApplyFeat)
		r.Post("/feat/check", h.HandleCheckFeatPrereqs)
	})
}

// HandleLevelUp processes a level-up request from the dashboard.
func (h *Handler) HandleLevelUp(w http.ResponseWriter, r *http.Request) {
	var req LevelUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CharacterID == uuid.Nil || req.ClassID == "" || req.NewLevel < 1 || req.NewLevel > 20 {
		http.Error(w, "character_id, class_id, and new_level (1-20) are required", http.StatusBadRequest)
		return
	}

	details, err := h.service.ApplyLevelUp(r.Context(), req.CharacterID, req.ClassID, req.NewLevel)
	if err != nil {
		h.logger.Error("level-up failed", "error", err)
		http.Error(w, "level-up failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, LevelUpResponse{
		NewLevel:            details.NewLevel,
		HPGained:            details.HPGained,
		NewHPMax:            details.NewHPMax,
		NewProficiencyBonus: details.NewProficiencyBonus,
		NewAttacksPerAction: details.NewAttacksPerAction,
		GrantsASI:           details.GrantsASI,
		NeedsSubclass:       details.NeedsSubclass,
	})
}

// HandleApproveASI processes an ASI approval from the DM.
func (h *Handler) HandleApproveASI(w http.ResponseWriter, r *http.Request) {
	var req ASIApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if h.ownershipChecker != nil {
		userID, ok := auth.DiscordUserIDFromContext(r.Context())
		if !ok || userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		dmUserID, err := h.ownershipChecker.GetCampaignDMUserID(r.Context(), req.CharacterID)
		if err != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		if dmUserID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	if err := h.service.ApproveASI(r.Context(), req.CharacterID, req.Choice); err != nil {
		h.logger.Error("ASI approval failed", "error", err)
		http.Error(w, "ASI approval failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// HandleDenyASI processes an ASI denial from the DM.
func (h *Handler) HandleDenyASI(w http.ResponseWriter, r *http.Request) {
	var req ASIDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.DenyASI(r.Context(), req.CharacterID, req.Reason); err != nil {
		h.logger.Error("ASI denial failed", "error", err)
		http.Error(w, "ASI denial failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "denied"})
}

// HandleApplyFeat processes a feat application request.
func (h *Handler) HandleApplyFeat(w http.ResponseWriter, r *http.Request) {
	var req FeatApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ApplyFeat(r.Context(), req.CharacterID, req.Feat); err != nil {
		h.logger.Error("feat application failed", "error", err)
		if errors.Is(err, ErrInvalidFeatChoices) {
			http.Error(w, "feat application failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "feat application failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

// HandleCheckFeatPrereqs checks whether a character meets feat prerequisites.
func (h *Handler) HandleCheckFeatPrereqs(w http.ResponseWriter, r *http.Request) {
	var req FeatPrereqCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	eligible, reason := CheckFeatPrerequisites(req.Prerequisites, req.Scores, req.ArmorProficiencies, req.IsSpellcaster)

	writeJSON(w, http.StatusOK, FeatPrereqCheckResponse{
		Eligible: eligible,
		Reason:   reason,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
