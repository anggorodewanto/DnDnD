package characteroverview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/character"
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
	r.Get("/api/character-overview/{characterID}/slots", h.GetSlots)
	r.Post("/api/character-overview/{characterID}/slots", h.UpdateSlots)
	r.Get("/api/character-overview/{characterID}/feature-uses", h.GetFeatureUses)
	r.Post("/api/character-overview/{characterID}/feature-uses", h.UpdateFeatureUses)
}

// authorizeDM reports whether the request's Discord user owns the given campaign.
// With no verifier configured (test/dev), all callers are authorized.
func (h *Handler) authorizeDM(ctx context.Context, campaignID uuid.UUID) bool {
	if h.campaignVerifier == nil {
		return true
	}
	userID, _ := auth.DiscordUserIDFromContext(ctx)
	owns, err := h.campaignVerifier.IsCampaignDM(ctx, userID, campaignID.String())
	return err == nil && owns
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

// slotsRequest is the JSON body for an out-of-combat spell/pact slot edit. A nil
// field means "leave that store untouched".
type slotsRequest struct {
	SpellSlots     *json.RawMessage `json:"spell_slots"`
	PactMagicSlots *json.RawMessage `json:"pact_magic_slots"`
	Reason         string           `json:"reason"`
}

// slotsResponse is the JSON envelope returned by GetSlots/UpdateSlots: spell
// slots string-keyed for the wire, pact magic null when the character has none.
type slotsResponse struct {
	SpellSlots     map[string]character.SlotInfo `json:"spell_slots"`
	PactMagicSlots *character.PactMagicSlots     `json:"pact_magic_slots"`
}

// slotsResponseFrom renders int-keyed spell slots + a pact value into the wire
// shape (string-keyed spell map, nil pact when zero-valued).
func slotsResponseFrom(spell map[int]character.SlotInfo, pact character.PactMagicSlots) slotsResponse {
	strKeyed := make(map[string]character.SlotInfo, len(spell))
	for level, info := range spell {
		strKeyed[strconv.Itoa(level)] = info
	}
	var pactPtr *character.PactMagicSlots
	if pact != (character.PactMagicSlots{}) {
		p := pact
		pactPtr = &p
	}
	return slotsResponse{SpellSlots: strKeyed, PactMagicSlots: pactPtr}
}

// GetSlots returns a character's current spell/pact slots. Reads are allowed in
// or out of combat. DM-authorized via the owning campaign.
func (h *Handler) GetSlots(w http.ResponseWriter, r *http.Request) {
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sctx, err := h.svc.GetSlotsContext(ctx, characterID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load character", http.StatusInternalServerError)
		return
	}

	if !h.authorizeDM(ctx, sctx.CampaignID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, slotsResponseFrom(sctx.SpellSlots, sctx.PactMagicSlots))
}

// featureUsesResponse is the read-only payload for GetFeatureUses.
type featureUsesResponse struct {
	FeatureUses map[string]character.FeatureUse `json:"feature_uses"`
}

// GetFeatureUses returns a character's limited-use feature pools (e.g. Barbarian
// rage), keyed by feature name. Reads are allowed in or out of combat; it backs
// the in-combat feature-use override editor's prefill. DM-authorized via the
// owning campaign. A character with no such features yields an empty object.
func (h *Handler) GetFeatureUses(w http.ResponseWriter, r *http.Request) {
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sctx, err := h.svc.GetSlotsContext(ctx, characterID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load character", http.StatusInternalServerError)
		return
	}

	if !h.authorizeDM(ctx, sctx.CampaignID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	uses := map[string]character.FeatureUse{}
	if len(sctx.FeatureUses) > 0 {
		if err := json.Unmarshal(sctx.FeatureUses, &uses); err != nil {
			http.Error(w, "failed to parse feature_uses", http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusOK, featureUsesResponse{FeatureUses: uses})
}

// UpdateSlots applies a DM's out-of-combat edit to a character's spell and/or
// pact-magic slots. Refused (409) while the character is in an active combat;
// the in-combat controls own slot usage during combat.
func (h *Handler) UpdateSlots(w http.ResponseWriter, r *http.Request) {
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	var req slotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sctx, err := h.svc.GetSlotsContext(ctx, characterID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load character", http.StatusInternalServerError)
		return
	}

	if !h.authorizeDM(ctx, sctx.CampaignID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if sctx.InActiveCombat {
		http.Error(w, "character is in an active combat; use the in-combat controls", http.StatusConflict)
		return
	}

	update := SlotsUpdate{}
	spell := sctx.SpellSlots
	if req.SpellSlots != nil {
		parsed, err := character.ParseSpellSlotsJSON(*req.SpellSlots)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		update.SpellSlots = &parsed
		spell = parsed
	}
	pact := sctx.PactMagicSlots
	if req.PactMagicSlots != nil {
		var p character.PactMagicSlots
		if err := json.Unmarshal(*req.PactMagicSlots, &p); err != nil {
			http.Error(w, "invalid pact_magic_slots", http.StatusBadRequest)
			return
		}
		update.PactMagicSlots = &p
		pact = p
	}

	if err := h.svc.ApplySlots(ctx, characterID, update); err != nil {
		if errors.Is(err, ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to update slots", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, slotsResponseFrom(spell, pact))
}

// featureUsesUpdateRequest is the JSON body for an out-of-combat feature-uses
// edit: a batch of per-feature Current changes plus an optional audit reason.
// A nil Current marks a malformed change (the field was absent) and is rejected.
type featureUsesUpdateRequest struct {
	Changes []featureUseChangeRequest `json:"changes"`
	Reason  string                    `json:"reason"`
}

type featureUseChangeRequest struct {
	Feature string `json:"feature"`
	Current *int   `json:"current"`
}

// UpdateFeatureUses applies a DM's out-of-combat edit to a character's
// limited-use feature pools (Barbarian rage, ki, channel divinity, …). Each
// change sets one feature's remaining uses, preserving its Max + Recharge.
// Refused (409) while the character is in an active combat; the in-combat
// override owns feature uses during combat.
func (h *Handler) UpdateFeatureUses(w http.ResponseWriter, r *http.Request) {
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	var req featureUsesUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sctx, err := h.svc.GetSlotsContext(ctx, characterID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load character", http.StatusInternalServerError)
		return
	}

	if !h.authorizeDM(ctx, sctx.CampaignID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if sctx.InActiveCombat {
		http.Error(w, "character is in an active combat; use the in-combat controls", http.StatusConflict)
		return
	}

	// Validate body content after auth + combat checks, mirroring UpdateSlots —
	// an unauthorized caller gets 403, not a body-shape 400.
	changes := make([]FeatureUseChange, 0, len(req.Changes))
	for _, c := range req.Changes {
		if c.Feature == "" || c.Current == nil {
			http.Error(w, "each change requires a feature and current", http.StatusBadRequest)
			return
		}
		changes = append(changes, FeatureUseChange{Feature: c.Feature, Current: *c.Current})
	}

	uses, err := h.svc.ApplyFeatureUses(ctx, characterID, sctx.FeatureUses, changes)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to update feature uses", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, featureUsesResponse{FeatureUses: uses})
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
