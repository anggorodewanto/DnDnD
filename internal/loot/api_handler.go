package loot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// APIHandler handles loot pool HTTP endpoints.
type APIHandler struct {
	svc         *Service
	combatLogFn func(msg string)
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(svc *Service) *APIHandler {
	return &APIHandler{svc: svc}
}

// SetCombatLogFunc sets the callback for posting messages to #combat-log.
func (h *APIHandler) SetCombatLogFunc(fn func(msg string)) {
	h.combatLogFn = fn
}

func (h *APIHandler) logCombat(msg string) {
	if h.combatLogFn != nil {
		h.combatLogFn(msg)
	}
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// HandleGetLootPool handles GET /api/campaigns/:id/encounters/:eid/loot.
func (h *APIHandler) HandleGetLootPool(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	jsonOK(w, result)
}

// HandleCreateLootPool handles POST /api/campaigns/:id/encounters/:eid/loot.
func (h *APIHandler) HandleCreateLootPool(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.CreateLootPool(r.Context(), encounterID)
	if err != nil {
		switch err {
		case ErrEncounterNotCompleted:
			jsonError(w, err.Error(), http.StatusBadRequest)
		case ErrPoolAlreadyExists:
			jsonError(w, err.Error(), http.StatusConflict)
		default:
			jsonError(w, "failed to create loot pool", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, result)
}

// AddItemRequest is the request body for adding an item.
type AddItemRequest struct {
	ItemID             string `json:"item_id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Quantity           int    `json:"quantity"`
	Type               string `json:"type"`
	IsMagic            bool   `json:"is_magic"`
	MagicBonus         int    `json:"magic_bonus"`
	MagicProperties    string `json:"magic_properties"`
	RequiresAttunement bool   `json:"requires_attunement"`
	Rarity             string `json:"rarity"`
}

// HandleAddItem handles POST /api/campaigns/:id/encounters/:eid/loot/items.
func (h *APIHandler) HandleAddItem(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.Type == "" {
		req.Type = "other"
	}

	pool, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	item, err := h.svc.AddItem(r.Context(), pool.Pool.ID, refdata.CreateLootPoolItemParams{
		ItemID:             sql.NullString{String: req.ItemID, Valid: req.ItemID != ""},
		Name:               req.Name,
		Description:        req.Description,
		Quantity:           int32(req.Quantity),
		Type:               req.Type,
		IsMagic:            req.IsMagic,
		MagicBonus:         int32(req.MagicBonus),
		MagicProperties:    req.MagicProperties,
		RequiresAttunement: req.RequiresAttunement,
		Rarity:             req.Rarity,
	})
	if err != nil {
		jsonError(w, "failed to add item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, item)
}

// HandleRemoveItem handles DELETE /api/campaigns/:id/encounters/:eid/loot/items/:itemID.
func (h *APIHandler) HandleRemoveItem(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		jsonError(w, "invalid item_id", http.StatusBadRequest)
		return
	}

	pool, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	if err := h.svc.RemoveItem(r.Context(), pool.Pool.ID, itemID); err != nil {
		jsonError(w, "failed to remove item", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

// SetGoldRequest is the request body for setting gold.
type SetGoldRequest struct {
	Gold int `json:"gold"`
}

// HandleSplitGold handles POST /api/campaigns/:id/encounters/:eid/loot/split-gold.
func (h *APIHandler) HandleSplitGold(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	pool, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	share, err := h.svc.SplitGold(r.Context(), pool.Pool.ID)
	if err != nil {
		jsonError(w, fmt.Sprintf("failed to split gold: %v", err), http.StatusInternalServerError)
		return
	}

	h.logCombat(fmt.Sprintf("\U0001f4b0 Gold split: %d gp each", share))
	jsonOK(w, map[string]interface{}{"share": share})
}

// HandlePostAnnouncement handles POST /api/campaigns/:id/encounters/:eid/loot/post.
func (h *APIHandler) HandlePostAnnouncement(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	msg := FormatLootAnnouncement(result.Pool, result.Items)
	h.logCombat(msg)

	jsonOK(w, map[string]string{"status": "ok", "message": msg})
}

// HandleSetGold handles PUT for gold updates on the loot pool.
func (h *APIHandler) HandleSetGold(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	var req SetGoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	pool, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	updated, err := h.svc.SetGold(r.Context(), pool.Pool.ID, int32(req.Gold))
	if err != nil {
		jsonError(w, "failed to set gold", http.StatusInternalServerError)
		return
	}

	jsonOK(w, updated)
}

// HandleClearPool handles DELETE /api/campaigns/:id/encounters/:eid/loot.
func (h *APIHandler) HandleClearPool(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	pool, err := h.svc.GetLootPool(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "loot pool not found", http.StatusNotFound)
		return
	}

	if err := h.svc.ClearPool(r.Context(), pool.Pool.ID); err != nil {
		jsonError(w, "failed to clear pool", http.StatusInternalServerError)
		return
	}

	h.logCombat("\U0001f4b0 Loot pool cleared.")
	jsonOK(w, map[string]string{"status": "ok"})
}
