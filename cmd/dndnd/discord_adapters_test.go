package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
