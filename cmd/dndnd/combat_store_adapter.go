package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// mainCombatStoreAdapter wraps *refdata.Queries to satisfy combat.Store.
// refdata.Queries is sqlc-generated and exposes all the CRUD we need, but a
// handful of combat.Store methods (UpdateCharacterInventory,
// UpdateCharacterGold) use positional args instead of the sqlc Params
// structs. The adapter bridges those signatures and leaves everything else as
// promoted embedded methods.
type mainCombatStoreAdapter struct {
	*refdata.Queries
}

// UpdateCharacterInventory bridges the combat.Store positional-arg signature
// to the sqlc-generated struct-params signature.
func (a *mainCombatStoreAdapter) UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
	_, err := a.Queries.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        id,
		Inventory: inventory,
	})
	return err
}

// UpdateCharacterGold bridges the combat.Store positional-arg signature to
// the sqlc-generated struct-params signature.
func (a *mainCombatStoreAdapter) UpdateCharacterGold(ctx context.Context, id uuid.UUID, gold int32) error {
	_, err := a.Queries.UpdateCharacterGold(ctx, refdata.UpdateCharacterGoldParams{
		ID:   id,
		Gold: gold,
	})
	return err
}

// noopNotifier is the fallback combat.Notifier used when DISCORD_BOT_TOKEN is
// unset. Turn-timer messages are silently dropped so the timer can still
// update DB state (nudge_sent_at, warning_sent_at, etc.) without needing a
// live Discord session.
type noopNotifier struct{}

func (noopNotifier) SendMessage(_ string, _ string) error { return nil }
