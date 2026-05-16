package combat

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// storeAdapter wraps *refdata.Queries to satisfy Store. The sqlc-generated
// Queries already covers the vast majority of Store via promoted embedded
// methods; only UpdateCharacterInventory and UpdateCharacterGold need a manual
// bridge because Store takes positional args where sqlc uses Params structs.
type storeAdapter struct {
	*refdata.Queries

	// F-H05: in-process lair action persistence until a DB column is added.
	lairMu          sync.Mutex
	lastLairActions map[uuid.UUID]string

	// F-H06: in-process legendary budget persistence.
	legendaryMu      sync.Mutex
	legendaryBudgets map[uuid.UUID]int
}

// NewStoreAdapter returns a Store backed by *refdata.Queries. Both the main
// binary and integration tests should use this helper so the bridging code
// lives in exactly one place.
func NewStoreAdapter(q *refdata.Queries) Store {
	return &storeAdapter{Queries: q, lastLairActions: make(map[uuid.UUID]string), legendaryBudgets: make(map[uuid.UUID]int)}
}

func (a *storeAdapter) UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
	_, err := a.Queries.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        id,
		Inventory: inventory,
	})
	return err
}

func (a *storeAdapter) UpdateCharacterGold(ctx context.Context, id uuid.UUID, gold int32) error {
	_, err := a.Queries.UpdateCharacterGold(ctx, refdata.UpdateCharacterGoldParams{
		ID:   id,
		Gold: gold,
	})
	return err
}

// GetEncounterTemplate bridges the combat Store interface (which takes a plain UUID)
// to the campaign-scoped sqlc query. Combat lookups use the unchecked variant
// because the encounter is already validated at session start.
func (a *storeAdapter) GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	return a.Queries.GetEncounterTemplateUnchecked(ctx, id)
}

func (a *storeAdapter) GetMapByIDUnchecked(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return a.Queries.GetMapByIDUnchecked(ctx, id)
}

func (a *storeAdapter) SetLastLairAction(_ context.Context, encounterID uuid.UUID, action string) error {
	a.lairMu.Lock()
	defer a.lairMu.Unlock()
	a.lastLairActions[encounterID] = action
	return nil
}

func (a *storeAdapter) GetLastLairAction(_ context.Context, encounterID uuid.UUID) (string, error) {
	a.lairMu.Lock()
	defer a.lairMu.Unlock()
	return a.lastLairActions[encounterID], nil
}

func (a *storeAdapter) GetLegendaryBudget(_ context.Context, combatantID uuid.UUID) (int, error) {
	a.legendaryMu.Lock()
	defer a.legendaryMu.Unlock()
	budget, ok := a.legendaryBudgets[combatantID]
	if !ok {
		return -1, nil // -1 signals uninitialized; handler will use creature's default
	}
	return budget, nil
}

func (a *storeAdapter) DecrementLegendaryBudget(_ context.Context, combatantID uuid.UUID) error {
	a.legendaryMu.Lock()
	defer a.legendaryMu.Unlock()
	a.legendaryBudgets[combatantID]--
	return nil
}
