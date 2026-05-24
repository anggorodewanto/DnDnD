package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
)

// CharCreateServicer is the interface for DM character creation. It is
// satisfied by *portal.BuilderService — the single creation core shared with
// the player portal.
type CharCreateServicer interface {
	CreateCharacterDM(ctx context.Context, campaignID string, sub portal.CharacterSubmission) (portal.CreateCharacterResult, error)
}

type abilityMethodLister interface {
	AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]portal.AbilityScoreMethod, error)
}

// RefDataForCreate provides reference data for the character creation form.
type RefDataForCreate interface {
	ListRaces(ctx context.Context) ([]portal.RaceInfo, error)
	ListClasses(ctx context.Context) ([]portal.ClassInfo, error)
	ListEquipment(ctx context.Context, campaignID string) ([]portal.EquipmentItem, error)
	ListSpellsByClass(ctx context.Context, class string, campaignID string) ([]portal.SpellInfo, error)
}

// CharCreateHandler serves the DM character creation JSON API.
type CharCreateHandler struct {
	logger          *slog.Logger
	svc             CharCreateServicer
	refData         RefDataForCreate
	featureProvider portal.FeatureProvider
	dmVerifier      DMVerifier
}

// SetDMVerifier sets the verifier used to check campaign ownership.
func (h *CharCreateHandler) SetDMVerifier(v DMVerifier) {
	h.dmVerifier = v
}

// SetFeatureProvider sets the feature provider for preview/features endpoints.
func (h *CharCreateHandler) SetFeatureProvider(fp portal.FeatureProvider) {
	h.featureProvider = fp
}

// NewCharCreateHandler creates a new CharCreateHandler.
func NewCharCreateHandler(logger *slog.Logger, svc CharCreateServicer, refData RefDataForCreate) *CharCreateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CharCreateHandler{
		logger:  logger,
		svc:     svc,
		refData: refData,
	}
}

// RegisterCharCreateRoutes mounts character creation routes on the given router.
// The HTML create page is served by the Svelte SPA — only the JSON API
// endpoints live in this handler.
func (h *CharCreateHandler) RegisterCharCreateRoutes(r chi.Router) {
	r.Route("/dashboard/api/characters", func(r chi.Router) {
		r.Post("/", h.HandleCreate)
		r.Post("/preview", h.HandlePreview)
		r.Get("/ref/races", h.HandleListRefRaces)
		r.Get("/ref/classes", h.HandleListRefClasses)
		r.Get("/ref/equipment", h.HandleListRefEquipment)
		r.Get("/ref/starting-equipment", h.HandleListRefStartingEquipment)
		r.Get("/ref/spells", h.HandleListRefSpells)
		r.Get("/ability-methods", h.HandleAbilityMethods)
	})
}

// dmCreateRequest is the JSON body for DM character creation.
type dmCreateRequest struct {
	CampaignID string `json:"campaign_id"`
	portal.CharacterSubmission
}

// HandleCreate creates a new DM character.
func (h *CharCreateHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuthHelper(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req dmCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if h.dmVerifier != nil && req.CampaignID != "" {
		owns, err := h.dmVerifier.IsCampaignDM(r.Context(), userID, req.CampaignID)
		if err != nil || !owns {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	result, err := h.svc.CreateCharacterDM(r.Context(), req.CampaignID, req.CharacterSubmission)
	if err != nil {
		if strings.HasPrefix(err.Error(), "validation") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Error("creating dm character", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, http.StatusCreated, map[string]string{
		"character_id":        result.CharacterID,
		"player_character_id": result.PlayerCharacterID,
	})
}

// HandlePreview returns derived stats without saving.
func (h *CharCreateHandler) HandlePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var sub portal.CharacterSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	stats := portal.DeriveStats(sub, h.buildRaceSpeedMap(r.Context()))
	if h.featureProvider != nil {
		stats.Features = portal.CollectFeatures(
			portal.SubmissionClasses(sub),
			h.featureProvider.ClassFeatures(),
			h.featureProvider.SubclassFeatures(),
			h.featureProvider.RacialTraits(sub.Race),
		)
	}
	writeJSONResponse(w, http.StatusOK, stats)
}

// HandleAbilityMethods returns campaign-enabled ability score methods.
func (h *CharCreateHandler) HandleAbilityMethods(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	methods := portal.DefaultAbilityScoreMethods()
	if lister, ok := h.svc.(abilityMethodLister); ok {
		allowed, err := lister.AllowedAbilityScoreMethods(r.Context(), r.URL.Query().Get("campaign_id"))
		if err != nil {
			h.logger.Error("listing ability methods", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		methods = allowed
	}
	writeJSONResponse(w, http.StatusOK, methods)
}

// HandleListRefRaces returns available races as JSON.
func (h *CharCreateHandler) HandleListRefRaces(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.RaceInfo{})
		return
	}

	races, err := h.refData.ListRaces(r.Context())
	if err != nil {
		h.logger.Error("listing races for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if races == nil {
		races = []portal.RaceInfo{}
	}
	writeJSONResponse(w, http.StatusOK, races)
}

// HandleListRefClasses returns available classes as JSON.
func (h *CharCreateHandler) HandleListRefClasses(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.ClassInfo{})
		return
	}

	classes, err := h.refData.ListClasses(r.Context())
	if err != nil {
		h.logger.Error("listing classes for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if classes == nil {
		classes = []portal.ClassInfo{}
	}
	writeJSONResponse(w, http.StatusOK, classes)
}

// HandleListRefEquipment returns available equipment (weapons + armor) as JSON.
func (h *CharCreateHandler) HandleListRefEquipment(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.EquipmentItem{})
		return
	}

	equipment, err := h.refData.ListEquipment(r.Context(), r.URL.Query().Get("campaign_id"))
	if err != nil {
		h.logger.Error("listing equipment for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if equipment == nil {
		equipment = []portal.EquipmentItem{}
	}
	writeJSONResponse(w, http.StatusOK, equipment)
}

// HandleListRefStartingEquipment returns starting equipment packs for a class.
func (h *CharCreateHandler) HandleListRefStartingEquipment(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class parameter is required", http.StatusBadRequest)
		return
	}

	packs := portal.StartingEquipmentPacks(class)
	if packs == nil {
		packs = []portal.EquipmentPack{}
	}
	writeJSONResponse(w, http.StatusOK, packs)
}

// HandleListRefSpells returns spells filtered by class as JSON.
func (h *CharCreateHandler) HandleListRefSpells(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class parameter is required", http.StatusBadRequest)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.SpellInfo{})
		return
	}

	spells, err := h.refData.ListSpellsByClass(r.Context(), class, r.URL.Query().Get("campaign_id"))
	if err != nil {
		h.logger.Error("listing spells for char creation", "error", err, "class", class)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if spells == nil {
		spells = []portal.SpellInfo{}
	}

	// Filter by max_level if provided
	if maxLevelStr := r.URL.Query().Get("max_level"); maxLevelStr != "" {
		maxLevel, err := strconv.Atoi(maxLevelStr)
		if err == nil {
			spells = filterSpellsByMaxLevel(spells, maxLevel)
		}
	}

	writeJSONResponse(w, http.StatusOK, spells)
}

// filterSpellsByMaxLevel returns only spells at or below the given spell level.
// Cantrips (level 0) are always included.
func filterSpellsByMaxLevel(spells []portal.SpellInfo, maxLevel int) []portal.SpellInfo {
	filtered := make([]portal.SpellInfo, 0, len(spells))
	for _, sp := range spells {
		if sp.Level <= maxLevel {
			filtered = append(filtered, sp)
		}
	}
	return filtered
}

// requireAuthHelper extracts the discord user ID from the request context.
func requireAuthHelper(r *http.Request) (string, bool) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

// buildRaceSpeedMap builds a lowercase race name → speed map from the DB.
// Returns nil if refData is unavailable (callers fall back to hardcoded table).
func (h *CharCreateHandler) buildRaceSpeedMap(ctx context.Context) map[string]int {
	if h.refData == nil {
		return nil
	}
	races, err := h.refData.ListRaces(ctx)
	if err != nil {
		return nil
	}
	m := make(map[string]int, len(races))
	for _, r := range races {
		m[strings.ToLower(r.Name)] = r.SpeedFt
	}
	return m
}

// writeJSONResponse writes a JSON response with the given status code.
func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
