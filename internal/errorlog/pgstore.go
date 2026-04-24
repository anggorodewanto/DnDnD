package errorlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// PgStore persists error entries into the shared action_log table using
// action_type='error'. Phase 112 migration 20260424120001 drops the NOT NULL
// constraints on turn_id, encounter_id, and actor_id so errors originating
// outside of combat (e.g. /register, dashboard requests, PNG rendering) can
// be stored alongside the combat log.
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

// Record inserts a new error row into action_log. If CreatedAt is zero, the
// DB default (now()) applies because we omit the column from the INSERT.
func (p *PgStore) Record(ctx context.Context, entry Entry) error {
	q, args := buildInsertErrorQuery(entry)
	_, err := p.db.ExecContext(ctx, q, args...)
	return err
}

// CountSince returns the number of action_log rows with action_type='error'
// created at or after since.
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
			command sql.NullString
			userID  sql.NullString
			summary sql.NullString
			created time.Time
		)
		if err := rows.Scan(&command, &userID, &summary, &created); err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Command:   command.String,
			UserID:    userID.String,
			Summary:   summary.String,
			CreatedAt: created,
		})
	}
	return out, rows.Err()
}

// buildInsertErrorQuery returns the INSERT SQL and its bound args for a
// single error Entry. Columns unused by errors (turn_id, encounter_id,
// actor_id, target_id, dice_rolls) are omitted so the DB defaults / NULL
// apply. command + user_id ride in before_state so the panel can reconstruct
// "on /cmd by @user" without a schema extension:
//   - description  → entry.Summary
//   - before_state → {"command": ..., "user_id": ...} JSONB (phase-112 tag)
//   - after_state  → {} (NOT NULL default)
func buildInsertErrorQuery(entry Entry) (string, []any) {
	q := `INSERT INTO action_log (action_type, description, before_state, after_state)
VALUES ($1, $2, $3::jsonb, $4::jsonb)`
	before, _ := json.Marshal(map[string]string{
		"command": entry.Command,
		"user_id": entry.UserID,
	})
	return q, []any{"error", entry.Summary, string(before), "{}"}
}

// buildCountSinceQuery returns the COUNT SQL for errors since the given time.
func buildCountSinceQuery(since time.Time) (string, []any) {
	return `SELECT COUNT(*) FROM action_log WHERE action_type = 'error' AND created_at >= $1`,
		[]any{since}
}

// buildListRecentQuery returns the SELECT SQL for the most-recent errors.
// Columns: command (before_state->>command), user_id (before_state->>user_id),
// summary (description), created_at.
func buildListRecentQuery(limit int) (string, []any) {
	q := `SELECT
    COALESCE(before_state->>'command', '') AS command,
    COALESCE(before_state->>'user_id', '') AS user_id,
    COALESCE(description, '') AS summary,
    created_at
FROM action_log
WHERE action_type = 'error'
ORDER BY created_at DESC
LIMIT $1`
	return q, []any{limit}
}
