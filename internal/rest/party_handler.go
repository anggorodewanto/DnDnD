package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/google/uuid"
)

// PartyCharacterInfo holds the data needed to apply rest to a character.
type PartyCharacterInfo struct {
	ID               uuid.UUID
	Name             string
	DiscordUserID    string
	HPCurrent        int
	HPMax            int
	TempHP           int
	CONModifier      int
	HitDiceRemaining map[string]int
	Classes          []character.ClassEntry
	FeatureUses      map[string]character.FeatureUse
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
	DeathSaves       int // successes + failures combined (nonzero means needs reset)
	ExhaustionLevel  int
	Inventory        []character.InventoryItem
	RechargeInfo     map[string]inventory.RechargeInfo
}

// CharacterRestUpdate holds the data to persist after a rest.
type CharacterRestUpdate struct {
	CharacterID      uuid.UUID
	HPCurrent        int
	TempHPCleared    bool
	HitDiceRemaining map[string]int
	FeatureUses      map[string]character.FeatureUse
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
	ExhaustionLevel  int
	UpdatedInventory []character.InventoryItem
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
			TempHP:           c.TempHP,
			HitDiceRemaining: c.HitDiceRemaining,
			Classes:          c.Classes,
			FeatureUses:      c.FeatureUses,
			SpellSlots:       c.SpellSlots,
			PactMagicSlots:   c.PactMagicSlots,
			ExhaustionLevel:  c.ExhaustionLevel,
			Inventory:        c.Inventory,
			RechargeInfo:     c.RechargeInfo,
		}

		result := h.restService.LongRest(input)

		update := CharacterRestUpdate{
			CharacterID:      c.ID,
			HPCurrent:        result.HPAfter,
			TempHPCleared:    result.TempHPCleared,
			HitDiceRemaining: result.HitDiceRemaining,
			FeatureUses:      c.FeatureUses,
			SpellSlots:       result.SpellSlots,
			PactMagicSlots:   c.PactMagicSlots,
			ExhaustionLevel:  result.ExhaustionLevelAfter,
			UpdatedInventory: result.UpdatedInventory,
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

// SpendHitDiceRequest is the JSON body for the DM spend-hit-dice endpoint.
type SpendHitDiceRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	CampaignID  uuid.UUID `json:"campaign_id"`
	NumDice     int       `json:"num_dice"`
}

// HandleSpendHitDice lets the DM spend a single character's hit dice out of
// combat — rolling 1dX + CON each, healing (capped at max HP) and decrementing
// the dice. It is the DM-side fix for a player who took a short rest but skipped
// the hit-dice prompt: nothing was consumed, so the dice are still available.
//
// It deliberately spends hit dice ONLY — it does not re-recharge short-rest
// features or pact-magic slots (that would double-dip a character who already
// rested and has since spent those resources).
func (h *PartyRestHandler) HandleSpendHitDice(w http.ResponseWriter, r *http.Request) {
	var req SpendHitDiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NumDice < 1 {
		http.Error(w, "num_dice must be at least 1", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if h.encounter.HasActiveEncounter(ctx, req.CampaignID) {
		http.Error(w, "cannot spend hit dice during active combat", http.StatusConflict)
		return
	}

	allChars, err := h.lister.ListPartyCharacters(ctx, req.CampaignID)
	if err != nil {
		http.Error(w, "failed to list characters", http.StatusInternalServerError)
		return
	}

	var target *PartyCharacterInfo
	for i := range allChars {
		if allChars[i].ID == req.CharacterID {
			target = &allChars[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "character not found in campaign", http.StatusNotFound)
		return
	}

	spend, err := buildHitDiceSpend(target.HitDiceRemaining, req.NumDice)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.spendHitDice(ctx, *target, spend)
	if err != nil {
		http.Error(w, "failed to spend hit dice", http.StatusInternalServerError)
		return
	}

	_ = h.notifier.NotifyPlayer(ctx, PlayerNotification{
		DiscordUserID: target.DiscordUserID,
		CharacterName: target.Name,
		Message:       FormatShortRestResult(target.Name, result),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":             "ok",
		"hp_before":          result.HPBefore,
		"hp_after":           result.HPAfter,
		"hp_max":             result.HPMax,
		"hp_healed":          result.HPHealed,
		"hit_dice_remaining": result.HitDiceRemaining,
		"rolls":              result.HitDieRolls,
	})
}

// spendHitDice rolls the given hit-dice spend for one character and persists the
// healed HP + decremented dice, leaving all other resources untouched. It feeds
// the rest service nil feature/pact inputs so only the hit-dice loop runs, then
// re-writes the character's existing FeatureUses/PactMagicSlots unchanged so the
// updater's unconditional marshal cannot null them out.
func (h *PartyRestHandler) spendHitDice(ctx context.Context, c PartyCharacterInfo, spend map[string]int) (ShortRestResult, error) {
	result, err := h.restService.ShortRest(ShortRestInput{
		HPCurrent:        c.HPCurrent,
		HPMax:            c.HPMax,
		CONModifier:      c.CONModifier,
		HitDiceRemaining: c.HitDiceRemaining,
		HitDiceSpend:     spend,
	})
	if err != nil {
		return ShortRestResult{}, err
	}

	if err := h.updater.ApplyRestUpdate(ctx, CharacterRestUpdate{
		CharacterID:      c.ID,
		HPCurrent:        result.HPAfter,
		HitDiceRemaining: result.HitDiceRemaining,
		FeatureUses:      c.FeatureUses,
		PactMagicSlots:   c.PactMagicSlots,
		ExhaustionLevel:  c.ExhaustionLevel,
	}); err != nil {
		return ShortRestResult{}, err
	}

	return result, nil
}

// buildHitDiceSpend allocates numDice hit dice from a character's remaining
// pools, drawing from the largest die first (best expected healing). It errors
// when the character has fewer than numDice hit dice available.
func buildHitDiceSpend(remaining map[string]int, numDice int) (map[string]int, error) {
	total := 0
	for _, n := range remaining {
		total += n
	}
	if numDice > total {
		return nil, fmt.Errorf("cannot spend %d hit dice, only %d remaining", numDice, total)
	}

	types := make([]string, 0, len(remaining))
	for dt := range remaining {
		types = append(types, dt)
	}
	sort.Slice(types, func(i, j int) bool {
		return character.HitDieValue(types[i]) > character.HitDieValue(types[j])
	})

	spend := map[string]int{}
	left := numDice
	for _, dt := range types {
		if left == 0 {
			break
		}
		take := min(remaining[dt], left)
		if take > 0 {
			spend[dt] = take
			left -= take
		}
	}
	return spend, nil
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
