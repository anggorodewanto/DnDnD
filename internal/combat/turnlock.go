package combat

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrLockTimeout is returned when the advisory lock cannot be acquired within the timeout.
var ErrLockTimeout = errors.New("lock timeout: server is busy, please try again")

// TxBeginner abstracts the ability to begin a database transaction.
type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// UUIDToInt64 converts a UUID to an int64 by reading the first 8 bytes as a big-endian int64.
// This provides sufficient uniqueness for PostgreSQL advisory lock keys.
func UUIDToInt64(id uuid.UUID) int64 {
	return int64(binary.BigEndian.Uint64(id[:8]))
}

// AcquireTurnLock begins a transaction, sets a 5-second lock timeout,
// and acquires a PostgreSQL advisory lock keyed on the turn_id.
// The lock is automatically released when the returned transaction is committed or rolled back.
// Returns ErrLockTimeout if the lock cannot be acquired within 5 seconds.
func AcquireTurnLock(ctx context.Context, db TxBeginner, turnID uuid.UUID) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("setting lock timeout: %w", err)
	}

	lockKey := UUIDToInt64(turnID)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		tx.Rollback()
		if isLockTimeoutError(err) {
			return nil, ErrLockTimeout
		}
		return nil, fmt.Errorf("acquiring advisory lock: %w", err)
	}

	return tx, nil
}

// isLockTimeoutError checks if an error is a PostgreSQL lock timeout error (SQLSTATE 55P03).
func isLockTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "55P03"
	}
	return false
}
