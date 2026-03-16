package refdata

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// TxBeginner is satisfied by *sql.DB and *sql.Tx (via nested transactions in some drivers).
type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// UpdateTwoCharacterInventories atomically updates the inventories of two characters
// using a database transaction to ensure both succeed or both fail.
func (q *Queries) UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error {
	beginner, ok := q.db.(TxBeginner)
	if !ok {
		return fmt.Errorf("database connection does not support transactions")
	}

	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := q.WithTx(tx)

	if _, err := qtx.UpdateCharacterInventory(ctx, UpdateCharacterInventoryParams{
		ID:        id1,
		Inventory: inv1,
	}); err != nil {
		return fmt.Errorf("updating first character inventory: %w", err)
	}

	if _, err := qtx.UpdateCharacterInventory(ctx, UpdateCharacterInventoryParams{
		ID:        id2,
		Inventory: inv2,
	}); err != nil {
		return fmt.Errorf("updating second character inventory: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
