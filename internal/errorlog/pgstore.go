package errorlog

import (
	"context"
	"database/sql"
	"time"
)

// PgStore persists error entries into the dedicated error_log table. Phase 119
// migration 20260427120001 splits errors out of action_log, replacing the
// Phase 112 JSONB-overload approach with first-class columns (command,
// user_id, summary, error_detail, created_at). This keeps action_log
// combat-only and lets error queries skip the action_type='error' filter.
type PgStore struct {
	db *sql.DB
}

// NewPgStore returns a PgStore bound to db. A nil db returns nil so callers
// can fall back to MemoryStore without branching.
func NewPgStore(db *sql.DB) *PgStore {
	if db == nil {
		return nil
	}
	return &PgStore{db: db}
}

// Record inserts a new error row into error_log. CreatedAt on entry is
// ignored — the column is omitted from the INSERT and the DB default (now())
// fills it in.
func (p *PgStore) Record(ctx context.Context, entry Entry) error {
	q, args := buildInsertErrorQuery(entry)
	_, err := p.db.ExecContext(ctx, q, args...)
	return err
}

// CountSince returns the number of error_log rows created at or after since.
func (p *PgStore) CountSince(ctx context.Context, since time.Time) (int, error) {
	q, args := buildCountSinceQuery(since)
	var count int
	if err := p.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ListRecent returns up to limit error rows, most recent first.
func (p *PgStore) ListRecent(ctx context.Context, limit int) ([]Entry, error) {
	q, args := buildListRecentQuery(limit)
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Entry, 0, limit)
	for rows.Next() {
		var (
			command string
			userID  string
			summary string
			created time.Time
		)
		if err := rows.Scan(&command, &userID, &summary, &created); err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Command:   command,
			UserID:    userID,
			Summary:   summary,
			CreatedAt: created,
		})
	}
	return out, rows.Err()
}

// buildInsertErrorQuery returns the INSERT SQL and its bound args for a
// single error Entry. user_id is stored as NULL when empty so the column's
// "nullable: system-context errors have no user" semantics hold.
func buildInsertErrorQuery(entry Entry) (string, []any) {
	q := `INSERT INTO error_log (command, user_id, summary)
VALUES ($1, NULLIF($2, ''), $3)`
	return q, []any{entry.Command, entry.UserID, entry.Summary}
}

// buildCountSinceQuery returns the COUNT SQL for errors since the given time.
func buildCountSinceQuery(since time.Time) (string, []any) {
	return `SELECT COUNT(*) FROM error_log WHERE created_at >= $1`,
		[]any{since}
}

// buildListRecentQuery returns the SELECT SQL for the most-recent errors.
// user_id is COALESCEd to '' so the Go side can scan into a plain string.
func buildListRecentQuery(limit int) (string, []any) {
	q := `SELECT command, COALESCE(user_id, '') AS user_id, summary, created_at
FROM error_log
ORDER BY created_at DESC
LIMIT $1`
	return q, []any{limit}
}
