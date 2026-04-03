package shops

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// Handler handles shop HTTP endpoints.
type Handler struct {
	svc       *Service
	postToFn  func(channelID, content string)
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SetPostFunc sets the callback for posting messages to Discord channels.
func (h *Handler) SetPostFunc(fn func(channelID, content string)) {
	h.postToFn = fn
}

func (h *Handler) postTo(channelID, content string) {
	if h.postToFn != nil {
		h.postToFn(channelID, content)
	}
}

// HandleCreateShop handles POST /api/campaigns/{campaignID}/shops.
func (h *Handler) HandleCreateShop(w http.ResponseWriter, r *http.Request) {
	campaignID, err := uuid.Parse(chi.URLParam(r, "campaignID"))
	if err != nil {
		jsonError(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shop, err := h.svc.CreateShop(r.Context(), campaignID, req.Name, req.Description)
	if err != nil {
		if errors.Is(err, ErrNameRequired) {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonError(w, "failed to create shop", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, shop)
}

// HandleGetShop handles GET /api/campaigns/{campaignID}/shops/{shopID}.
func (h *Handler) HandleGetShop(w http.ResponseWriter, r *http.Request) {
	shopID, err := uuid.Parse(chi.URLParam(r, "shopID"))
	if err != nil {
		jsonError(w, "invalid shop_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.GetShop(r.Context(), shopID)
	if err != nil {
		if errors.Is(err, ErrShopNotFound) {
			jsonError(w, "shop not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to get shop", http.StatusInternalServerError)
		return
	}

	jsonOK(w, result)
}

// HandleListShops handles GET /api/campaigns/{campaignID}/shops.
func (h *Handler) HandleListShops(w http.ResponseWriter, r *http.Request) {
	campaignID, err := uuid.Parse(chi.URLParam(r, "campaignID"))
	if err != nil {
		jsonError(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	shops, err := h.svc.ListShops(r.Context(), campaignID)
	if err != nil {
		jsonError(w, "failed to list shops", http.StatusInternalServerError)
		return
	}

	jsonOK(w, shops)
}

// HandleUpdateShop handles PUT /api/campaigns/{campaignID}/shops/{shopID}.
func (h *Handler) HandleUpdateShop(w http.ResponseWriter, r *http.Request) {
	shopID, err := uuid.Parse(chi.URLParam(r, "shopID"))
	if err != nil {
		jsonError(w, "invalid shop_id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shop, err := h.svc.UpdateShop(r.Context(), shopID, req.Name, req.Description)
	if err != nil {
		if errors.Is(err, ErrShopNotFound) {
			jsonError(w, "shop not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrNameRequired) {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonError(w, "failed to update shop", http.StatusInternalServerError)
		return
	}

	jsonOK(w, shop)
}

// HandleDeleteShop handles DELETE /api/campaigns/{campaignID}/shops/{shopID}.
func (h *Handler) HandleDeleteShop(w http.ResponseWriter, r *http.Request) {
	shopID, err := uuid.Parse(chi.URLParam(r, "shopID"))
	if err != nil {
		jsonError(w, "invalid shop_id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteShop(r.Context(), shopID); err != nil {
		if errors.Is(err, ErrShopNotFound) {
			jsonError(w, "shop not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to delete shop", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

// AddItemRequest is the request body for adding a shop item.
type AddItemRequest struct {
	ItemID      string `json:"item_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceGP     int    `json:"price_gp"`
	Quantity    int    `json:"quantity"`
	Type        string `json:"type"`
}

// HandleAddItem handles POST /api/campaigns/{campaignID}/shops/{shopID}/items.
func (h *Handler) HandleAddItem(w http.ResponseWriter, r *http.Request) {
	shopID, err := uuid.Parse(chi.URLParam(r, "shopID"))
	if err != nil {
		jsonError(w, "invalid shop_id", http.StatusBadRequest)
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

	item, err := h.svc.AddItem(r.Context(), shopID, refdata.CreateShopItemParams{
		ItemID:      req.ItemID,
		Name:        req.Name,
		Description: req.Description,
		PriceGp:     int32(req.PriceGP),
		Quantity:    int32(req.Quantity),
		Type:        req.Type,
	})
	if err != nil {
		if errors.Is(err, ErrShopNotFound) {
			jsonError(w, "shop not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to add item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, item)
}

// HandleRemoveItem handles DELETE /api/campaigns/{campaignID}/shops/{shopID}/items/{itemID}.
func (h *Handler) HandleRemoveItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		jsonError(w, "invalid item_id", http.StatusBadRequest)
		return
	}

	if err := h.svc.RemoveItem(r.Context(), itemID); err != nil {
		jsonError(w, "failed to remove item", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

// HandlePostToDiscord handles POST /api/campaigns/{campaignID}/shops/{shopID}/post.
func (h *Handler) HandlePostToDiscord(w http.ResponseWriter, r *http.Request) {
	campaignID, err := uuid.Parse(chi.URLParam(r, "campaignID"))
	if err != nil {
		jsonError(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	shopID, err := uuid.Parse(chi.URLParam(r, "shopID"))
	if err != nil {
		jsonError(w, "invalid shop_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.GetShop(r.Context(), shopID)
	if err != nil {
		if errors.Is(err, ErrShopNotFound) {
			jsonError(w, "shop not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to get shop", http.StatusInternalServerError)
		return
	}

	// Look up the-story channel from campaign settings
	camp, err := h.svc.GetCampaign(r.Context(), campaignID)
	if err != nil {
		jsonError(w, "campaign not found", http.StatusNotFound)
		return
	}

	channelID, err := getTheStoryChannelID(camp)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := FormatShopAnnouncement(result.Shop, result.Items)
	h.postTo(channelID, msg)

	jsonOK(w, map[string]string{"status": "ok", "message": msg})
}

func getTheStoryChannelID(camp refdata.Campaign) (string, error) {
	if !camp.Settings.Valid {
		return "", fmt.Errorf("campaign settings not configured")
	}
	var settings campaign.Settings
	if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
		return "", fmt.Errorf("parsing campaign settings: %w", err)
	}
	channelID, ok := settings.ChannelIDs["the-story"]
	if !ok || channelID == "" {
		return "", fmt.Errorf("the-story channel not configured")
	}
	return channelID, nil
}

// HandleUpdateItem handles PUT /api/campaigns/{campaignID}/shops/{shopID}/items/{itemID}.
func (h *Handler) HandleUpdateItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		jsonError(w, "invalid item_id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceGP     int    `json:"price_gp"`
		Quantity    int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	item, err := h.svc.UpdateItem(r.Context(), refdata.UpdateShopItemParams{
		ID:          itemID,
		Name:        req.Name,
		Description: req.Description,
		PriceGp:     int32(req.PriceGP),
		Quantity:    int32(req.Quantity),
	})
	if err != nil {
		jsonError(w, "failed to update item", http.StatusInternalServerError)
		return
	}

	jsonOK(w, item)
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

