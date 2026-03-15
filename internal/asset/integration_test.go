package asset_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestAssetCRUD_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run")
	}

	testDB := sharedDB.AcquireDB(t)
	queries := refdata.New(testDB)
	ctx := context.Background()

	// Create a campaign first (FK dependency)
	campaign, err := queries.CreateCampaign(ctx, refdata.CreateCampaignParams{
		GuildID:  "guild-test",
		DmUserID: "dm-test",
		Name:     "Test Campaign",
	})
	require.NoError(t, err)

	// Create an asset
	asset, err := queries.CreateAsset(ctx, refdata.CreateAssetParams{
		CampaignID:   campaign.ID,
		Type:         "map_background",
		OriginalName: "dungeon.png",
		MimeType:     "image/png",
		ByteSize:     1024,
		StoragePath:  campaign.ID.String() + "/maps/" + uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Equal(t, campaign.ID, asset.CampaignID)
	assert.Equal(t, "map_background", asset.Type)
	assert.Equal(t, "dungeon.png", asset.OriginalName)
	assert.Equal(t, int64(1024), asset.ByteSize)

	// Get by ID
	got, err := queries.GetAssetByID(ctx, asset.ID)
	require.NoError(t, err)
	assert.Equal(t, asset.ID, got.ID)

	// List by campaign
	list, err := queries.ListAssetsByCampaignID(ctx, campaign.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = queries.DeleteAsset(ctx, asset.ID)
	require.NoError(t, err)

	// Verify gone
	list, err = queries.ListAssetsByCampaignID(ctx, campaign.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestAssetFK_BackgroundImage_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run")
	}

	testDB := sharedDB.AcquireDB(t)
	queries := refdata.New(testDB)
	ctx := context.Background()

	// Create a campaign
	campaign, err := queries.CreateCampaign(ctx, refdata.CreateCampaignParams{
		GuildID:  "guild-fk",
		DmUserID: "dm-fk",
		Name:     "FK Test Campaign",
	})
	require.NoError(t, err)

	// Create an asset
	asset, err := queries.CreateAsset(ctx, refdata.CreateAssetParams{
		CampaignID:   campaign.ID,
		Type:         "map_background",
		OriginalName: "bg.png",
		MimeType:     "image/png",
		ByteSize:     512,
		StoragePath:  campaign.ID.String() + "/maps/" + uuid.New().String(),
	})
	require.NoError(t, err)

	// Create a map referencing the asset as background_image_id
	m, err := queries.CreateMap(ctx, refdata.CreateMapParams{
		CampaignID:        campaign.ID,
		Name:              "Test Map",
		WidthSquares:      10,
		HeightSquares:     10,
		TiledJson:         []byte(`{"version": 1}`),
		BackgroundImageID: uuid.NullUUID{UUID: asset.ID, Valid: true},
	})
	require.NoError(t, err)
	assert.True(t, m.BackgroundImageID.Valid)
	assert.Equal(t, asset.ID, m.BackgroundImageID.UUID)

	// Creating a map with a bogus background_image_id should fail (FK constraint)
	_, err = queries.CreateMap(ctx, refdata.CreateMapParams{
		CampaignID:        campaign.ID,
		Name:              "Bad Map",
		WidthSquares:      5,
		HeightSquares:     5,
		TiledJson:         []byte(`{"version": 1}`),
		BackgroundImageID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
	})
	assert.Error(t, err, "should fail FK constraint on background_image_id")
}
