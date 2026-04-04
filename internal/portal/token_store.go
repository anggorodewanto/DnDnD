package portal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrTokenNotFound is returned when a portal token does not exist.
var ErrTokenNotFound = errors.New("portal token not found")

// PortalToken represents a one-time portal access token.
type PortalToken struct {
	ID            uuid.UUID
	Token         string
	CampaignID    uuid.UUID
	DiscordUserID string
	Purpose       string
	Used          bool
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// TokenStore manages portal token persistence in PostgreSQL.
type TokenStore struct {
	db *sql.DB
}

// NewTokenStore creates a new TokenStore.
func NewTokenStore(db *sql.DB) *TokenStore {
	return &TokenStore{db: db}
}

const tokenColumns = `id, token, campaign_id, discord_user_id, purpose, used, expires_at, created_at`

func scanToken(row interface{ Scan(dest ...any) error }) (*PortalToken, error) {
	tok := &PortalToken{}
	err := row.Scan(&tok.ID, &tok.Token, &tok.CampaignID, &tok.DiscordUserID,
		&tok.Purpose, &tok.Used, &tok.ExpiresAt, &tok.CreatedAt)
	if err != nil {
		return nil, err
	}
	return tok, nil
}

// Create inserts a new portal token and returns it.
func (s *TokenStore) Create(ctx context.Context, token string, campaignID uuid.UUID, discordUserID, purpose string, expiresAt time.Time) (*PortalToken, error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO portal_tokens (token, campaign_id, discord_user_id, purpose, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+tokenColumns,
		token, campaignID, discordUserID, purpose, expiresAt,
	)
	tok, err := scanToken(row)
	if err != nil {
		return nil, fmt.Errorf("creating portal token: %w", err)
	}
	return tok, nil
}

// GetByToken retrieves a portal token by its token string.
func (s *TokenStore) GetByToken(ctx context.Context, token string) (*PortalToken, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+tokenColumns+` FROM portal_tokens WHERE token = $1`, token,
	)
	tok, err := scanToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting portal token: %w", err)
	}
	return tok, nil
}

// MarkUsed sets the token's used flag to true.
func (s *TokenStore) MarkUsed(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE portal_tokens SET used = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("marking portal token used: %w", err)
	}
	return nil
}
