package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Session represents a server-side session stored in PostgreSQL.
type Session struct {
	ID             uuid.UUID
	DiscordUserID  string
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      time.Time
}

// SessionStore manages session persistence in PostgreSQL.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

// Create inserts a new session and returns it.
func (s *SessionStore) Create(ctx context.Context, discordUserID, accessToken, refreshToken string, tokenExpiresAt *time.Time) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sessions (discord_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, discord_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at, expires_at`,
		discordUserID, accessToken, refreshToken, tokenExpiresAt,
	).Scan(&sess.ID, &sess.DiscordUserID, &sess.AccessToken, &sess.RefreshToken,
		&sess.TokenExpiresAt, &sess.CreatedAt, &sess.UpdatedAt, &sess.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return sess, nil
}

// ErrSessionNotFound is returned when a session does not exist or is expired.
var ErrSessionNotFound = errors.New("session not found")

// GetByID retrieves a session by ID. Returns ErrSessionNotFound if the session
// does not exist or has expired.
func (s *SessionStore) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, discord_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at, expires_at
		 FROM sessions
		 WHERE id = $1 AND expires_at > now()`, id,
	).Scan(&sess.ID, &sess.DiscordUserID, &sess.AccessToken, &sess.RefreshToken,
		&sess.TokenExpiresAt, &sess.CreatedAt, &sess.UpdatedAt, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	return sess, nil
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// SlideTTL updates the session's expires_at to now + 30 days and refreshes updated_at.
func (s *SessionStore) SlideTTL(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = now() + INTERVAL '30 days', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("sliding session TTL: %w", err)
	}
	return nil
}

// UpdateTokens updates the access token, refresh token, and token expiry for a session.
func (s *SessionStore) UpdateTokens(ctx context.Context, id uuid.UUID, accessToken, refreshToken string, tokenExpiresAt *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET access_token = $2, refresh_token = $3, token_expires_at = $4, updated_at = now() WHERE id = $1`,
		id, accessToken, refreshToken, tokenExpiresAt)
	if err != nil {
		return fmt.Errorf("updating session tokens: %w", err)
	}
	return nil
}

// ExpireSession sets a session's expires_at to the past (for testing).
func (s *SessionStore) ExpireSession(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = now() - INTERVAL '1 second' WHERE id = $1`, id)
	return err
}
