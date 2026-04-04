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

func seedCampaign(t *testing.T) uuid.UUID {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	var id uuid.UUID
	err := db.QueryRow(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-"+uuid.NewString()[:8], "dm-user", "Test Campaign").Scan(&id)
	require.NoError(t, err)
	return id
}

func newTestTokenStore(t *testing.T) *portal.TokenStore {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	return portal.NewTokenStore(db)
}

func TestTokenStore_Create(t *testing.T) {
	db := sharedDB.AcquireDB(t)
	store := portal.NewTokenStore(db)
	ctx := context.Background()

	// Seed a campaign
	var campaignID uuid.UUID
	err := db.QueryRow(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-create", "dm-user", "Test Campaign").Scan(&campaignID)
	require.NoError(t, err)

	expiresAt := time.Now().Add(24 * time.Hour)
	tok, err := store.Create(ctx, "test-token-abc", campaignID, "user-123", "create_character", expiresAt)
	require.NoError(t, err)
	require.NotNil(t, tok)

	assert.NotEqual(t, uuid.Nil, tok.ID)
	assert.Equal(t, "test-token-abc", tok.Token)
	assert.Equal(t, campaignID, tok.CampaignID)
	assert.Equal(t, "user-123", tok.DiscordUserID)
	assert.Equal(t, "create_character", tok.Purpose)
	assert.False(t, tok.Used)
	assert.WithinDuration(t, expiresAt, tok.ExpiresAt, time.Second)
	assert.WithinDuration(t, time.Now(), tok.CreatedAt, 5*time.Second)
}

func TestTokenStore_GetByToken(t *testing.T) {
	db := sharedDB.AcquireDB(t)
	store := portal.NewTokenStore(db)
	ctx := context.Background()

	var campaignID uuid.UUID
	err := db.QueryRow(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-get", "dm-user", "Test Campaign").Scan(&campaignID)
	require.NoError(t, err)

	expiresAt := time.Now().Add(24 * time.Hour)
	created, err := store.Create(ctx, "get-token-xyz", campaignID, "user-456", "create_character", expiresAt)
	require.NoError(t, err)

	got, err := store.GetByToken(ctx, "get-token-xyz")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "get-token-xyz", got.Token)
}

func TestTokenStore_GetByToken_NotFound(t *testing.T) {
	store := newTestTokenStore(t)
	ctx := context.Background()

	_, err := store.GetByToken(ctx, "nonexistent-token")
	assert.ErrorIs(t, err, portal.ErrTokenNotFound)
}

func TestTokenStore_MarkUsed(t *testing.T) {
	db := sharedDB.AcquireDB(t)
	store := portal.NewTokenStore(db)
	ctx := context.Background()

	var campaignID uuid.UUID
	err := db.QueryRow(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-used", "dm-user", "Test Campaign").Scan(&campaignID)
	require.NoError(t, err)

	expiresAt := time.Now().Add(24 * time.Hour)
	created, err := store.Create(ctx, "used-token-123", campaignID, "user-789", "create_character", expiresAt)
	require.NoError(t, err)

	err = store.MarkUsed(ctx, created.ID)
	require.NoError(t, err)

	got, err := store.GetByToken(ctx, "used-token-123")
	require.NoError(t, err)
	assert.True(t, got.Used)
}
