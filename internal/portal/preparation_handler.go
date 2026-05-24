package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PrepareService is the subset of combat.Service used to drive web spell
// preparation. *combat.Service satisfies this interface directly.
type PrepareService interface {
	GetPreparationInfo(ctx context.Context, charID uuid.UUID, className, subclass string) (combat.PreparationInfo, error)
	PrepareSpells(ctx context.Context, input combat.PrepareSpellsInput) (combat.PrepareSpellsResult, error)
}

// CardUpdater refreshes the persistent #character-cards message after a
// successful preparation save (SR-007), keeping it in sync with the new
// prepared list. charactercard.Service.OnCharacterUpdated satisfies it; the
// interface is declared locally to avoid an import cycle. Errors are
// best-effort: a card-edit failure must never undo a committed save.
type CardUpdater interface {
	OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error
}

// PreparationStore provides owner-check and character lookup for the
// preparation endpoints. *CharacterSheetStoreAdapter satisfies GetCharacterOwner;
// the same adapter's querier exposes GetCharacter.
type PreparationStore interface {
	GetCharacterOwner(ctx context.Context, characterID string) (string, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
}

// PreparationStoreAdapter implements PreparationStore from a CharacterQuerier
// (which *refdata.Queries satisfies). It reuses CharacterSheetStoreAdapter for
// the character→campaign→player_character owner join and exposes GetCharacter
// directly from the querier.
type PreparationStoreAdapter struct {
	*CharacterSheetStoreAdapter
	q CharacterQuerier
}

// NewPreparationStoreAdapter creates a PreparationStoreAdapter.
func NewPreparationStoreAdapter(q CharacterQuerier) *PreparationStoreAdapter {
	return &PreparationStoreAdapter{
		CharacterSheetStoreAdapter: NewCharacterSheetStoreAdapter(q),
		q:                          q,
	}
}

// GetCharacter loads a character by UUID, mapping the no-rows case to
// ErrCharacterNotFound so the handler returns 404.
func (a *PreparationStoreAdapter) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	char, err := a.q.GetCharacter(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return refdata.Character{}, ErrCharacterNotFound
		}
		return refdata.Character{}, err
	}
	return char, nil
}

// PreparationHandler serves the web spell-preparation JSON API endpoints.
type PreparationHandler struct {
	logger      *slog.Logger
	svc         PrepareService
	store       PreparationStore
	refData     RefDataStore
	cardUpdater CardUpdater
}

// SetCardUpdater wires the SR-007 #character-cards refresh fired after a
// successful preparation save. Optional; nil is a no-op.
func (h *PreparationHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// NewPreparationHandler creates a new PreparationHandler.
func NewPreparationHandler(logger *slog.Logger, svc PrepareService, store PreparationStore, refData RefDataStore) *PreparationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PreparationHandler{
		logger:  logger,
		svc:     svc,
		store:   store,
		refData: refData,
	}
}

// preparationGETResponse is the GET /preparation response body.
type preparationGETResponse struct {
	CharacterName       string      `json:"character_name"`
	Class               string      `json:"class"`
	Subclass            string      `json:"subclass"`
	MaxPrepared         int         `json:"max_prepared"`
	CurrentPrepared     []string    `json:"current_prepared"`
	AlwaysPrepared      []string    `json:"always_prepared"`
	AvailableSlotLevels []int       `json:"available_slot_levels"`
	Spells              []SpellInfo `json:"spells"`
}

// preparationPOSTRequest is the POST /preparation request body.
type preparationPOSTRequest struct {
	Spells []string `json:"spells"`
}

// preparationPOSTResponse is the POST /preparation response body.
type preparationPOSTResponse struct {
	PreparedCount  int      `json:"prepared_count"`
	MaxPrepared    int      `json:"max_prepared"`
	AlwaysPrepared []string `json:"always_prepared"`
}

// preparationContext holds the resolved auth + caster data shared by both endpoints.
type preparationContext struct {
	charID   uuid.UUID
	char     refdata.Character
	class    string
	subclass string
}

// GetPreparation returns the current preparation state plus the full class
// spell list so the web UI can browse and select spells.
func (h *PreparationHandler) GetPreparation(w http.ResponseWriter, r *http.Request) {
	pctx, ok := h.resolve(w, r)
	if !ok {
		return
	}

	info, err := h.svc.GetPreparationInfo(r.Context(), pctx.charID, pctx.class, pctx.subclass)
	if err != nil {
		h.logger.Error("getting preparation info", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build the FULL class spell list (all levels) from the reused RefDataStore,
	// NOT info.ClassSpells which is pre-filtered to castable slot levels.
	spells, err := h.refData.ListSpellsByClass(r.Context(), pctx.class, pctx.char.CampaignID.String())
	if err != nil {
		h.logger.Error("listing class spells", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, preparationGETResponse{
		CharacterName:       pctx.char.Name,
		Class:               pctx.class,
		Subclass:            pctx.subclass,
		MaxPrepared:         info.MaxPrepared,
		CurrentPrepared:     nonNilStrings(info.CurrentPrepared),
		AlwaysPrepared:      nonNilStrings(info.AlwaysPrepared),
		AvailableSlotLevels: sortedSlotLevels(info.AvailableSlotLevels),
		Spells:              nonNilSpells(spells),
	})
}

// PostPreparation validates and persists the player's chosen prepared spells.
func (h *PreparationHandler) PostPreparation(w http.ResponseWriter, r *http.Request) {
	pctx, ok := h.resolve(w, r)
	if !ok {
		return
	}

	var body preparationPOSTRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	res, err := h.svc.PrepareSpells(r.Context(), combat.PrepareSpellsInput{
		CharacterID: pctx.charID,
		ClassName:   pctx.class,
		Subclass:    pctx.subclass,
		Selected:    body.Spells,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.notifyCardUpdate(r.Context(), pctx.charID)

	writeJSON(w, http.StatusOK, preparationPOSTResponse{
		PreparedCount:  res.PreparedCount,
		MaxPrepared:    res.MaxPrepared,
		AlwaysPrepared: nonNilStrings(res.AlwaysPrepared),
	})
}

// notifyCardUpdate fires the card refresh for the given character if one is
// wired. Best-effort: a nil updater is a no-op, and errors are logged and
// swallowed so a Discord hiccup cannot undo a committed save.
func (h *PreparationHandler) notifyCardUpdate(ctx context.Context, characterID uuid.UUID) {
	if h.cardUpdater == nil || characterID == uuid.Nil {
		return
	}
	if err := h.cardUpdater.OnCharacterUpdated(ctx, characterID); err != nil {
		h.logger.Error("character card auto-update failed", "character_id", characterID, "error", err)
	}
}

// resolve performs the shared auth + ownership + caster resolution for both
// endpoints. It writes the appropriate error response and returns ok=false on
// any failure.
func (h *PreparationHandler) resolve(w http.ResponseWriter, r *http.Request) (preparationContext, bool) {
	userID, authed := auth.DiscordUserIDFromContext(r.Context())
	if !authed || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return preparationContext{}, false
	}

	charIDStr := chi.URLParam(r, "characterID")
	charID, err := uuid.Parse(charIDStr)
	if err != nil {
		http.Error(w, "invalid character ID", http.StatusBadRequest)
		return preparationContext{}, false
	}

	ownerID, err := h.store.GetCharacterOwner(r.Context(), charIDStr)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return preparationContext{}, false
		}
		h.logger.Error("getting character owner", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return preparationContext{}, false
	}
	if ownerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return preparationContext{}, false
	}

	char, err := h.store.GetCharacter(r.Context(), charID)
	if err != nil {
		if errors.Is(err, ErrCharacterNotFound) {
			http.Error(w, "character not found", http.StatusNotFound)
			return preparationContext{}, false
		}
		h.logger.Error("getting character", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return preparationContext{}, false
	}

	class, subclass, isCaster := combat.ResolvePreparedClass(char.Classes, "", "")
	if !isCaster {
		http.Error(w, "not a prepared caster", http.StatusBadRequest)
		return preparationContext{}, false
	}

	return preparationContext{charID: charID, char: char, class: class, subclass: subclass}, true
}

// sortedSlotLevels returns the truthy keys of the slot-level set ascending.
func sortedSlotLevels(levels map[int]bool) []int {
	result := make([]int, 0, len(levels))
	for level, ok := range levels {
		if ok {
			result = append(result, level)
		}
	}
	sort.Ints(result)
	return result
}

// nonNilStrings guarantees a non-nil slice so JSON marshals [] not null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// nonNilSpells guarantees a non-nil slice so JSON marshals [] not null.
func nonNilSpells(s []SpellInfo) []SpellInfo {
	if s == nil {
		return []SpellInfo{}
	}
	return s
}
