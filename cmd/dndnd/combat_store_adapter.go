package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// combatStoreAdapter wraps *refdata.Queries to satisfy combat.Store. The sqlc
// generated Queries already covers the vast majority of combat.Store via
// promoted embedded methods; only UpdateCharacterInventory and
// UpdateCharacterGold need a manual bridge because combat.Store takes
// positional args where sqlc uses Params structs.
type combatStoreAdapter struct {
	*refdata.Queries
}

func (a *combatStoreAdapter) UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
	_, err := a.Queries.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        id,
		Inventory: inventory,
	})
	return err
}

func (a *combatStoreAdapter) UpdateCharacterGold(ctx context.Context, id uuid.UUID, gold int32) error {
	_, err := a.Queries.UpdateCharacterGold(ctx, refdata.UpdateCharacterGoldParams{
		ID:   id,
		Gold: gold,
	})
	return err
}
