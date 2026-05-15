package portal_test

import (
	"context"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/portal"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTokenService(t *testing.T) (*portal.TokenService, *portal.TokenStore) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	store := portal.NewTokenStore(db)
	svc := portal.NewTokenService(store)
	return svc, store
}

func seedCampaignForService(t *testing.T) uuid.UUID {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	var id uuid.UUID
	err := db.QueryRow(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-svc-"+uuid.NewString()[:8], "dm-user", "Test Campaign").Scan(&id)
	require.NoError(t, err)
	return id
}

func TestTokenService_CreateToken(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-111", "create_character", 24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	// Token should be long enough (cryptographically random)
	assert.True(t, len(token) >= 32, "token should be at least 32 chars, got %d", len(token))
}

func TestTokenService_ValidateToken(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-222", "create_character", 24*time.Hour)
	require.NoError(t, err)

	tok, err := svc.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, campaignID, tok.CampaignID)
	assert.Equal(t, "user-222", tok.DiscordUserID)
	assert.Equal(t, "create_character", tok.Purpose)
	assert.False(t, tok.Used)
}

func TestTokenService_ValidateToken_Expired(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	// Create a token that's already expired (negative TTL)
	token, err := svc.CreateToken(ctx, campaignID, "user-333", "create_character", -1*time.Hour)
	require.NoError(t, err)

	_, err = svc.ValidateToken(ctx, token)
	assert.ErrorIs(t, err, portal.ErrTokenExpired)
}

func TestTokenService_ValidateToken_Used(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-444", "create_character", 24*time.Hour)
	require.NoError(t, err)

	err = svc.RedeemToken(ctx, token)
	require.NoError(t, err)

	_, err = svc.ValidateToken(ctx, token)
	assert.ErrorIs(t, err, portal.ErrTokenUsed)
}

func TestTokenService_ValidateToken_NotFound(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()

	_, err := svc.ValidateToken(ctx, "nonexistent")
	assert.ErrorIs(t, err, portal.ErrTokenNotFound)
}

func TestTokenService_RedeemToken(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-555", "create_character", 24*time.Hour)
	require.NoError(t, err)

	err = svc.RedeemToken(ctx, token)
	require.NoError(t, err)

	// Second redeem should fail
	err = svc.RedeemToken(ctx, token)
	assert.ErrorIs(t, err, portal.ErrTokenUsed)
}

func TestTokenService_RedeemToken_Expired(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-666", "create_character", -1*time.Hour)
	require.NoError(t, err)

	err = svc.RedeemToken(ctx, token)
	assert.ErrorIs(t, err, portal.ErrTokenExpired)
}

func TestTokenService_RedeemToken_NotFound(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()

	err := svc.RedeemToken(ctx, "nonexistent-redeem")
	assert.ErrorIs(t, err, portal.ErrTokenNotFound)
}

func TestTokenService_RedeemToken_ConcurrentDoubleRedeem(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token, err := svc.CreateToken(ctx, campaignID, "user-race", "create_character", 24*time.Hour)
	require.NoError(t, err)

	// Run two concurrent redemptions — exactly one must fail with ErrTokenUsed.
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			errs <- svc.RedeemToken(ctx, token)
		}()
	}

	err1 := <-errs
	err2 := <-errs

	// One succeeds, one fails
	if err1 == nil && err2 == nil {
		t.Fatal("both concurrent RedeemToken calls succeeded; expected one to fail with ErrTokenUsed")
	}
	if err1 != nil && err2 != nil {
		t.Fatalf("both concurrent RedeemToken calls failed: err1=%v, err2=%v", err1, err2)
	}

	// The failing one must be ErrTokenUsed
	failing := err1
	if failing == nil {
		failing = err2
	}
	assert.ErrorIs(t, failing, portal.ErrTokenUsed)
}
