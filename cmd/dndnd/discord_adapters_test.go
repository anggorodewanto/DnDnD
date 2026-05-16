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
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeResolverQueries mocks the subset of refdata.Queries methods used by the
// discordUserEncounterResolver so we can unit test the guild->character->
// encounter chain without spinning up Postgres.
type fakeResolverQueries struct {
	campaignsByGuild    map[string]refdata.Campaign
	campaignErr         error
	playerByCampUser    map[string]refdata.PlayerCharacter
	playerErr           error
	activeEncByChar     map[uuid.UUID]uuid.UUID
	activeEncErr        error
	activeEncNotFound   bool
	activeEncCountByChar map[uuid.UUID]int64
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

func (f *fakeResolverQueries) CountActiveEncountersByCharacterID(ctx context.Context, characterID uuid.NullUUID) (int64, error) {
	if f.activeEncCountByChar == nil {
		return 1, nil
	}
	count, ok := f.activeEncCountByChar[characterID.UUID]
	if !ok {
		return 0, nil
	}
	return count, nil
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

func TestDiscordUserEncounterResolver_AmbiguousEncounter(t *testing.T) {
	campaignID := uuid.New()
	characterID := uuid.New()
	encounterID := uuid.New()

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
		activeEncByChar: map[uuid.UUID]uuid.UUID{
			characterID: encounterID,
		},
		activeEncCountByChar: map[uuid.UUID]int64{
			characterID: 2, // two active encounters — corrupt state
		},
	}

	r := newDiscordUserEncounterResolver(q)
	_, err := r.ActiveEncounterForUser(context.Background(), "guild-42", "user-7")
	require.ErrorIs(t, err, ErrAmbiguousEncounter)
}

// fakeSetupQueries mocks the subset of refdata.Queries used by
// setupCampaignLookup so the adapter can be unit tested without a live
// Postgres instance.
type fakeSetupQueries struct {
	campaignsByGuild map[string]refdata.Campaign
	campaignErr      error
	updateErr        error
	updateCalls      []refdata.UpdateCampaignSettingsParams
	createErr        error
	createCalls      []refdata.CreateCampaignParams
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

// CreateCampaign records the params and (when the call is meant to succeed)
// inserts the row into campaignsByGuild so subsequent GetCampaignByGuildID
// lookups in the same test see it.
func (f *fakeSetupQueries) CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
	f.createCalls = append(f.createCalls, arg)
	if f.createErr != nil {
		return refdata.Campaign{}, f.createErr
	}
	c := refdata.Campaign{
		ID:       uuid.New(),
		GuildID:  arg.GuildID,
		DmUserID: arg.DmUserID,
		Name:     arg.Name,
		Settings: arg.Settings,
	}
	if f.campaignsByGuild == nil {
		f.campaignsByGuild = map[string]refdata.Campaign{}
	}
	f.campaignsByGuild[arg.GuildID] = c
	return c, nil
}

func TestSetupCampaignLookup_GetCampaignForSetup_ReturnsDMUserID(t *testing.T) {
	q := &fakeSetupQueries{
		campaignsByGuild: map[string]refdata.Campaign{
			"guild-1": {ID: uuid.New(), GuildID: "guild-1", DmUserID: "dm-user-9"},
		},
	}
	lookup := newSetupCampaignLookup(q)

	info, err := lookup.GetCampaignForSetup("guild-1", "invoker-ignored")
	require.NoError(t, err)
	assert.Equal(t, "dm-user-9", info.DMUserID)
	assert.False(t, info.AutoCreated, "existing campaign should not be flagged auto-created")
	assert.Empty(t, q.createCalls, "existing campaign should not trigger a create")
}

func TestSetupCampaignLookup_GetCampaignForSetup_PropagatesError(t *testing.T) {
	boom := errors.New("db down")
	q := &fakeSetupQueries{campaignErr: boom}
	lookup := newSetupCampaignLookup(q)

	_, err := lookup.GetCampaignForSetup("guild-1", "invoker-1")
	require.ErrorIs(t, err, boom)
}

// med-41: when the guild has no campaign row yet, /setup auto-creates one
// with the invoker as DM and default settings (closes the Phase 11 dead end
// that used to error out with "no campaign found for this server").
func TestSetupCampaignLookup_GetCampaignForSetup_AutoCreatesOnNoRows(t *testing.T) {
	q := &fakeSetupQueries{}
	lookup := newSetupCampaignLookup(q)

	info, err := lookup.GetCampaignForSetup("guild-new", "dm-77")
	require.NoError(t, err)
	assert.Equal(t, "dm-77", info.DMUserID)
	assert.True(t, info.AutoCreated)

	require.Len(t, q.createCalls, 1)
	created := q.createCalls[0]
	assert.Equal(t, "guild-new", created.GuildID)
	assert.Equal(t, "dm-77", created.DmUserID)
	assert.NotEmpty(t, created.Name)
	require.True(t, created.Settings.Valid)

	var saved campaign.Settings
	require.NoError(t, json.Unmarshal(created.Settings.RawMessage, &saved))
	assert.Equal(t, 24, saved.TurnTimeoutHours)
	assert.Equal(t, "standard", saved.DiagonalRule)
}

func TestSetupCampaignLookup_GetCampaignForSetup_AutoCreate_RequiresInvoker(t *testing.T) {
	q := &fakeSetupQueries{}
	lookup := newSetupCampaignLookup(q)

	_, err := lookup.GetCampaignForSetup("guild-new", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoker")
	assert.Empty(t, q.createCalls, "must not create campaign with empty DM user id")
}

func TestSetupCampaignLookup_GetCampaignForSetup_AutoCreate_PropagatesCreateError(t *testing.T) {
	boom := errors.New("create failed")
	q := &fakeSetupQueries{createErr: boom}
	lookup := newSetupCampaignLookup(q)

	_, err := lookup.GetCampaignForSetup("guild-new", "dm-1")
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

// fakePlayerDM mocks playerDirectMessenger so playerNotifierAdapter can be
// unit tested without a live Discord session. Records every send so the
// tests can assert on the body verbatim.
type fakePlayerDM struct {
	calls []fakePlayerDMCall
	err   error
}

type fakePlayerDMCall struct {
	UserID string
	Body   string
}

func (f *fakePlayerDM) SendDirectMessage(discordUserID, body string) ([]string, error) {
	f.calls = append(f.calls, fakePlayerDMCall{UserID: discordUserID, Body: body})
	if f.err != nil {
		return nil, f.err
	}
	return []string{"msg-1"}, nil
}

func TestPlayerNotifierAdapter_NotifyApproval_SendsDMWithCharacterName(t *testing.T) {
	dm := &fakePlayerDM{}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyApproval(context.Background(), "user-7", "Tordek")
	require.NoError(t, err)
	require.Len(t, dm.calls, 1)
	assert.Equal(t, "user-7", dm.calls[0].UserID)
	assert.Contains(t, dm.calls[0].Body, "Tordek")
	assert.Contains(t, dm.calls[0].Body, "approved")
}

func TestPlayerNotifierAdapter_NotifyChangesRequested_IncludesFeedback(t *testing.T) {
	dm := &fakePlayerDM{}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyChangesRequested(context.Background(), "user-7", "Tordek", "please pick a subclass")
	require.NoError(t, err)
	require.Len(t, dm.calls, 1)
	assert.Equal(t, "user-7", dm.calls[0].UserID)
	assert.Contains(t, dm.calls[0].Body, "Tordek")
	assert.Contains(t, dm.calls[0].Body, "please pick a subclass")
}

func TestPlayerNotifierAdapter_NotifyRejection_IncludesFeedback(t *testing.T) {
	dm := &fakePlayerDM{}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyRejection(context.Background(), "user-7", "Tordek", "no homebrew classes")
	require.NoError(t, err)
	require.Len(t, dm.calls, 1)
	assert.Equal(t, "user-7", dm.calls[0].UserID)
	assert.Contains(t, dm.calls[0].Body, "Tordek")
	assert.Contains(t, dm.calls[0].Body, "no homebrew classes")
}

func TestPlayerNotifierAdapter_NotifyApproval_PropagatesDMError(t *testing.T) {
	boom := errors.New("dm channel closed")
	dm := &fakePlayerDM{err: boom}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyApproval(context.Background(), "user-7", "Tordek")
	require.ErrorIs(t, err, boom)
}

func TestPlayerNotifierAdapter_NotifyChangesRequested_PropagatesDMError(t *testing.T) {
	boom := errors.New("dm channel closed")
	dm := &fakePlayerDM{err: boom}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyChangesRequested(context.Background(), "user-7", "Tordek", "feedback")
	require.ErrorIs(t, err, boom)
}

func TestPlayerNotifierAdapter_NotifyRejection_PropagatesDMError(t *testing.T) {
	boom := errors.New("dm channel closed")
	dm := &fakePlayerDM{err: boom}
	a := newPlayerNotifierAdapter(dm)

	err := a.NotifyRejection(context.Background(), "user-7", "Tordek", "feedback")
	require.ErrorIs(t, err, boom)
}

// TestPlayerNotifierAdapter_SatisfiesPlayerNotifier locks in the contract that
// playerNotifierAdapter implements dashboard.PlayerNotifier so a future
// signature change to the interface fails this test before reaching main.go.
func TestPlayerNotifierAdapter_SatisfiesPlayerNotifier(t *testing.T) {
	var _ dashboard.PlayerNotifier = (*playerNotifierAdapter)(nil)
}
