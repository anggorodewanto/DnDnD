package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ab/dndnd/internal/character"
	"github.com/google/uuid"
)

// PartyCharacterInfo holds the data needed to apply rest to a character.
type PartyCharacterInfo struct {
	ID               uuid.UUID
	Name             string
	DiscordUserID    string
	HPCurrent        int
	HPMax            int
	CONModifier      int
	HitDiceRemaining map[string]int
	Classes          []character.ClassEntry
	FeatureUses      map[string]character.FeatureUse
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
	DeathSaves       int // successes + failures combined (nonzero means needs reset)
	ExhaustionLevel  int
}

// CharacterRestUpdate holds the data to persist after a rest.
type CharacterRestUpdate struct {
	CharacterID      uuid.UUID
	HPCurrent        int
	HitDiceRemaining map[string]int
	FeatureUses      map[string]character.FeatureUse
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
	ExhaustionLevel  int
}

// PlayerNotification holds data for a notification to a player.
type PlayerNotification struct {
	DiscordUserID string
	CharacterName string
	Message       string
}

// PartyCharacterLister lists all characters in a campaign for party rest selection.
type PartyCharacterLister interface {
	ListPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]PartyCharacterInfo, error)
}

// PartyCharacterUpdater persists character rest updates.
type PartyCharacterUpdater interface {
	ApplyRestUpdate(ctx context.Context, update CharacterRestUpdate) error
}

// PartyEncounterChecker checks if a campaign has an active encounter.
type PartyEncounterChecker interface {
	HasActiveEncounter(ctx context.Context, campaignID uuid.UUID) bool
}

// PartyPlayerNotifier sends notifications to players.
type PartyPlayerNotifier interface {
	NotifyPlayer(ctx context.Context, n PlayerNotification) error
}

// PartySummaryPoster posts summary messages to #roll-history.
type PartySummaryPoster interface {
	PostToRollHistory(ctx context.Context, campaignID uuid.UUID, msg string) error
}

// PartyRestRequest is the JSON body for the party rest endpoint.
type PartyRestRequest struct {
	RestType     string      `json:"rest_type"`
	CharacterIDs []uuid.UUID `json:"character_ids"`
	CampaignID   uuid.UUID   `json:"campaign_id"`
	Narration    string      `json:"narration,omitempty"`
}

// InterruptRestRequest is the JSON body for the interrupt rest endpoint.
type InterruptRestRequest struct {
	RestType       string      `json:"rest_type"`
	CharacterIDs   []uuid.UUID `json:"character_ids"`
	CampaignID     uuid.UUID   `json:"campaign_id"`
	Reason         string      `json:"reason"`
	OneHourElapsed bool        `json:"one_hour_elapsed"`
}

// PartyRestHandler handles party rest and interruption API endpoints.
type PartyRestHandler struct {
	restService *Service
	lister      PartyCharacterLister
	updater     PartyCharacterUpdater
	encounter   PartyEncounterChecker
	notifier    PartyPlayerNotifier
	poster      PartySummaryPoster
}

// NewPartyRestHandler creates a new PartyRestHandler.
func NewPartyRestHandler(
	restService *Service,
	lister PartyCharacterLister,
	updater PartyCharacterUpdater,
	encounter PartyEncounterChecker,
	notifier PartyPlayerNotifier,
	poster PartySummaryPoster,
) *PartyRestHandler {
	return &PartyRestHandler{
		restService: restService,
		lister:      lister,
		updater:     updater,
		encounter:   encounter,
		notifier:    notifier,
		poster:      poster,
	}
}

// HandlePartyRest processes a DM-initiated party rest.
func (h *PartyRestHandler) HandlePartyRest(w http.ResponseWriter, r *http.Request) {
	var req PartyRestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RestType != "short" && req.RestType != "long" {
		http.Error(w, "rest_type must be 'short' or 'long'", http.StatusBadRequest)
		return
	}

	if len(req.CharacterIDs) == 0 {
		http.Error(w, "character_ids must not be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if h.encounter.HasActiveEncounter(ctx, req.CampaignID) {
		http.Error(w, "cannot rest during active combat", http.StatusConflict)
		return
	}

	allChars, err := h.lister.ListPartyCharacters(ctx, req.CampaignID)
	if err != nil {
		http.Error(w, "failed to list characters", http.StatusInternalServerError)
		return
	}

	selectedSet := uuidSet(req.CharacterIDs)

	// Partition into rested and excluded
	var rested []PartyCharacterInfo
	var restedNames, excludedNames []string
	for _, c := range allChars {
		if selectedSet[c.ID] {
			rested = append(rested, c)
			restedNames = append(restedNames, c.Name)
		} else {
			excludedNames = append(excludedNames, c.Name)
		}
	}

	switch req.RestType {
	case "long":
		h.applyPartyLongRest(ctx, rested)
	case "short":
		h.applyPartyShortRest(ctx, rested)
	}

	// Post summary
	summary := FormatPartyRestSummary(req.RestType, restedNames, excludedNames)
	_ = h.poster.PostToRollHistory(ctx, req.CampaignID, summary)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "summary": summary})
}

func (h *PartyRestHandler) applyPartyLongRest(ctx context.Context, chars []PartyCharacterInfo) {
	for _, c := range chars {
		input := LongRestInput{
			HPCurrent:        c.HPCurrent,
			HPMax:            c.HPMax,
			HitDiceRemaining: c.HitDiceRemaining,
			Classes:          c.Classes,
			FeatureUses:      c.FeatureUses,
			SpellSlots:       c.SpellSlots,
			PactMagicSlots:   c.PactMagicSlots,
			ExhaustionLevel:  c.ExhaustionLevel,
		}

		result := h.restService.LongRest(input)

		update := CharacterRestUpdate{
			CharacterID:      c.ID,
			HPCurrent:        result.HPAfter,
			HitDiceRemaining: result.HitDiceRemaining,
			FeatureUses:      c.FeatureUses,
			SpellSlots:       result.SpellSlots,
			PactMagicSlots:   c.PactMagicSlots,
			ExhaustionLevel:  result.ExhaustionLevelAfter,
		}
		_ = h.updater.ApplyRestUpdate(ctx, update)
		h.restService.PersistLongRestExhaustion(ctx, c.ID, result)

		msg := FormatLongRestResult(c.Name, result)
		_ = h.notifier.NotifyPlayer(ctx, PlayerNotification{
			DiscordUserID: c.DiscordUserID,
			CharacterName: c.Name,
			Message:       msg,
		})
	}
}

func (h *PartyRestHandler) applyPartyShortRest(ctx context.Context, chars []PartyCharacterInfo) {
	for _, c := range chars {
		if !h.applyCharShortRest(ctx, c) {
			continue
		}

		_ = h.notifier.NotifyPlayer(ctx, PlayerNotification{
			DiscordUserID: c.DiscordUserID,
			CharacterName: c.Name,
			Message:       "Short rest started. Use your hit dice buttons to heal.",
		})
	}
}

// applyCharShortRest applies a short rest (feature recharge, pact slots) to a
// single character and persists the update. Returns false if the rest failed.
func (h *PartyRestHandler) applyCharShortRest(ctx context.Context, c PartyCharacterInfo) bool {
	input := ShortRestInput{
		HPCurrent:        c.HPCurrent,
		HPMax:            c.HPMax,
		CONModifier:      c.CONModifier,
		HitDiceRemaining: c.HitDiceRemaining,
		HitDiceSpend:     map[string]int{},
		FeatureUses:      c.FeatureUses,
		PactMagicSlots:   c.PactMagicSlots,
		Classes:          c.Classes,
	}

	result, err := h.restService.ShortRest(input)
	if err != nil {
		return false
	}

	update := CharacterRestUpdate{
		CharacterID:      c.ID,
		HPCurrent:        result.HPAfter,
		HitDiceRemaining: result.HitDiceRemaining,
		FeatureUses:      c.FeatureUses,
		PactMagicSlots:   c.PactMagicSlots,
	}
	_ = h.updater.ApplyRestUpdate(ctx, update)
	return true
}

// RegisterPartyRestRoutes registers party rest API endpoints on the given mux.
func RegisterPartyRestRoutes(mux *http.ServeMux, h *PartyRestHandler) {
	mux.HandleFunc("POST /dashboard/api/party-rest", h.HandlePartyRest)
	mux.HandleFunc("POST /dashboard/api/interrupt-rest", h.HandleInterruptRest)
}

// HandleInterruptRest processes a DM-initiated rest interruption.
func (h *PartyRestHandler) HandleInterruptRest(w http.ResponseWriter, r *http.Request) {
	var req InterruptRestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	allChars, err := h.lister.ListPartyCharacters(ctx, req.CampaignID)
	if err != nil {
		http.Error(w, "failed to list characters", http.StatusInternalServerError)
		return
	}

	selectedSet := uuidSet(req.CharacterIDs)

	result := InterruptRest(req.RestType, req.OneHourElapsed)
	grantBenefits := result.Benefits == "short"

	for _, c := range allChars {
		if !selectedSet[c.ID] {
			continue
		}

		if grantBenefits {
			h.applyCharShortRest(ctx, c)
		}

		msg := FormatInterruptNotification(c.Name, req.RestType, req.Reason, grantBenefits)
		_ = h.notifier.NotifyPlayer(ctx, PlayerNotification{
			DiscordUserID: c.DiscordUserID,
			CharacterName: c.Name,
			Message:       msg,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// uuidSet builds a lookup set from a slice of UUIDs.
func uuidSet(ids []uuid.UUID) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
