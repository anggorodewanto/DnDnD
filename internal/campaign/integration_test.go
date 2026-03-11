package campaign_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.NewMigratedTestDB(t, dbfs.Migrations)
}

func setupService(t *testing.T) (*campaign.Service, *sql.DB) {
	t.Helper()
	db := setupTestDB(t)
	queries := refdata.New(db)
	svc := campaign.NewService(queries, nil)
	return svc, db
}

func TestIntegration_CreateAndGetCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-100", "dm-1", "Heroes of the Realm", nil)
	require.NoError(t, err)
	assert.Equal(t, "guild-100", c.GuildID)
	assert.Equal(t, "dm-1", c.DmUserID)
	assert.Equal(t, "Heroes of the Realm", c.Name)
	assert.Equal(t, "active", c.Status)
	assert.NotEqual(t, uuid.Nil, c.ID)

	// Default settings should be applied
	var settings campaign.Settings
	err = json.Unmarshal(c.Settings.RawMessage, &settings)
	require.NoError(t, err)
	assert.Equal(t, 24, settings.TurnTimeoutHours)
	assert.Equal(t, "standard", settings.DiagonalRule)

	// Get by guild ID
	got, err := svc.GetByGuildID(ctx, "guild-100")
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)

	// Get by ID
	got2, err := svc.GetByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, got2.ID)
}

func TestIntegration_CreateCampaign_DuplicateGuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	_, err := svc.CreateCampaign(ctx, "guild-200", "dm-1", "First Campaign", nil)
	require.NoError(t, err)

	// Second campaign for same guild should fail (UNIQUE constraint)
	_, err = svc.CreateCampaign(ctx, "guild-200", "dm-2", "Second Campaign", nil)
	require.Error(t, err)
}

func TestIntegration_CreateCampaign_WithSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	settings := &campaign.Settings{
		TurnTimeoutHours: 48,
		DiagonalRule:     "variant",
		Open5eSources:    []string{"srd", "phb"},
	}
	c, err := svc.CreateCampaign(ctx, "guild-300", "dm-1", "Custom Campaign", settings)
	require.NoError(t, err)

	var parsed campaign.Settings
	err = json.Unmarshal(c.Settings.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 48, parsed.TurnTimeoutHours)
	assert.Equal(t, "variant", parsed.DiagonalRule)
	assert.Equal(t, []string{"srd", "phb"}, parsed.Open5eSources)
}

func TestIntegration_PauseResumeCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-400", "dm-1", "Pause Test", nil)
	require.NoError(t, err)
	assert.Equal(t, "active", c.Status)

	// Pause
	paused, err := svc.PauseCampaign(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "paused", paused.Status)

	// Verify persisted
	got, err := svc.GetByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "paused", got.Status)

	// Resume
	resumed, err := svc.ResumeCampaign(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", resumed.Status)
}

func TestIntegration_ArchiveCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-500", "dm-1", "Archive Test", nil)
	require.NoError(t, err)

	archived, err := svc.ArchiveCampaign(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", archived.Status)

	// Cannot pause archived campaign
	_, err = svc.PauseCampaign(ctx, c.ID)
	require.Error(t, err)

	// Cannot resume archived campaign
	_, err = svc.ResumeCampaign(ctx, c.ID)
	require.Error(t, err)
}

func TestIntegration_UpdateSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-600", "dm-1", "Settings Test", nil)
	require.NoError(t, err)

	newSettings := &campaign.Settings{
		TurnTimeoutHours: 72,
		DiagonalRule:     "variant",
		Open5eSources:    []string{"srd"},
	}
	updated, err := svc.UpdateSettings(ctx, c.ID, newSettings)
	require.NoError(t, err)

	var parsed campaign.Settings
	err = json.Unmarshal(updated.Settings.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 72, parsed.TurnTimeoutHours)
}

func TestIntegration_UpdateName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-700", "dm-1", "Old Name", nil)
	require.NoError(t, err)

	updated, err := svc.UpdateName(ctx, c.ID, "New Name")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)

	got, err := svc.GetByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
}

func TestIntegration_ListCampaigns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	_, err := svc.CreateCampaign(ctx, "guild-801", "dm-1", "Campaign A", nil)
	require.NoError(t, err)
	_, err = svc.CreateCampaign(ctx, "guild-802", "dm-2", "Campaign B", nil)
	require.NoError(t, err)

	campaigns, err := svc.ListCampaigns(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(campaigns), 2)
}

func TestIntegration_GuildScoping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c1, err := svc.CreateCampaign(ctx, "guild-901", "dm-1", "Campaign One", nil)
	require.NoError(t, err)
	c2, err := svc.CreateCampaign(ctx, "guild-902", "dm-2", "Campaign Two", nil)
	require.NoError(t, err)

	// GetByGuildID scopes correctly
	got1, err := svc.GetByGuildID(ctx, "guild-901")
	require.NoError(t, err)
	assert.Equal(t, c1.ID, got1.ID)
	assert.Equal(t, "Campaign One", got1.Name)

	got2, err := svc.GetByGuildID(ctx, "guild-902")
	require.NoError(t, err)
	assert.Equal(t, c2.ID, got2.ID)
	assert.Equal(t, "Campaign Two", got2.Name)

	// Non-existent guild returns error
	_, err = svc.GetByGuildID(ctx, "guild-999")
	require.Error(t, err)
}

func TestIntegration_GetByGuildID_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	_, err := svc.GetByGuildID(ctx, "nonexistent-guild")
	require.Error(t, err)
}

func TestIntegration_UpdatedAtChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _ := setupService(t)
	ctx := context.Background()

	c, err := svc.CreateCampaign(ctx, "guild-1000", "dm-1", "Time Test", nil)
	require.NoError(t, err)
	originalUpdatedAt := c.UpdatedAt

	// Update name should change updated_at
	updated, err := svc.UpdateName(ctx, c.ID, "Time Test Updated")
	require.NoError(t, err)
	assert.True(t, !updated.UpdatedAt.Before(originalUpdatedAt))
}
