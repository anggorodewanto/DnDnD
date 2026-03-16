package refdata

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// UpdateTwoCharacterInventories atomically updates the inventories of two characters
// in a single SQL statement, ensuring both succeed or both fail.
func (q *Queries) UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error {
	const query = `UPDATE characters SET
		inventory = CASE id WHEN $1 THEN $2 WHEN $3 THEN $4 END,
		updated_at = now()
	WHERE id IN ($1, $3)`

	result, err := q.db.ExecContext(ctx, query, id1, inv1, id2, inv2)
	if err != nil {
		return fmt.Errorf("updating two character inventories: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows != 2 {
		return fmt.Errorf("expected 2 rows affected, got %d", rows)
	}
	return nil
}
