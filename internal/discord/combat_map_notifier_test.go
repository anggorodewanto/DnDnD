package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

// readFile drains a MessageSend's first file Reader into a string.
func readFirstFile(data *discordgo.MessageSend) []byte {
	if len(data.Files) == 0 {
		return nil
	}
	buf := make([]byte, 1024)
	n, _ := data.Files[0].Reader.Read(buf)
	return buf[:n]
}

// TestDiscordCombatMapNotifier_PostsMap asserts the notifier renders the PNG
// and posts it to #combat-map with the "⚔️ <name> — Round N" label.
func TestDiscordCombatMapNotifier_PostsMap(t *testing.T) {
	encounterID := uuid.New()
	pngData := []byte("fake-png-bytes")

	var sentChannel, sentContent string
	var sentFile []byte
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = data.Content
			sentFile = readFirstFile(data)
			return &discordgo.Message{}, nil
		},
	}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-map": "ch-combat-map"}, nil
		},
	}
	mr := &mockMapRegenerator{
		regenerateMap: func(_ context.Context, _ uuid.UUID) ([]byte, error) { return pngData, nil },
	}
	lookup := &mockEnemyTurnEncounterLookup{
		getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: id, Name: "Rooftop Ambush", RoundNumber: 3}, nil
		},
	}

	n := NewDiscordCombatMapNotifier(session, csp, mr, lookup)
	n.PostCombatMap(context.Background(), encounterID)

	assert.Equal(t, "ch-combat-map", sentChannel)
	assert.Equal(t, pngData, sentFile)
	assert.Contains(t, sentContent, "Rooftop Ambush")
	assert.Contains(t, sentContent, "Round 3")
}

// TestDiscordCombatMapNotifier_NilLookup_EmptyLabel posts with an empty label
// (no encounter lookup wired) but still uploads the PNG.
func TestDiscordCombatMapNotifier_NilLookup_EmptyLabel(t *testing.T) {
	var sentChannel, sentContent string
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = data.Content
			return &discordgo.Message{}, nil
		},
	}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-map": "ch-combat-map"}, nil
		},
	}
	mr := &mockMapRegenerator{
		regenerateMap: func(_ context.Context, _ uuid.UUID) ([]byte, error) { return []byte("png"), nil },
	}

	n := NewDiscordCombatMapNotifier(session, csp, mr, nil)
	n.PostCombatMap(context.Background(), uuid.New())

	assert.Equal(t, "ch-combat-map", sentChannel)
	assert.Equal(t, "", sentContent)
}

func TestDiscordCombatMapNotifier_NilSettingsProvider_NoOp(t *testing.T) {
	posted := false
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(string, *discordgo.MessageSend) (*discordgo.Message, error) {
			posted = true
			return &discordgo.Message{}, nil
		},
	}
	n := NewDiscordCombatMapNotifier(session, nil, nil, nil)
	n.PostCombatMap(context.Background(), uuid.New())
	assert.False(t, posted, "no settings provider must be a silent no-op")
}

func TestDiscordCombatMapNotifier_ChannelLookupError_NoOp(t *testing.T) {
	posted := false
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(string, *discordgo.MessageSend) (*discordgo.Message, error) {
			posted = true
			return &discordgo.Message{}, nil
		},
	}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return nil, errors.New("db down")
		},
	}
	n := NewDiscordCombatMapNotifier(session, csp, nil, nil)
	n.PostCombatMap(context.Background(), uuid.New())
	assert.False(t, posted, "a channel-id lookup error must be a silent no-op")
}

func TestDiscordCombatMapNotifier_NoCombatMapChannel_NoOp(t *testing.T) {
	posted := false
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(string, *discordgo.MessageSend) (*discordgo.Message, error) {
			posted = true
			return &discordgo.Message{}, nil
		},
	}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-log"}, nil // no combat-map key
		},
	}
	mr := &mockMapRegenerator{
		regenerateMap: func(_ context.Context, _ uuid.UUID) ([]byte, error) { return []byte("png"), nil },
	}
	n := NewDiscordCombatMapNotifier(session, csp, mr, nil)
	n.PostCombatMap(context.Background(), uuid.New())
	assert.False(t, posted, "a missing #combat-map channel must be a silent no-op")
}

// TestDiscordCombatMapNotifier_RenderFailure_NotifiesDM asserts a render error
// surfaces to the wired CombatMapRenderFailureNotifier and posts nothing.
func TestDiscordCombatMapNotifier_RenderFailure_NotifiesDM(t *testing.T) {
	encounterID := uuid.New()
	posted := false
	session := &MockSession{
		ChannelMessageSendComplexFunc: func(string, *discordgo.MessageSend) (*discordgo.Message, error) {
			posted = true
			return &discordgo.Message{}, nil
		},
	}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-map": "ch-combat-map"}, nil
		},
	}
	mr := &mockMapRegenerator{
		regenerateMap: func(_ context.Context, _ uuid.UUID) ([]byte, error) {
			return nil, errors.New("render boom")
		},
	}
	fail := &recordingMapRenderFailNotifier{}

	n := NewDiscordCombatMapNotifier(session, csp, mr, nil)
	n.SetMapRenderFailureNotifier(fail)
	n.PostCombatMap(context.Background(), encounterID)

	assert.False(t, posted, "a render failure must not post a (stale/empty) map")
	assert.Equal(t, []uuid.UUID{encounterID}, fail.encounterIDs, "render failure must surface to the DM")
}
