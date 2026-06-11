package portal

import (
	"context"
	"encoding/json"
	"errors"
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
	DarkvisionFt   int             `json:"darkvision_ft,omitempty"`
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
	WeaponMasteryCount  int             `json:"weapon_mastery_count,omitempty"`
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
	Mastery    string   `json:"mastery,omitempty"`
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
	ListEquipment(ctx context.Context, campaignID string) ([]EquipmentItem, error)
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
	campaignID := r.URL.Query().Get("campaign_id")
	items, err := h.refData.ListEquipment(r.Context(), campaignID)
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

// ListAbilityMethods returns campaign-enabled ability-score generation methods.
func (h *APIHandler) ListAbilityMethods(w http.ResponseWriter, r *http.Request) {
	campaignID := r.URL.Query().Get("campaign_id")
	methods := DefaultAbilityScoreMethods()
	if h.builderSvc != nil {
		allowed, err := h.builderSvc.AllowedAbilityScoreMethods(r.Context(), campaignID)
		if err != nil {
			h.logger.Error("listing ability methods", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		methods = allowed
	}
	writeJSON(w, http.StatusOK, methods)
}

// draftModePlayer is the draft-namespace mode for the player portal. It mirrors
// the client's draftScope() prefix so a player PC draft and a DM NPC draft for
// the same campaign never collide. The portal endpoints only ever serve the
// player surface, so the mode is fixed server-side rather than trusted from the
// request.
const draftModePlayer = "player"

// submitRequest is the JSON body for character submission. BuilderDraft is the
// optional raw builder-state blob (serializeDraft output) the client sends so
// the server can persist it for the "request changes" / cross-device resume
// flow (T11 / Finding 4·b); it is stored opaquely and never interpreted here.
type submitRequest struct {
	Token        string          `json:"token"`
	CampaignID   string          `json:"campaign_id"`
	BuilderDraft json.RawMessage `json:"builder_draft,omitempty"`
	CharacterSubmission
}

// draftResponse is the JSON body returned by GetCharacterDraft. Draft is null
// when no draft is stored for the player.
type draftResponse struct {
	Draft json.RawMessage `json:"draft"`
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

	// Persist the builder draft first and unconditionally (best-effort): a
	// submit that fails the token check — expired / already-used — must still
	// leave the work recoverable so the reissued /create-character link
	// rehydrates the form instead of a blank one (T11 / Finding 4·b).
	if err := h.builderSvc.SaveDraft(r.Context(), req.CampaignID, userID, draftModePlayer, req.BuilderDraft); err != nil {
		h.logger.Warn("saving builder draft", "error", err)
	}

	result, err := h.builderSvc.CreateCharacter(r.Context(), req.CampaignID, userID, req.Token, req.CharacterSubmission)
	if err != nil {
		if isValidationError(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrAlreadyActive) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if status, msg, ok := tokenErrorResponse(err); ok {
			http.Error(w, msg, status)
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

// PreviewCharacter returns derived stats for a draft submission without
// persisting it, so the builder can show a live stat preview.
func (h *APIHandler) PreviewCharacter(w http.ResponseWriter, r *http.Request) {
	if userID, ok := auth.DiscordUserIDFromContext(r.Context()); !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.builderSvc == nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}

	var sub CharacterSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, h.builderSvc.Preview(r.Context(), sub))
}

// GetCharacterDraft returns the player's stored in-progress builder draft for
// the given campaign so the builder can rehydrate after a "request changes"
// cycle or on another device (T11 / Finding 4·b). The body is {"draft": null}
// when nothing is stored; the draft blob is whatever the client previously
// submitted and is returned verbatim.
func (h *APIHandler) GetCharacterDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID == "" {
		http.Error(w, "campaign_id is required", http.StatusBadRequest)
		return
	}

	draft, err := h.builderSvc.LoadDraft(r.Context(), campaignID, userID, draftModePlayer)
	if err != nil {
		h.logger.Error("loading builder draft", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, draftResponse{Draft: draft})
}

func isValidationError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "validation")
}

// tokenErrorResponse maps a portal-token failure to an HTTP status and a
// player-facing message that tells them how to recover. Token errors reach the
// handler wrapped ("validating token: ..." or "redeeming token: ..."), so
// errors.Is is used to see through the chain. The bool is false when err is not
// a recognized token error, leaving the caller on the generic 500 path. Status
// codes mirror the page route's handleTokenError so both surfaces agree:
// expired/used are 410 Gone, unknown is 404, wrong-account is 403.
func tokenErrorResponse(err error) (status int, msg string, ok bool) {
	const reissue = "Please request a new link from Discord with /create-character."
	switch {
	case errors.Is(err, ErrTokenExpired):
		return http.StatusGone, "This link has expired. " + reissue, true
	case errors.Is(err, ErrTokenUsed):
		return http.StatusGone, "This link has already been used. " + reissue, true
	case errors.Is(err, ErrTokenNotFound):
		return http.StatusNotFound, "This link is invalid or was not found. " + reissue, true
	case errors.Is(err, ErrTokenOwnership):
		return http.StatusForbidden, "This link belongs to a different Discord account. " + reissue, true
	}
	return 0, "", false
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
