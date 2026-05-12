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

// UUIDToInt64 converts a UUID to an int64 by reading the first 8 bytes as a
// big-endian int64. PostgreSQL advisory-lock keys are bigint; the truncation
// from 128 → 64 bits leaves a 2^-64 collision probability per pair of
// concurrent turns. Two turns that happen to share their leading 8 bytes
// would serialise against each other (a false-positive lock), but the
// space is large enough that this is operationally negligible. We accept
// the truncation as a documented trade-off (Phase 27, F-17) rather than
// hash-folding all 16 bytes — the failure mode is "occasional spurious
// wait", never a missed lock.
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

// txContextKey is the unexported key used to stash a *sql.Tx in a
// context.Context. F-4: AcquireAndRun threads its lock-holding tx through the
// fn's context so callers can opt into running their writes on the same tx
// (e.g. via refdata.Queries.WithTx(tx)). Callers that don't opt in still
// benefit from serialization — peers block at the advisory-lock acquire
// until our tx commits/rolls back.
type txContextKey struct{}

// ContextWithTx returns a copy of ctx that carries tx. Use TxFromContext to
// retrieve it inside an AcquireAndRun callback.
func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext returns the *sql.Tx stashed in ctx by ContextWithTx, or nil
// when no tx is attached. AcquireAndRun callbacks that want to run their
// writes on the lock-holding tx call this and pass the returned tx to
// refdata.Queries.WithTx; callbacks that don't care can ignore it (writes
// will still be serialized via the held advisory lock).
func TxFromContext(ctx context.Context) *sql.Tx {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txContextKey{}).(*sql.Tx)
	return tx
}
