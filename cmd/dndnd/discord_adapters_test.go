package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeResolverQueries mocks the subset of refdata.Queries methods used by the
// discordUserEncounterResolver so we can unit test the guild->character->
// encounter chain without spinning up Postgres.
type fakeResolverQueries struct {
	campaignsByGuild  map[string]refdata.Campaign
	campaignErr       error
	playerByCampUser  map[string]refdata.PlayerCharacter
	playerErr         error
	activeEncByChar   map[uuid.UUID]uuid.UUID
	activeEncErr      error
	activeEncNotFound bool
}

func (f *fakeResolverQueries) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	if f.campaignErr != nil {
		return refdata.Campaign{}, f.campaignErr
	}
	c, ok := f.campaignsByGuild[guildID]
	if !ok {
		return refdata.Campaign{}, sql.ErrNoRows
	}
	return c, nil
}

func (f *fakeResolverQueries) GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error) {
	if f.playerErr != nil {
		return refdata.PlayerCharacter{}, f.playerErr
	}
	key := arg.CampaignID.String() + "|" + arg.DiscordUserID
	pc, ok := f.playerByCampUser[key]
	if !ok {
		return refdata.PlayerCharacter{}, sql.ErrNoRows
	}
	return pc, nil
}

func (f *fakeResolverQueries) GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error) {
	if f.activeEncErr != nil {
		return uuid.Nil, f.activeEncErr
	}
	if f.activeEncNotFound {
		return uuid.Nil, sql.ErrNoRows
	}
	encID, ok := f.activeEncByChar[characterID.UUID]
	if !ok {
		return uuid.Nil, sql.ErrNoRows
	}
	return encID, nil
}

func TestDiscordUserEncounterResolver_HappyPath(t *testing.T) {
	campaignID := uuid.New()
	characterID := uuid.New()
	encounterID := uuid.New()
	pcID := uuid.New()

	q := &fakeResolverQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-42": {ID: campaignID, GuildID: "guild-42"},
		},
		playerByCampUser: map[string]refdata.PlayerCharacter{
			campaignID.String() + "|user-7": {
				ID:            pcID,
				CampaignID:    campaignID,
				CharacterID:   characterID,
				DiscordUserID: "user-7",
			},
		},
		activeEncByChar: map[uuid.UUID]uuid.UUID{
			characterID: encounterID,
		},
	}

	r := newDiscordUserEncounterResolver(q)
	got, err := r.ActiveEncounterForUser(context.Background(), "guild-42", "user-7")
	require.NoError(t, err)
	assert.Equal(t, encounterID, got)
}

func TestDiscordUserEncounterResolver_MissingCampaign(t *testing.T) {
	q := &fakeResolverQueries{
		campaignsByGuild: map[string]refdata.Campaign{},
	}
	r := newDiscordUserEncounterResolver(q)
	_, err := r.ActiveEncounterForUser(context.Background(), "no-such-guild", "user-7")
	require.Error(t, err)
}

func TestDiscordUserEncounterResolver_MissingPlayerCharacter(t *testing.T) {
	campaignID := uuid.New()
	q := &fakeResolverQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-42": {ID: campaignID, GuildID: "guild-42"},
		},
		playerByCampUser: map[string]refdata.PlayerCharacter{},
	}
	r := newDiscordUserEncounterResolver(q)
	_, err := r.ActiveEncounterForUser(context.Background(), "guild-42", "unregistered")
	require.Error(t, err)
}

func TestDiscordUserEncounterResolver_CharacterNotInCombat(t *testing.T) {
	campaignID := uuid.New()
	characterID := uuid.New()
	q := &fakeResolverQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-42": {ID: campaignID, GuildID: "guild-42"},
		},
		playerByCampUser: map[string]refdata.PlayerCharacter{
			campaignID.String() + "|user-7": {
				ID:            uuid.New(),
				CampaignID:    campaignID,
				CharacterID:   characterID,
				DiscordUserID: "user-7",
			},
		},
		activeEncNotFound: true,
	}
	r := newDiscordUserEncounterResolver(q)
	_, err := r.ActiveEncounterForUser(context.Background(), "guild-42", "user-7")
	require.Error(t, err)
}

func TestDiscordUserEncounterResolver_PropagatesUnexpectedError(t *testing.T) {
	boom := errors.New("db exploded")
	q := &fakeResolverQueries{
		campaignErr: boom,
	}
	r := newDiscordUserEncounterResolver(q)
	_, err := r.ActiveEncounterForUser(context.Background(), "guild-42", "user-7")
	require.ErrorIs(t, err, boom)
}

// fakeSetupQueries mocks the subset of refdata.Queries used by
// setupCampaignLookup so the adapter can be unit tested without a live
// Postgres instance.
type fakeSetupQueries struct {
	campaignsByGuild map[string]refdata.Campaign
	campaignErr      error
	updateErr        error
	updateCalls      []refdata.UpdateCampaignSettingsParams
}

func (f *fakeSetupQueries) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	if f.campaignErr != nil {
		return refdata.Campaign{}, f.campaignErr
	}
	c, ok := f.campaignsByGuild[guildID]
	if !ok {
		return refdata.Campaign{}, sql.ErrNoRows
	}
	return c, nil
}

func (f *fakeSetupQueries) UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
	f.updateCalls = append(f.updateCalls, arg)
	if f.updateErr != nil {
		return refdata.Campaign{}, f.updateErr
	}
	c := f.campaignsByGuild[""]
	return c, nil
}

func TestSetupCampaignLookup_GetCampaignForSetup_ReturnsDMUserID(t *testing.T) {
	q := &fakeSetupQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-1": {ID: uuid.New(), GuildID: "guild-1", DmUserID: "dm-user-9"},
		},
	}
	lookup := newSetupCampaignLookup(q)

	info, err := lookup.GetCampaignForSetup("guild-1")
	require.NoError(t, err)
	assert.Equal(t, "dm-user-9", info.DMUserID)
}

func TestSetupCampaignLookup_GetCampaignForSetup_PropagatesError(t *testing.T) {
	boom := errors.New("db down")
	q := &fakeSetupQueries{campaignErr: boom}
	lookup := newSetupCampaignLookup(q)

	_, err := lookup.GetCampaignForSetup("guild-1")
	require.ErrorIs(t, err, boom)
}

func TestSetupCampaignLookup_SaveChannelIDs_MergesIntoSettings(t *testing.T) {
	campaignID := uuid.New()
	existing := campaign.Settings{
		TurnTimeoutHours: 24,
		DiagonalRule:     "standard",
		Open5eSources:    []string{"srd"},
	}
	rawExisting, err := json.Marshal(existing)
	require.NoError(t, err)

	q := &fakeSetupQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-1": {
				ID:       campaignID,
				GuildID:  "guild-1",
				DmUserID: "dm-user-9",
				Settings: pqtype.NullRawMessage{RawMessage: rawExisting, Valid: true},
			},
		},
	}
	lookup := newSetupCampaignLookup(q)

	channelIDs := map[string]string{
		"the-story":   "chan-1",
		"combat-map":  "chan-2",
		"combat-log":  "chan-3",
		"roll-history": "chan-4",
	}
	require.NoError(t, lookup.SaveChannelIDs("guild-1", channelIDs))

	require.Len(t, q.updateCalls, 1)
	assert.Equal(t, campaignID, q.updateCalls[0].ID)
	require.True(t, q.updateCalls[0].Settings.Valid)

	var saved campaign.Settings
	require.NoError(t, json.Unmarshal(q.updateCalls[0].Settings.RawMessage, &saved))
	assert.Equal(t, 24, saved.TurnTimeoutHours)
	assert.Equal(t, "standard", saved.DiagonalRule)
	assert.Equal(t, []string{"srd"}, saved.Open5eSources)
	assert.Equal(t, channelIDs, saved.ChannelIDs)
}

func TestSetupCampaignLookup_SaveChannelIDs_DefaultSettingsWhenNullSettings(t *testing.T) {
	campaignID := uuid.New()
	q := &fakeSetupQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-1": {
				ID:       campaignID,
				GuildID:  "guild-1",
				DmUserID: "dm-user-9",
				// Settings deliberately null.
			},
		},
	}
	lookup := newSetupCampaignLookup(q)

	channelIDs := map[string]string{"the-story": "chan-1"}
	require.NoError(t, lookup.SaveChannelIDs("guild-1", channelIDs))

	require.Len(t, q.updateCalls, 1)
	var saved campaign.Settings
	require.NoError(t, json.Unmarshal(q.updateCalls[0].Settings.RawMessage, &saved))
	// Defaults applied for the JSONB blob even though the row had NULL.
	assert.Equal(t, 24, saved.TurnTimeoutHours)
	assert.Equal(t, "standard", saved.DiagonalRule)
	assert.Equal(t, channelIDs, saved.ChannelIDs)
}

func TestSetupCampaignLookup_SaveChannelIDs_PropagatesGetError(t *testing.T) {
	boom := errors.New("get failed")
	q := &fakeSetupQueries{campaignErr: boom}
	lookup := newSetupCampaignLookup(q)

	err := lookup.SaveChannelIDs("guild-1", map[string]string{"foo": "bar"})
	require.ErrorIs(t, err, boom)
	assert.Len(t, q.updateCalls, 0)
}

func TestSetupCampaignLookup_SaveChannelIDs_PropagatesUpdateError(t *testing.T) {
	boom := errors.New("update failed")
	q := &fakeSetupQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-1": {ID: uuid.New(), GuildID: "guild-1", DmUserID: "dm-user-9"},
		},
		updateErr: boom,
	}
	lookup := newSetupCampaignLookup(q)

	err := lookup.SaveChannelIDs("guild-1", map[string]string{"foo": "bar"})
	require.ErrorIs(t, err, boom)
}
