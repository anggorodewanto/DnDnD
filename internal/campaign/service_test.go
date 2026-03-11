package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	createCampaignFn       func(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
	getCampaignByGuildIDFn func(ctx context.Context, guildID string) (refdata.Campaign, error)
	getCampaignByIDFn      func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	updateCampaignStatusFn func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error)
	updateCampaignSettingsFn func(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
	updateCampaignNameFn   func(ctx context.Context, arg refdata.UpdateCampaignNameParams) (refdata.Campaign, error)
	listCampaignsFn        func(ctx context.Context) ([]refdata.Campaign, error)
}

func (m *mockStore) CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
	return m.createCampaignFn(ctx, arg)
}
func (m *mockStore) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.getCampaignByGuildIDFn(ctx, guildID)
}
func (m *mockStore) GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return m.getCampaignByIDFn(ctx, id)
}
func (m *mockStore) UpdateCampaignStatus(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
	return m.updateCampaignStatusFn(ctx, arg)
}
func (m *mockStore) UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
	return m.updateCampaignSettingsFn(ctx, arg)
}
func (m *mockStore) UpdateCampaignName(ctx context.Context, arg refdata.UpdateCampaignNameParams) (refdata.Campaign, error) {
	return m.updateCampaignNameFn(ctx, arg)
}
func (m *mockStore) ListCampaigns(ctx context.Context) ([]refdata.Campaign, error) {
	return m.listCampaignsFn(ctx)
}

// mockAnnouncer implements Announcer for unit tests.
type mockAnnouncer struct {
	announceFunc func(guildID, message string) error
}

func (m *mockAnnouncer) AnnounceToStory(guildID, message string) error {
	if m.announceFunc != nil {
		return m.announceFunc(guildID, message)
	}
	return nil
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	assert.Equal(t, 24, s.TurnTimeoutHours)
	assert.Equal(t, "standard", s.DiagonalRule)
	assert.Empty(t, s.Open5eSources)
}

func TestSettings_MarshalJSON(t *testing.T) {
	s := Settings{
		TurnTimeoutHours: 48,
		DiagonalRule:     "variant",
		Open5eSources:    []string{"srd", "phb"},
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)

	var parsed Settings
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, s, parsed)
}

func TestCreateCampaign_Success(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		createCampaignFn: func(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
			return refdata.Campaign{
				ID:       campaignID,
				GuildID:  arg.GuildID,
				DmUserID: arg.DmUserID,
				Name:     arg.Name,
				Settings: arg.Settings,
				Status:   StatusActive,
			}, nil
		},
	}

	svc := NewService(store, nil)
	c, err := svc.CreateCampaign(context.Background(), "guild-1", "dm-1", "Test Campaign", nil)
	require.NoError(t, err)
	assert.Equal(t, campaignID, c.ID)
	assert.Equal(t, "guild-1", c.GuildID)
	assert.Equal(t, "dm-1", c.DmUserID)
	assert.Equal(t, "Test Campaign", c.Name)
	assert.Equal(t, StatusActive, c.Status)
}

func TestCreateCampaign_WithSettings(t *testing.T) {
	store := &mockStore{
		createCampaignFn: func(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
			return refdata.Campaign{
				ID:       uuid.New(),
				GuildID:  arg.GuildID,
				DmUserID: arg.DmUserID,
				Name:     arg.Name,
				Settings: arg.Settings,
				Status:   StatusActive,
			}, nil
		},
	}

	settings := &Settings{TurnTimeoutHours: 48, DiagonalRule: "variant"}
	svc := NewService(store, nil)
	c, err := svc.CreateCampaign(context.Background(), "guild-1", "dm-1", "Test", settings)
	require.NoError(t, err)

	var parsed Settings
	err = json.Unmarshal(c.Settings.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 48, parsed.TurnTimeoutHours)
	assert.Equal(t, "variant", parsed.DiagonalRule)
}

func TestCreateCampaign_EmptyGuildID(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.CreateCampaign(context.Background(), "", "dm-1", "Test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guild_id")
}

func TestCreateCampaign_EmptyName(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.CreateCampaign(context.Background(), "guild-1", "dm-1", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCreateCampaign_EmptyDMUserID(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.CreateCampaign(context.Background(), "guild-1", "", "Test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dm_user_id")
}

func TestCreateCampaign_StoreError(t *testing.T) {
	store := &mockStore{
		createCampaignFn: func(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("db error")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.CreateCampaign(context.Background(), "guild-1", "dm-1", "Test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestGetByGuildID_Success(t *testing.T) {
	expected := refdata.Campaign{
		ID:      uuid.New(),
		GuildID: "guild-1",
		Name:    "My Campaign",
		Status:  "active",
	}
	store := &mockStore{
		getCampaignByGuildIDFn: func(ctx context.Context, guildID string) (refdata.Campaign, error) {
			assert.Equal(t, "guild-1", guildID)
			return expected, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.GetByGuildID(context.Background(), "guild-1")
	require.NoError(t, err)
	assert.Equal(t, expected.ID, c.ID)
}

func TestGetByGuildID_EmptyGuildID(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.GetByGuildID(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guild_id")
}

func TestGetByID_Success(t *testing.T) {
	id := uuid.New()
	expected := refdata.Campaign{ID: id, GuildID: "guild-1", Name: "Test"}
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			assert.Equal(t, id, cid)
			return expected, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, c.ID)
}

func TestPauseCampaign_Success(t *testing.T) {
	id := uuid.New()
	var announced bool
	announcer := &mockAnnouncer{
		announceFunc: func(guildID, message string) error {
			assert.Equal(t, "guild-1", guildID)
			assert.Contains(t, message, "paused")
			announced = true
			return nil
		},
	}
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			assert.Equal(t, StatusPaused, arg.Status)
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
	}
	svc := NewService(store, announcer)
	c, err := svc.PauseCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusPaused, c.Status)
	assert.True(t, announced)
}

func TestPauseCampaign_AlreadyPaused(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.PauseCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already paused")
}

func TestPauseCampaign_Archived(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusArchived}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.PauseCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition")
}

func TestResumeCampaign_Success(t *testing.T) {
	id := uuid.New()
	var announced bool
	announcer := &mockAnnouncer{
		announceFunc: func(guildID, message string) error {
			assert.Equal(t, "guild-1", guildID)
			assert.Contains(t, message, "resumed")
			announced = true
			return nil
		},
	}
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			assert.Equal(t, StatusActive, arg.Status)
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
	}
	svc := NewService(store, announcer)
	c, err := svc.ResumeCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Status)
	assert.True(t, announced)
}

func TestResumeCampaign_AlreadyActive(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ResumeCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")
}

func TestResumeCampaign_Archived(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusArchived}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ResumeCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition")
}

func TestArchiveCampaign_Success(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			assert.Equal(t, StatusArchived, arg.Status)
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusArchived}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.ArchiveCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusArchived, c.Status)
}

func TestArchiveCampaign_AlreadyArchived(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusArchived}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ArchiveCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already archived")
}

func TestUpdateSettings_Success(t *testing.T) {
	id := uuid.New()
	settings := &Settings{TurnTimeoutHours: 48, DiagonalRule: "variant"}
	store := &mockStore{
		updateCampaignSettingsFn: func(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
			assert.Equal(t, id, arg.ID)
			return refdata.Campaign{ID: id, Settings: arg.Settings}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.UpdateSettings(context.Background(), id, settings)
	require.NoError(t, err)
	assert.Equal(t, id, c.ID)
}

func TestUpdateSettings_NilSettings(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.UpdateSettings(context.Background(), uuid.New(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings")
}

func TestUpdateName_Success(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		updateCampaignNameFn: func(ctx context.Context, arg refdata.UpdateCampaignNameParams) (refdata.Campaign, error) {
			assert.Equal(t, id, arg.ID)
			assert.Equal(t, "New Name", arg.Name)
			return refdata.Campaign{ID: id, Name: "New Name"}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.UpdateName(context.Background(), id, "New Name")
	require.NoError(t, err)
	assert.Equal(t, "New Name", c.Name)
}

func TestUpdateName_Empty(t *testing.T) {
	svc := NewService(&mockStore{}, nil)
	_, err := svc.UpdateName(context.Background(), uuid.New(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestListCampaigns_Success(t *testing.T) {
	expected := []refdata.Campaign{
		{ID: uuid.New(), GuildID: "guild-1", Name: "Campaign 1"},
		{ID: uuid.New(), GuildID: "guild-2", Name: "Campaign 2"},
	}
	store := &mockStore{
		listCampaignsFn: func(ctx context.Context) ([]refdata.Campaign, error) {
			return expected, nil
		},
	}
	svc := NewService(store, nil)
	campaigns, err := svc.ListCampaigns(context.Background())
	require.NoError(t, err)
	assert.Len(t, campaigns, 2)
}

func TestPauseCampaign_AnnounceError_StillPauses(t *testing.T) {
	id := uuid.New()
	announcer := &mockAnnouncer{
		announceFunc: func(guildID, message string) error {
			return errors.New("discord error")
		},
	}
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
	}
	svc := NewService(store, announcer)
	c, err := svc.PauseCampaign(context.Background(), id)
	// Should still succeed; announcement failure is non-fatal
	require.NoError(t, err)
	assert.Equal(t, StatusPaused, c.Status)
}

func TestResumeCampaign_AnnounceError_StillResumes(t *testing.T) {
	id := uuid.New()
	announcer := &mockAnnouncer{
		announceFunc: func(guildID, message string) error {
			return errors.New("discord error")
		},
	}
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
	}
	svc := NewService(store, announcer)
	c, err := svc.ResumeCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Status)
}

func TestPauseCampaign_NilAnnouncer(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.PauseCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusPaused, c.Status)
}

func TestCreateCampaign_DefaultSettings(t *testing.T) {
	store := &mockStore{
		createCampaignFn: func(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
			// When nil settings passed, default settings should be used
			assert.True(t, arg.Settings.Valid)
			var s Settings
			err := json.Unmarshal(arg.Settings.RawMessage, &s)
			require.NoError(t, err)
			assert.Equal(t, 24, s.TurnTimeoutHours)
			assert.Equal(t, "standard", s.DiagonalRule)
			return refdata.Campaign{
				ID:       uuid.New(),
				GuildID:  arg.GuildID,
				Settings: arg.Settings,
				Status:   StatusActive,
			}, nil
		},
	}
	svc := NewService(store, nil)
	_, err := svc.CreateCampaign(context.Background(), "guild-1", "dm-1", "Test", nil)
	require.NoError(t, err)
}

func TestPauseCampaign_GetByIDError(t *testing.T) {
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.PauseCampaign(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching campaign")
}

func TestPauseCampaign_UpdateStatusError(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("db error")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.PauseCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating status")
}

func TestResumeCampaign_GetByIDError(t *testing.T) {
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ResumeCampaign(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching campaign")
}

func TestResumeCampaign_UpdateStatusError(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("db error")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ResumeCampaign(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating status")
}

func TestArchiveCampaign_GetByIDError(t *testing.T) {
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.ArchiveCampaign(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching campaign")
}

func TestArchiveCampaign_FromPaused(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			assert.Equal(t, StatusArchived, arg.Status)
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusArchived}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.ArchiveCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusArchived, c.Status)
}

func TestUpdateSettings_StoreError(t *testing.T) {
	store := &mockStore{
		updateCampaignSettingsFn: func(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("db error")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.UpdateSettings(context.Background(), uuid.New(), &Settings{TurnTimeoutHours: 24})
	require.Error(t, err)
}

func TestResumeCampaign_NilAnnouncer(t *testing.T) {
	id := uuid.New()
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, cid uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusPaused}, nil
		},
		updateCampaignStatusFn: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: id, GuildID: "guild-1", Status: StatusActive}, nil
		},
	}
	svc := NewService(store, nil)
	c, err := svc.ResumeCampaign(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Status)
}

func TestGetByID_StoreError(t *testing.T) {
	store := &mockStore{
		getCampaignByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestGetByGuildID_StoreError(t *testing.T) {
	store := &mockStore{
		getCampaignByGuildIDFn: func(ctx context.Context, guildID string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}
	svc := NewService(store, nil)
	_, err := svc.GetByGuildID(context.Background(), "guild-1")
	require.Error(t, err)
}

func TestSettingsToNullRawMessage(t *testing.T) {
	s := &Settings{TurnTimeoutHours: 12, DiagonalRule: "variant", Open5eSources: []string{"srd"}}
	msg, err := settingsToNullRawMessage(s)
	require.NoError(t, err)
	assert.True(t, msg.Valid)

	var parsed Settings
	err = json.Unmarshal(msg.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 12, parsed.TurnTimeoutHours)
	assert.Equal(t, "variant", parsed.DiagonalRule)
	assert.Equal(t, []string{"srd"}, parsed.Open5eSources)
}

func TestSettings_ChannelIDs_RoundTrip(t *testing.T) {
	s := &Settings{
		TurnTimeoutHours: 24,
		DiagonalRule:     "standard",
		ChannelIDs: map[string]string{
			"the-story":   "chan-1",
			"combat-map":  "chan-2",
			"in-character": "chan-3",
		},
	}
	msg, err := settingsToNullRawMessage(s)
	require.NoError(t, err)

	var parsed Settings
	err = json.Unmarshal(msg.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"the-story":   "chan-1",
		"combat-map":  "chan-2",
		"in-character": "chan-3",
	}, parsed.ChannelIDs)
}
