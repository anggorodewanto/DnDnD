package discord

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// stubUndoCombatantLookup returns a fixed combatant for the discord user.
type stubUndoCombatantLookup struct {
	combatantID uuid.UUID
	displayName string
	encounterID uuid.UUID
	err         error
}

func (s *stubUndoCombatantLookup) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error) {
	if s.err != nil {
		return uuid.Nil, "", s.err
	}
	return s.combatantID, s.displayName, nil
}

// stubUndoEncounterResolver returns the active encounter for the discord user.
type stubUndoEncounterResolver struct {
	encounterID uuid.UUID
	err         error
}

func (s *stubUndoEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	return s.encounterID, s.err
}

// stubUndoActionLog returns a fixed list of action_log rows for the encounter.
type stubUndoActionLog struct {
	rows []refdata.ActionLog
	err  error
}

func (s *stubUndoActionLog) ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
	return s.rows, s.err
}

// stubUndoNotifier captures Post invocations.
type stubUndoNotifier struct {
	posted []dmqueue.Event
	err    error
}

func (s *stubUndoNotifier) Post(ctx context.Context, e dmqueue.Event) (string, error) {
	s.posted = append(s.posted, e)
	return "item-1", s.err
}

func (s *stubUndoNotifier) Cancel(ctx context.Context, itemID, reason string) error {
	return nil
}

func (s *stubUndoNotifier) Resolve(ctx context.Context, itemID, outcome string) error {
	return nil
}

func (s *stubUndoNotifier) ResolveWhisper(ctx context.Context, itemID, replyText string) error {
	return nil
}

func (s *stubUndoNotifier) ResolveSkillCheckNarration(ctx context.Context, itemID, narration string) error {
	return nil
}

func (s *stubUndoNotifier) Get(itemID string) (dmqueue.Item, bool) {
	return dmqueue.Item{}, false
}

func (s *stubUndoNotifier) ListPending() []dmqueue.Item {
	return nil
}

func makeUndoInteraction(guildID, userID, reason string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{}
	if reason != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "reason",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: reason,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "undo",
			Options: opts,
		},
	}
}

func TestUndoHandler_PostsToDMQueue(t *testing.T) {
	sess := &mockInventorySession{}
	encID := uuid.New()
	combatantID := uuid.New()
	campID := uuid.New()

	notifier := &stubUndoNotifier{}
	handler := NewUndoHandler(
		sess,
		&stubUndoEncounterResolver{encounterID: encID},
		&stubUndoCombatantLookup{combatantID: combatantID, displayName: "Aria"},
		&stubUndoActionLog{rows: []refdata.ActionLog{
			{
				ID:          uuid.New(),
				EncounterID: encID,
				ActorID:     combatantID,
				ActionType:  "attack",
				Description: sql.NullString{String: "Aria attacks Goblin", Valid: true},
				CreatedAt:   time.Now().Add(-2 * time.Minute),
			},
			{
				ID:          uuid.New(),
				EncounterID: encID,
				ActorID:     combatantID,
				ActionType:  "move",
				Description: sql.NullString{String: "Aria moves to (3,4)", Valid: true},
				CreatedAt:   time.Now(),
			},
		}},
		notifier,
	)
	// SR-002: wire campaign provider so post carries CampaignID.
	handler.SetCampaignProvider(&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
		return refdata.Campaign{ID: campID}, nil
	}})

	handler.Handle(makeUndoInteraction("guild1", "user1", "misclicked"))

	require.Len(t, notifier.posted, 1, "expected exactly one dm-queue event")
	evt := notifier.posted[0]
	assert.Equal(t, dmqueue.KindUndoRequest, evt.Kind)
	assert.Equal(t, "Aria", evt.PlayerName)
	assert.Equal(t, "guild1", evt.GuildID)
	assert.Contains(t, evt.Summary, "misclicked")
	assert.Contains(t, evt.Summary, "Aria moves to (3,4)", "summary should include the most recent action")
	// SR-002: CampaignID populated so PgStore.Insert succeeds.
	assert.Equal(t, campID.String(), evt.CampaignID,
		"SR-002: /undo dm-queue post must carry CampaignID")
	assert.Contains(t, sess.lastResponse, "sent")
}

func TestUndoHandler_NoEncounter_StillPosts(t *testing.T) {
	sess := &mockInventorySession{}
	notifier := &stubUndoNotifier{}
	handler := NewUndoHandler(
		sess,
		&stubUndoEncounterResolver{err: errors.New("no encounter")},
		&stubUndoCombatantLookup{},
		&stubUndoActionLog{},
		notifier,
	)

	handler.Handle(makeUndoInteraction("guild1", "user1", "test"))

	require.Len(t, notifier.posted, 1)
	assert.Contains(t, notifier.posted[0].Summary, "test")
	assert.Contains(t, notifier.posted[0].Summary, "(no recent action)")
}

func TestUndoHandler_NoNotifier_RespondsAnyway(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewUndoHandler(
		sess,
		&stubUndoEncounterResolver{err: errors.New("none")},
		&stubUndoCombatantLookup{},
		&stubUndoActionLog{},
		nil,
	)
	handler.Handle(makeUndoInteraction("guild1", "user1", "x"))
	// Player still gets ephemeral confirmation even without dm-queue wired.
	assert.Contains(t, sess.lastResponse, "DM")
}

func TestMostRecentActionFor(t *testing.T) {
	actor := uuid.New()
	other := uuid.New()

	t.Run("returns description when latest matching row has one", func(t *testing.T) {
		got := mostRecentActionFor([]refdata.ActionLog{
			{ActorID: other, Description: sql.NullString{String: "ignored", Valid: true}},
			{ActorID: actor, Description: sql.NullString{String: "Aria moves", Valid: true}},
		}, actor)
		assert.Equal(t, "Aria moves", got)
	})

	t.Run("falls back to action_type when description missing", func(t *testing.T) {
		got := mostRecentActionFor([]refdata.ActionLog{
			{ActorID: actor, ActionType: "attack"},
		}, actor)
		assert.Equal(t, "(attack)", got)
	})

	t.Run("returns unknown when description and action_type are empty", func(t *testing.T) {
		got := mostRecentActionFor([]refdata.ActionLog{
			{ActorID: actor},
		}, actor)
		assert.Equal(t, "(unknown action)", got)
	})

	t.Run("returns empty when no rows match the actor", func(t *testing.T) {
		got := mostRecentActionFor([]refdata.ActionLog{
			{ActorID: other, Description: sql.NullString{String: "x", Valid: true}},
		}, actor)
		assert.Equal(t, "", got)
	})
}

func TestUndoHandler_MissingReason_StillPosts(t *testing.T) {
	sess := &mockInventorySession{}
	notifier := &stubUndoNotifier{}
	handler := NewUndoHandler(
		sess,
		&stubUndoEncounterResolver{err: errors.New("none")},
		&stubUndoCombatantLookup{},
		&stubUndoActionLog{},
		notifier,
	)
	handler.Handle(makeUndoInteraction("guild1", "user1", ""))
	require.Len(t, notifier.posted, 1)
	assert.Contains(t, notifier.posted[0].Summary, "(no reason given)")
}
