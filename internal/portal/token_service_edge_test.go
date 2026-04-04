package portal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenService_CreateToken_GenerationError(t *testing.T) {
	svc, _ := newTestTokenService(t)
	svc.SetGenerateToken(func() (string, error) {
		return "", errors.New("rng failure")
	})

	_, err := svc.CreateToken(context.Background(), uuid.New(), "user-999", "create_character", 24*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generating token")
}

func TestTokenService_CreateToken_UniqueTokens(t *testing.T) {
	svc, _ := newTestTokenService(t)
	ctx := context.Background()
	campaignID := seedCampaignForService(t)

	token1, err := svc.CreateToken(ctx, campaignID, "user-aaa", "create_character", 24*time.Hour)
	require.NoError(t, err)

	token2, err := svc.CreateToken(ctx, campaignID, "user-bbb", "create_character", 24*time.Hour)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2)
}
