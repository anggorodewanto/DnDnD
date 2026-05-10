package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/portal"
)

// fakePortalTokenRepo is a memory-backed portal.TokenRepository so the
// wiring test can prove that newPortalTokenIssuer produces a real token
// (not the legacy "e2e-token" placeholder) and that the same TokenService
// validates it round-trip.
type fakePortalTokenRepo struct {
	created   []*portal.PortalToken
	createErr error
	getErr    error
	markErr   error
	rows      map[string]*portal.PortalToken
}

func newFakePortalTokenRepo() *fakePortalTokenRepo {
	return &fakePortalTokenRepo{rows: map[string]*portal.PortalToken{}}
}

func (f *fakePortalTokenRepo) Create(_ context.Context, token string, campaignID uuid.UUID, discordUserID, purpose string, expiresAt time.Time) (*portal.PortalToken, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	tok := &portal.PortalToken{
		ID:            uuid.New(),
		Token:         token,
		CampaignID:    campaignID,
		DiscordUserID: discordUserID,
		Purpose:       purpose,
		ExpiresAt:     expiresAt,
		CreatedAt:     time.Now(),
	}
	f.created = append(f.created, tok)
	f.rows[token] = tok
	return tok, nil
}

func (f *fakePortalTokenRepo) GetByToken(_ context.Context, token string) (*portal.PortalToken, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	tok, ok := f.rows[token]
	if !ok {
		return nil, portal.ErrTokenNotFound
	}
	return tok, nil
}

func (f *fakePortalTokenRepo) MarkUsed(_ context.Context, id uuid.UUID) error {
	if f.markErr != nil {
		return f.markErr
	}
	for _, tok := range f.rows {
		if tok.ID == id {
			tok.Used = true
		}
	}
	return nil
}

// TestNewPortalTokenIssuer_ProducesRealToken_NotPlaceholder is the regression
// test for crit-06: the production TokenFunc must call portal.TokenService
// instead of returning the literal "e2e-token". A real call must persist a
// row in the token repo with the right campaign / user / purpose.
func TestNewPortalTokenIssuer_ProducesRealToken_NotPlaceholder(t *testing.T) {
	repo := newFakePortalTokenRepo()
	svc := portal.NewTokenService(repo)
	ctx := context.Background()

	issue := newPortalTokenIssuer(ctx, svc)

	campaignID := uuid.New()
	token, err := issue(campaignID, "user-7")
	require.NoError(t, err)
	assert.NotEqual(t, "e2e-token", token, "production TokenFunc must not return the e2e placeholder")
	assert.NotEmpty(t, token)

	require.Len(t, repo.created, 1)
	assert.Equal(t, campaignID, repo.created[0].CampaignID)
	assert.Equal(t, "user-7", repo.created[0].DiscordUserID)
	assert.Equal(t, "create_character", repo.created[0].Purpose)
	// 24h TTL per Phase 91a spec; allow a small clock-skew window.
	delta := time.Until(repo.created[0].ExpiresAt)
	assert.InDelta(t, 24*time.Hour, delta, float64(time.Minute))
}

// TestNewPortalTokenIssuer_RoundTripsThroughValidator proves the same
// TokenService that issues tokens also validates them — i.e. the validator
// passed to portal.NewHandler will accept tokens minted by the TokenFunc
// the registration handler hands to the player.
func TestNewPortalTokenIssuer_RoundTripsThroughValidator(t *testing.T) {
	repo := newFakePortalTokenRepo()
	svc := portal.NewTokenService(repo)
	ctx := context.Background()
	issue := newPortalTokenIssuer(ctx, svc)

	campaignID := uuid.New()
	token, err := issue(campaignID, "user-9")
	require.NoError(t, err)

	got, err := svc.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, campaignID, got.CampaignID)
	assert.Equal(t, "user-9", got.DiscordUserID)
}

// TestNewPortalTokenIssuer_PropagatesServiceError confirms the lambda does
// not swallow errors from the token store — the registration handler relies
// on a non-nil error to surface "Error generating portal link".
func TestNewPortalTokenIssuer_PropagatesServiceError(t *testing.T) {
	repo := newFakePortalTokenRepo()
	repo.createErr = portal.ErrTokenNotFound // any non-nil error
	svc := portal.NewTokenService(repo)
	issue := newPortalTokenIssuer(context.Background(), svc)

	_, err := issue(uuid.New(), "user-7")
	require.Error(t, err)
}
