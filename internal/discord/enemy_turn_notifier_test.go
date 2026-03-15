package discord

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- TDD Cycle 1: Posts combat log to #combat-log ---

func TestDiscordEnemyTurnNotifier_PostsCombatLog(t *testing.T) {
	encounterID := uuid.New()
	combatLog := "**Goblin's Turn**\n⚔️ Scimitar vs Aragorn: 19 to hit — **Hit!** 7 slashing damage"

	var sentChannel, sentContent string
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}

	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{
				"combat-log": "ch-combat-log",
				"combat-map": "ch-combat-map",
			}, nil
		},
	}

	notifier := NewDiscordEnemyTurnNotifier(session, csp, nil)
	notifier.NotifyEnemyTurnExecuted(context.Background(), encounterID, combatLog)

	assert.Equal(t, "ch-combat-log", sentChannel)
	assert.Equal(t, combatLog, sentContent)
}

// --- TDD Cycle 2: Regenerates and posts map to #combat-map ---

func TestDiscordEnemyTurnNotifier_PostsMap(t *testing.T) {
	encounterID := uuid.New()
	pngData := []byte("fake-png-data")

	var mapChannelID string
	var mapFileData []byte
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return &discordgo.Message{}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			mapChannelID = channelID
			if len(data.Files) > 0 {
				buf := make([]byte, 1024)
				n, _ := data.Files[0].Reader.Read(buf)
				mapFileData = buf[:n]
			}
			return &discordgo.Message{}, nil
		},
	}

	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{
				"combat-log": "ch-combat-log",
				"combat-map": "ch-combat-map",
			}, nil
		},
	}

	mr := &mockMapRegenerator{
		regenerateMap: func(ctx context.Context, eid uuid.UUID) ([]byte, error) {
			return pngData, nil
		},
	}

	notifier := NewDiscordEnemyTurnNotifier(session, csp, mr)
	notifier.NotifyEnemyTurnExecuted(context.Background(), encounterID, "combat log")

	assert.Equal(t, "ch-combat-map", mapChannelID)
	assert.Equal(t, pngData, mapFileData)
}

// --- TDD Cycle 3: Graceful when no settings provider ---

func TestDiscordEnemyTurnNotifier_NilProviderNoPanic(t *testing.T) {
	session := &MockSession{}
	notifier := NewDiscordEnemyTurnNotifier(session, nil, nil)

	// Should not panic
	notifier.NotifyEnemyTurnExecuted(context.Background(), uuid.New(), "log")
}

// --- TDD Cycle 4: Graceful when no map regenerator ---

func TestDiscordEnemyTurnNotifier_NilMapRegeneratorSkipsMap(t *testing.T) {
	var sentContent string
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}

	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{
				"combat-log": "ch-combat-log",
				"combat-map": "ch-combat-map",
			}, nil
		},
	}

	notifier := NewDiscordEnemyTurnNotifier(session, csp, nil)
	notifier.NotifyEnemyTurnExecuted(context.Background(), uuid.New(), "log msg")

	assert.Equal(t, "log msg", sentContent)
}
