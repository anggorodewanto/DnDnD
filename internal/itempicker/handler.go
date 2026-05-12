package itempicker

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// Store defines the database operations needed by the item picker.
type Store interface {
	ListWeapons(ctx context.Context) ([]refdata.Weapon, error)
	ListArmor(ctx context.Context) ([]refdata.Armor, error)
	ListMagicItems(ctx context.Context) ([]refdata.MagicItem, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
}

// SearchResult is a unified item representation returned by the search endpoint.
type SearchResult struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	CostGP      int                    `json:"cost_gp"`
	// F-86: surface the refdata homebrew flag so DM consumers can render a
	// "homebrew" pill and let the picker filter on it server-side.
	Homebrew bool                   `json:"homebrew"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Handler handles item picker HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a new item picker Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// HandleSearch handles GET /api/campaigns/:id/items/search?q=...&category=...&homebrew=true|false
// F-86: the optional `homebrew` query param filters by the refdata homebrew
// flag — "true" keeps only homebrew rows, "false" keeps only SRD/official
// rows, anything else (or unset) returns both. Every result also includes
// the boolean `homebrew` field in its body so callers can render a badge
// without a second lookup.
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	category := r.URL.Query().Get("category")
	homebrewFilter, hasHomebrewFilter := parseHomebrewFilter(r.URL.Query().Get("homebrew"))

	results := []SearchResult{}

	if category == "" || category == "weapons" {
		weapons, err := h.store.ListWeapons(r.Context())
		if err != nil {
			jsonError(w, "failed to list weapons", http.StatusInternalServerError)
			return
		}
		for _, wp := range weapons {
			if q != "" && !strings.Contains(strings.ToLower(wp.Name), q) {
				continue
			}
			homebrew := wp.Homebrew.Valid && wp.Homebrew.Bool
			if hasHomebrewFilter && homebrew != homebrewFilter {
				continue
			}
			results = append(results, SearchResult{
				ID:       wp.ID,
				Name:     wp.Name,
				Type:     "weapon",
				Homebrew: homebrew,
				Metadata: map[string]interface{}{
					"damage":      wp.Damage,
					"damage_type": wp.DamageType,
					"weapon_type": wp.WeaponType,
				},
			})
		}
	}

	if category == "" || category == "armor" {
		armor, err := h.store.ListArmor(r.Context())
		if err != nil {
			jsonError(w, "failed to list armor", http.StatusInternalServerError)
			return
		}
		for _, a := range armor {
			if q != "" && !strings.Contains(strings.ToLower(a.Name), q) {
				continue
			}
			// Armor refdata has no homebrew column today — treat every
			// row as official so the filter shape stays consistent.
			homebrew := false
			if hasHomebrewFilter && homebrew != homebrewFilter {
				continue
			}
			results = append(results, SearchResult{
				ID:       a.ID,
				Name:     a.Name,
				Type:     "armor",
				Homebrew: homebrew,
				Metadata: map[string]interface{}{
					"ac_base":    a.AcBase,
					"armor_type": a.ArmorType,
				},
			})
		}
	}

	if category == "" || category == "magic_items" {
		magicItems, err := h.store.ListMagicItems(r.Context())
		if err != nil {
			jsonError(w, "failed to list magic items", http.StatusInternalServerError)
			return
		}
		for _, mi := range magicItems {
			if q != "" && !strings.Contains(strings.ToLower(mi.Name), q) {
				continue
			}
			homebrew := mi.Homebrew.Valid && mi.Homebrew.Bool
			if hasHomebrewFilter && homebrew != homebrewFilter {
				continue
			}
			results = append(results, SearchResult{
				ID:          mi.ID,
				Name:        mi.Name,
				Type:        "magic_item",
				Description: mi.Description,
				Homebrew:    homebrew,
				Metadata: map[string]interface{}{
					"rarity":              mi.Rarity,
					"requires_attunement": mi.RequiresAttunement.Bool,
				},
			})
		}
	}

	jsonOK(w, results)
}

// parseHomebrewFilter decodes the `homebrew` query param into a (value, set)
// pair. Returns ok=false (skip filter) for empty / unrecognised values, so
// existing callers that don't pass the param continue to see all results.
func parseHomebrewFilter(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	default:
		return false, false
	}
}

// CustomEntryRequest is the JSON body for POST .../items/custom — the DM
// can register a one-off ("freeform") item that isn't in any refdata table.
// All fields besides Name are optional; the server lets downstream
// consumers (loot, shops, /give) decide defaults for missing values.
// (F-86 custom-entry surface)
type CustomEntryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Quantity    int    `json:"quantity,omitempty"`
	GoldGP      int    `json:"gold_gp,omitempty"`
	PriceGP     int    `json:"price_gp,omitempty"`
	Type        string `json:"type,omitempty"`
}

// CustomEntryResponse is the JSON returned to the DM after registering a
// custom item entry. Downstream code (loot.AddItemRequest, shops.CreateShop
// ItemParams) consumes the same fields the request carries plus a generated
// ID so the freeform entry can be referenced once placed into inventory.
type CustomEntryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Quantity    int    `json:"quantity"`
	GoldGP      int    `json:"gold_gp"`
	PriceGP     int    `json:"price_gp"`
	Custom      bool   `json:"custom"`
	Homebrew    bool   `json:"homebrew"`
}

// HandleCustomEntry handles POST /api/campaigns/:id/items/custom — registers
// a freeform DM-authored item (name + optional description + quantity +
// gold). F-86: the item picker shared component has historically only
// surfaced SRD-imported rows; this lets the DM hand-roll an entry for a
// single shop / loot drop without a homebrew round-trip.
func (h *Handler) HandleCustomEntry(w http.ResponseWriter, r *http.Request) {
	var req CustomEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}
	itemType := strings.TrimSpace(req.Type)
	if itemType == "" {
		itemType = "custom"
	}
	resp := CustomEntryResponse{
		ID:          "custom-" + uuid.New().String(),
		Name:        name,
		Type:        itemType,
		Description: req.Description,
		Quantity:    qty,
		GoldGP:      req.GoldGP,
		PriceGP:     req.PriceGP,
		Custom:      true,
		Homebrew:    true,
	}
	jsonOK(w, resp)
}

// CreatureInventory holds a single defeated creature's items and gold.
type CreatureInventory struct {
	Name  string                   `json:"name"`
	Gold  int32                    `json:"gold"`
	Items []character.InventoryItem `json:"items"`
}

// CreatureInventoriesResponse is the response for the creature inventories endpoint.
type CreatureInventoriesResponse struct {
	Creatures []CreatureInventory `json:"creatures"`
}

// HandleCreatureInventories handles GET /api/campaigns/:id/encounters/:eid/creature-inventories
func (h *Handler) HandleCreatureInventories(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		jsonError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	combatants, err := h.store.ListCombatantsByEncounterID(r.Context(), encounterID)
	if err != nil {
		jsonError(w, "failed to list combatants", http.StatusInternalServerError)
		return
	}

	creatures := []CreatureInventory{}
	for _, c := range combatants {
		if !c.IsNpc || c.IsAlive || !c.CharacterID.Valid {
			continue
		}

		char, err := h.store.GetCharacter(r.Context(), c.CharacterID.UUID)
		if err != nil {
			continue // skip if character not found
		}

		items, _ := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
		if items == nil {
			items = []character.InventoryItem{}
		}
		creatures = append(creatures, CreatureInventory{
			Name:  c.DisplayName,
			Gold:  char.Gold,
			Items: items,
		})
	}

	jsonOK(w, CreatureInventoriesResponse{Creatures: creatures})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
