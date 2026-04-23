package portal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ab/dndnd/internal/auth"
)

// RaceInfo is the API response for a race.
type RaceInfo struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	SpeedFt        int             `json:"speed_ft"`
	Size           string          `json:"size,omitempty"`
	AbilityBonuses json.RawMessage `json:"ability_bonuses,omitempty"`
	Languages      []string        `json:"languages,omitempty"`
	Traits         json.RawMessage `json:"traits,omitempty"`
	Subraces       json.RawMessage `json:"subraces,omitempty"`
}

// ClassInfo is the API response for a class.
type ClassInfo struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	HitDie              string          `json:"hit_die"`
	PrimaryAbility      string          `json:"primary_ability,omitempty"`
	SaveProficiencies   []string        `json:"save_proficiencies,omitempty"`
	ArmorProficiencies  []string        `json:"armor_proficiencies,omitempty"`
	WeaponProficiencies []string        `json:"weapon_proficiencies,omitempty"`
	SkillChoices        json.RawMessage `json:"skill_choices,omitempty"`
	Spellcasting        json.RawMessage `json:"spellcasting,omitempty"`
	Subclasses          json.RawMessage `json:"subclasses,omitempty"`
	SubclassLevel       int             `json:"subclass_level,omitempty"`
}

// EquipmentItem is the API response for a weapon or armor item.
type EquipmentItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"` // "weapon" or "armor"
	WeaponType string   `json:"weapon_type,omitempty"`
	Damage     string   `json:"damage,omitempty"`
	DamageType string   `json:"damage_type,omitempty"`
	Properties []string `json:"properties,omitempty"`
	ArmorType  string   `json:"armor_type,omitempty"`
	ACBase     int      `json:"ac_base,omitempty"`
}

// SpellInfo is the API response for a spell.
type SpellInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Level       int      `json:"level"`
	School      string   `json:"school"`
	CastingTime string   `json:"casting_time,omitempty"`
	Duration    string   `json:"duration,omitempty"`
	Description string   `json:"description,omitempty"`
	Classes     []string `json:"classes"`
}

// RefDataStore provides reference data for the API handler.
//
// ListSpellsByClass receives the campaign id so the store can apply
// per-campaign gating (Open5e sources). An empty campaignID means "no
// campaign context" and the store applies the safe default (hide all
// non-SRD / non-homebrew rows).
type RefDataStore interface {
	ListRaces(ctx context.Context) ([]RaceInfo, error)
	ListClasses(ctx context.Context) ([]ClassInfo, error)
	ListSpellsByClass(ctx context.Context, class, campaignID string) ([]SpellInfo, error)
	ListEquipment(ctx context.Context) ([]EquipmentItem, error)
}

// APIHandler serves the portal JSON API endpoints.
type APIHandler struct {
	logger     *slog.Logger
	refData    RefDataStore
	builderSvc *BuilderService
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(logger *slog.Logger, refData RefDataStore, builderSvc *BuilderService) *APIHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &APIHandler{
		logger:     logger,
		refData:    refData,
		builderSvc: builderSvc,
	}
}

// ListRaces returns all races as JSON.
func (h *APIHandler) ListRaces(w http.ResponseWriter, r *http.Request) {
	races, err := h.refData.ListRaces(r.Context())
	if err != nil {
		h.logger.Error("listing races", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, races)
}

// ListClasses returns all classes as JSON.
func (h *APIHandler) ListClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := h.refData.ListClasses(r.Context())
	if err != nil {
		h.logger.Error("listing classes", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, classes)
}

// ListSpells returns spells filtered by class as JSON.
func (h *APIHandler) ListSpells(w http.ResponseWriter, r *http.Request) {
	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class query parameter is required", http.StatusBadRequest)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	spells, err := h.refData.ListSpellsByClass(r.Context(), class, campaignID)
	if err != nil {
		h.logger.Error("listing spells", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if spells == nil {
		spells = []SpellInfo{}
	}
	writeJSON(w, http.StatusOK, spells)
}

// ListEquipment returns all weapons and armor as JSON.
func (h *APIHandler) ListEquipment(w http.ResponseWriter, r *http.Request) {
	items, err := h.refData.ListEquipment(r.Context())
	if err != nil {
		h.logger.Error("listing equipment", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []EquipmentItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

// GetStartingEquipment returns starting equipment packs for a class.
func (h *APIHandler) GetStartingEquipment(w http.ResponseWriter, r *http.Request) {
	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class query parameter is required", http.StatusBadRequest)
		return
	}
	packs := StartingEquipmentPacks(class)
	if packs == nil {
		packs = []EquipmentPack{}
	}
	writeJSON(w, http.StatusOK, packs)
}

// submitRequest is the JSON body for character submission.
type submitRequest struct {
	Token      string `json:"token"`
	CampaignID string `json:"campaign_id"`
	CharacterSubmission
}

// SubmitCharacter creates a new character from the builder form.
func (h *APIHandler) SubmitCharacter(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	result, err := h.builderSvc.CreateCharacter(r.Context(), req.CampaignID, userID, req.Token, req.CharacterSubmission)
	if err != nil {
		if isValidationError(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Error("creating character", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"character_id":        result.CharacterID,
		"player_character_id": result.PlayerCharacterID,
	})
}

func isValidationError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "validation")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
