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
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Handler handles item picker HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a new item picker Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// HandleSearch handles GET /api/campaigns/:id/items/search?q=...&category=...
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	category := r.URL.Query().Get("category")

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
			results = append(results, SearchResult{
				ID:   wp.ID,
				Name: wp.Name,
				Type: "weapon",
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
			results = append(results, SearchResult{
				ID:   a.ID,
				Name: a.Name,
				Type: "armor",
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
			results = append(results, SearchResult{
				ID:          mi.ID,
				Name:        mi.Name,
				Type:        "magic_item",
				Description: mi.Description,
				Metadata: map[string]interface{}{
					"rarity":             mi.Rarity,
					"requires_attunement": mi.RequiresAttunement.Bool,
				},
			})
		}
	}

	jsonOK(w, results)
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
