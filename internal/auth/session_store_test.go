package auth_test

import (
	"context"
	"testing"
	"time"

	db "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSessionStore(t *testing.T) *auth.SessionStore {
	t.Helper()
	testDB := testutil.NewMigratedTestDB(t, db.Migrations)
	return auth.NewSessionStore(testDB)
}

func TestSessionStore_Create(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	sess, err := store.Create(ctx, "user123", "access_tok", "refresh_tok", &expires)
	require.NoError(t, err)
	require.NotNil(t, sess)

	assert.NotEqual(t, sess.ID.String(), "00000000-0000-0000-0000-000000000000")
	assert.Equal(t, "user123", sess.DiscordUserID)
	assert.Equal(t, "access_tok", sess.AccessToken)
	assert.Equal(t, "refresh_tok", sess.RefreshToken)
	assert.NotNil(t, sess.TokenExpiresAt)
	assert.WithinDuration(t, expires, *sess.TokenExpiresAt, time.Second)
	assert.WithinDuration(t, time.Now(), sess.CreatedAt, 5*time.Second)
	assert.WithinDuration(t, time.Now().Add(30*24*time.Hour), sess.ExpiresAt, 5*time.Second)
}

func TestSessionStore_GetByID(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	created, err := store.Create(ctx, "user456", "at", "rt", &expires)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "user456", got.DiscordUserID)
}

func TestSessionStore_GetByID_NotFound(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()

	got, err := store.GetByID(ctx, uuid.New())
	assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	assert.Nil(t, got)
}

func TestSessionStore_GetByID_Expired(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	created, err := store.Create(ctx, "user789", "at", "rt", &expires)
	require.NoError(t, err)

	// Manually expire the session
	store.ExpireSession(ctx, created.ID)

	got, err := store.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	assert.Nil(t, got)
}

func TestSessionStore_Delete(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	created, err := store.Create(ctx, "user_del", "at", "rt", &expires)
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	assert.Nil(t, got)
}

func TestSessionStore_SlideTTL(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	created, err := store.Create(ctx, "user_slide", "at", "rt", &expires)
	require.NoError(t, err)

	err = store.SlideTTL(ctx, created.ID)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	// ExpiresAt should be close to now + 30 days
	assert.WithinDuration(t, time.Now().Add(30*24*time.Hour), got.ExpiresAt, 5*time.Second)
	// UpdatedAt should be refreshed
	assert.True(t, got.UpdatedAt.After(created.UpdatedAt) || got.UpdatedAt.Equal(created.UpdatedAt))
}

func TestSessionStore_UpdateTokens(t *testing.T) {
	store := newTestSessionStore(t)
	ctx := context.Background()
	expires := time.Now().Add(7 * 24 * time.Hour)

	created, err := store.Create(ctx, "user_tok", "old_at", "old_rt", &expires)
	require.NoError(t, err)

	newExpires := time.Now().Add(14 * 24 * time.Hour)
	err = store.UpdateTokens(ctx, created.ID, "new_at", "new_rt", &newExpires)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_at", got.AccessToken)
	assert.Equal(t, "new_rt", got.RefreshToken)
	assert.WithinDuration(t, newExpires, *got.TokenExpiresAt, time.Second)
}
