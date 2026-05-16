package loot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// Errors returned by the loot service.
var (
	ErrEncounterNotCompleted = errors.New("encounter must be completed to create loot pool")
	ErrPoolNotFound          = errors.New("loot pool not found")
	ErrPoolClosed            = errors.New("loot pool is closed")
	ErrItemAlreadyClaimed    = errors.New("item already claimed")
	ErrPoolAlreadyExists     = errors.New("loot pool already exists for this encounter")
)

// Store defines the database operations needed by the loot service.
type Store interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	CreateLootPool(ctx context.Context, arg refdata.CreateLootPoolParams) (refdata.LootPool, error)
	CreateLootPoolItem(ctx context.Context, arg refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error)
	GetLootPoolByEncounter(ctx context.Context, encounterID uuid.UUID) (refdata.LootPool, error)
	GetLootPool(ctx context.Context, id uuid.UUID) (refdata.LootPool, error)
	ListLootPoolItems(ctx context.Context, lootPoolID uuid.UUID) ([]refdata.LootPoolItem, error)
	ClaimLootPoolItem(ctx context.Context, arg refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error)
	UpdateLootPoolGold(ctx context.Context, arg refdata.UpdateLootPoolGoldParams) (refdata.LootPool, error)
	UpdateLootPoolStatus(ctx context.Context, arg refdata.UpdateLootPoolStatusParams) (refdata.LootPool, error)
	DeleteLootPoolItem(ctx context.Context, id uuid.UUID) error
	DeleteUnclaimedLootPoolItems(ctx context.Context, lootPoolID uuid.UUID) error
	DeleteLootPool(ctx context.Context, id uuid.UUID) error
	ListPlayerCharactersByCampaignApproved(ctx context.Context, campaignID uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error)
	UpdateCharacterGold(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

// LootPoolResult holds the pool and its items.
type LootPoolResult struct {
	Pool  refdata.LootPool
	Items []refdata.LootPoolItem
}

// Service manages loot pool operations.
type Service struct {
	store Store
}

// NewService creates a new loot Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateLootPool creates a loot pool for a completed encounter, auto-populating
// from defeated creatures' inventories and gold.
func (s *Service) CreateLootPool(ctx context.Context, encounterID uuid.UUID) (LootPoolResult, error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return LootPoolResult{}, fmt.Errorf("getting encounter: %w", err)
	}
	if enc.Status != "completed" {
		return LootPoolResult{}, ErrEncounterNotCompleted
	}

	// Check if pool already exists
	_, err = s.store.GetLootPoolByEncounter(ctx, encounterID)
	if err == nil {
		return LootPoolResult{}, ErrPoolAlreadyExists
	}

	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return LootPoolResult{}, fmt.Errorf("listing combatants: %w", err)
	}

	// Collect items and gold from defeated NPCs with character sheets
	var allItems []character.InventoryItem
	var totalGold int32

	for _, c := range combatants {
		if !c.IsNpc || c.IsAlive {
			continue
		}
		if !c.CharacterID.Valid {
			continue
		}
		char, err := s.store.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil {
			continue // skip if character not found
		}
		totalGold += char.Gold

		items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}

	pool, err := s.store.CreateLootPool(ctx, refdata.CreateLootPoolParams{
		EncounterID: enc.ID,
		CampaignID:  enc.CampaignID,
		GoldTotal:   totalGold,
	})
	if err != nil {
		return LootPoolResult{}, fmt.Errorf("creating loot pool: %w", err)
	}

	var poolItems []refdata.LootPoolItem
	for _, item := range allItems {
		created, err := s.store.CreateLootPoolItem(ctx, refdata.CreateLootPoolItemParams{
			LootPoolID:         pool.ID,
			ItemID:             sql.NullString{String: item.ItemID, Valid: item.ItemID != ""},
			Name:               item.Name,
			Description:        "",
			Quantity:           int32(item.Quantity),
			Type:               item.Type,
			IsMagic:            item.IsMagic,
			MagicBonus:         int32(item.MagicBonus),
			MagicProperties:    item.MagicProperties,
			RequiresAttunement: item.RequiresAttunement,
			Rarity:             item.Rarity,
		})
		if err != nil {
			return LootPoolResult{}, fmt.Errorf("creating loot pool item: %w", err)
		}
		poolItems = append(poolItems, created)
	}

	return LootPoolResult{Pool: pool, Items: poolItems}, nil
}

// EligibleEncounter is a thin projection of refdata.Encounter for the loot
// pool widget. Captures just the fields the DM needs to pick an encounter.
type EligibleEncounter struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
}

// ListEligibleEncounters returns encounters in the given campaign that are
// eligible to own a loot pool (currently: status == "completed"). The slice
// is sorted by Encounter.UpdatedAt descending so the most recently finished
// encounter shows first in the dashboard dropdown.
func (s *Service) ListEligibleEncounters(ctx context.Context, campaignID uuid.UUID) ([]EligibleEncounter, error) {
	encs, err := s.store.ListEncountersByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("listing encounters: %w", err)
	}
	out := make([]EligibleEncounter, 0, len(encs))
	for _, e := range encs {
		if e.Status != "completed" {
			continue
		}
		dn := ""
		if e.DisplayName.Valid {
			dn = e.DisplayName.String
		}
		out = append(out, EligibleEncounter{
			ID:          e.ID,
			Name:        e.Name,
			DisplayName: dn,
			Status:      e.Status,
		})
	}
	return out, nil
}

// GetLootPool returns the loot pool for an encounter with all its items.
func (s *Service) GetLootPool(ctx context.Context, encounterID uuid.UUID) (LootPoolResult, error) {
	pool, err := s.store.GetLootPoolByEncounter(ctx, encounterID)
	if err != nil {
		return LootPoolResult{}, ErrPoolNotFound
	}
	items, err := s.store.ListLootPoolItems(ctx, pool.ID)
	if err != nil {
		return LootPoolResult{}, fmt.Errorf("listing loot pool items: %w", err)
	}
	return LootPoolResult{Pool: pool, Items: items}, nil
}

// AddItem adds a new item to the loot pool.
func (s *Service) AddItem(ctx context.Context, poolID uuid.UUID, item refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error) {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return refdata.LootPoolItem{}, ErrPoolNotFound
	}
	if pool.Status != "open" {
		return refdata.LootPoolItem{}, ErrPoolClosed
	}
	item.LootPoolID = poolID
	return s.store.CreateLootPoolItem(ctx, item)
}

// RemoveItem removes an item from the loot pool.
func (s *Service) RemoveItem(ctx context.Context, poolID uuid.UUID, itemID uuid.UUID) error {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return ErrPoolNotFound
	}
	if pool.Status != "open" {
		return ErrPoolClosed
	}
	return s.store.DeleteLootPoolItem(ctx, itemID)
}

// ClaimItem claims a loot pool item for a character. Adds the item to the character's inventory.
func (s *Service) ClaimItem(ctx context.Context, poolID uuid.UUID, itemID uuid.UUID, characterID uuid.UUID) (refdata.LootPoolItem, error) {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return refdata.LootPoolItem{}, ErrPoolNotFound
	}
	if pool.Status != "open" {
		return refdata.LootPoolItem{}, ErrPoolClosed
	}

	claimed, err := s.store.ClaimLootPoolItem(ctx, refdata.ClaimLootPoolItemParams{
		ID:        itemID,
		ClaimedBy: uuid.NullUUID{UUID: characterID, Valid: true},
	})
	if err != nil {
		return refdata.LootPoolItem{}, ErrItemAlreadyClaimed
	}

	// Add item to character's inventory
	char, err := s.store.GetCharacter(ctx, characterID)
	if err != nil {
		return claimed, fmt.Errorf("getting character: %w", err)
	}

	items, _ := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	newItem := character.InventoryItem{
		ItemID:             claimed.ItemID.String,
		Name:               claimed.Name,
		Quantity:           int(claimed.Quantity),
		Type:               claimed.Type,
		IsMagic:            claimed.IsMagic,
		MagicBonus:         int(claimed.MagicBonus),
		MagicProperties:    claimed.MagicProperties,
		RequiresAttunement: claimed.RequiresAttunement,
		Rarity:             claimed.Rarity,
	}

	items = inventory.AddItemQuantity(items, newItem, newItem.Quantity)

	invJSON, err := character.MarshalInventory(items)
	if err != nil {
		return claimed, fmt.Errorf("marshaling inventory: %w", err)
	}

	if _, err := s.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        characterID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		return claimed, fmt.Errorf("updating inventory: %w", err)
	}

	return claimed, nil
}

// SetGold sets the gold total for a loot pool.
func (s *Service) SetGold(ctx context.Context, poolID uuid.UUID, amount int32) (refdata.LootPool, error) {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return refdata.LootPool{}, ErrPoolNotFound
	}
	if pool.Status != "open" {
		return refdata.LootPool{}, ErrPoolClosed
	}
	return s.store.UpdateLootPoolGold(ctx, refdata.UpdateLootPoolGoldParams{
		ID:        poolID,
		GoldTotal: amount,
	})
}

// SplitGold divides the loot pool gold evenly among all approved party members.
func (s *Service) SplitGold(ctx context.Context, poolID uuid.UUID) (int32, error) {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return 0, ErrPoolNotFound
	}
	if pool.Status != "open" {
		return 0, ErrPoolClosed
	}
	if pool.GoldTotal <= 0 {
		return 0, nil
	}

	pcs, err := s.store.ListPlayerCharactersByCampaignApproved(ctx, pool.CampaignID)
	if err != nil {
		return 0, fmt.Errorf("listing party members: %w", err)
	}
	if len(pcs) == 0 {
		return 0, errors.New("no approved party members found")
	}

	share := pool.GoldTotal / int32(len(pcs))

	for _, pc := range pcs {
		newGold := pc.Gold + share
		if _, err := s.store.UpdateCharacterGold(ctx, refdata.UpdateCharacterGoldParams{
			ID:   pc.CharacterID,
			Gold: newGold,
		}); err != nil {
			return 0, fmt.Errorf("updating gold for %s: %w", pc.CharacterName, err)
		}
	}

	// Retain remainder in pool
	remainder := pool.GoldTotal % int32(len(pcs))
	if _, err := s.store.UpdateLootPoolGold(ctx, refdata.UpdateLootPoolGoldParams{
		ID:        poolID,
		GoldTotal: remainder,
	}); err != nil {
		return 0, fmt.Errorf("updating pool gold remainder: %w", err)
	}

	return share, nil
}

// ClearPool removes all unclaimed items and closes the pool.
func (s *Service) ClearPool(ctx context.Context, poolID uuid.UUID) error {
	pool, err := s.store.GetLootPool(ctx, poolID)
	if err != nil {
		return ErrPoolNotFound
	}
	if pool.Status != "open" {
		return ErrPoolClosed
	}

	if err := s.store.DeleteUnclaimedLootPoolItems(ctx, poolID); err != nil {
		return fmt.Errorf("deleting unclaimed items: %w", err)
	}

	if _, err := s.store.UpdateLootPoolStatus(ctx, refdata.UpdateLootPoolStatusParams{
		ID:     poolID,
		Status: "closed",
	}); err != nil {
		return fmt.Errorf("closing pool: %w", err)
	}

	return nil
}

// FormatLootAnnouncement formats the loot pool as a Discord message.
func FormatLootAnnouncement(pool refdata.LootPool, items []refdata.LootPoolItem) string {
	var parts []string

	for _, item := range items {
		if item.ClaimedBy.Valid {
			continue
		}
		if item.Quantity > 1 {
			parts = append(parts, fmt.Sprintf("%s \u00d7%d", item.Name, item.Quantity))
		} else {
			parts = append(parts, item.Name)
		}
	}

	if pool.GoldTotal > 0 {
		parts = append(parts, fmt.Sprintf("%d gp", pool.GoldTotal))
	}

	if len(parts) == 0 {
		return "\U0001f4b0 No loot available."
	}

	return fmt.Sprintf("\U0001f4b0 Loot available: %s\nType /loot to claim items", strings.Join(parts, ", "))
}
