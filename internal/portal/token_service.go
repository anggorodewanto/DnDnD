package portal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrTokenExpired is returned when a portal token has expired.
	ErrTokenExpired = errors.New("portal token expired")
	// ErrTokenUsed is returned when a portal token has already been used.
	ErrTokenUsed = errors.New("portal token already used")
)

// TokenRepository defines the persistence interface for portal tokens.
type TokenRepository interface {
	Create(ctx context.Context, token string, campaignID uuid.UUID, discordUserID, purpose string, expiresAt time.Time) (*PortalToken, error)
	GetByToken(ctx context.Context, token string) (*PortalToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

// TokenService manages portal token lifecycle.
type TokenService struct {
	store         TokenRepository
	generateToken func() (string, error)
}

// NewTokenService creates a new TokenService.
func NewTokenService(store TokenRepository) *TokenService {
	return &TokenService{
		store:         store,
		generateToken: defaultGenerateToken,
	}
}

// SetGenerateToken overrides the token generation function (for testing).
func (s *TokenService) SetGenerateToken(fn func() (string, error)) {
	s.generateToken = fn
}

// CreateToken generates a cryptographically random token, stores it, and returns the token string.
func (s *TokenService) CreateToken(ctx context.Context, campaignID uuid.UUID, discordUserID, purpose string, ttl time.Duration) (string, error) {
	tokenStr, err := s.generateToken()
	if err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	expiresAt := time.Now().Add(ttl)
	_, err = s.store.Create(ctx, tokenStr, campaignID, discordUserID, purpose, expiresAt)
	if err != nil {
		return "", fmt.Errorf("storing token: %w", err)
	}

	return tokenStr, nil
}

// ValidateToken checks that the token exists, is not expired, and is not used.
// Returns the token record on success.
func (s *TokenService) ValidateToken(ctx context.Context, token string) (*PortalToken, error) {
	tok, err := s.store.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if tok.Used {
		return nil, ErrTokenUsed
	}

	if time.Now().After(tok.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return tok, nil
}

// RedeemToken validates and marks the token as used (single-use).
func (s *TokenService) RedeemToken(ctx context.Context, token string) error {
	tok, err := s.ValidateToken(ctx, token)
	if err != nil {
		return err
	}

	return s.store.MarkUsed(ctx, tok.ID)
}

func defaultGenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}
