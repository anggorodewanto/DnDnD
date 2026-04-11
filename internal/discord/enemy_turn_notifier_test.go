package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

// mockEnemyTurnEncounterLookup is a minimal encounter lookup used by the
// DiscordEnemyTurnNotifier to resolve the encounter label (display name and
// round number) for Phase 105 simultaneous-encounter labeling.
type mockEnemyTurnEncounterLookup struct {
	getEncounter func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
}

func (m *mockEnemyTurnEncounterLookup) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounter(ctx, id)
}

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

// --- Phase 105 iter 2: Posts include encounter label prefix ---

func TestDiscordEnemyTurnNotifier_CombatLogIncludesEncounterLabel(t *testing.T) {
	encounterID := uuid.New()
	combatLog := "**Goblin's Turn**\n\u2694\ufe0f Attack roll"

	var sentContent string
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}

	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "chan-cl", "combat-map": "chan-cm"}, nil
		},
	}

	encLookup := &mockEnemyTurnEncounterLookup{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:          encounterID,
				Name:        "Cavern Skirmish",
				RoundNumber: 4,
			}, nil
		},
	}

	notifier := NewDiscordEnemyTurnNotifier(session, csp, nil)
	notifier.SetEncounterLookup(encLookup)
	notifier.NotifyEnemyTurnExecuted(context.Background(), encounterID, combatLog)

	want := "\u2694\ufe0f Cavern Skirmish \u2014 Round 4"
	if !strings.Contains(sentContent, want) {
		t.Errorf("expected combat log content to include label %q, got: %s", want, sentContent)
	}
	if !strings.Contains(sentContent, "Goblin's Turn") {
		t.Errorf("expected combat log content to still contain body, got: %s", sentContent)
	}
}

func TestDiscordEnemyTurnNotifier_CombatMapIncludesEncounterLabel(t *testing.T) {
	encounterID := uuid.New()

	var sentMapContent string
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return &discordgo.Message{}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentMapContent = data.Content
			return &discordgo.Message{}, nil
		},
	}

	csp := &mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "chan-cl", "combat-map": "chan-cm"}, nil
		},
	}

	mr := &mockMapRegenerator{
		regenerateMap: func(_ context.Context, _ uuid.UUID) ([]byte, error) {
			return []byte("png"), nil
		},
	}

	encLookup := &mockEnemyTurnEncounterLookup{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:          encounterID,
				Name:        "Cavern Skirmish",
				RoundNumber: 4,
			}, nil
		},
	}

	notifier := NewDiscordEnemyTurnNotifier(session, csp, mr)
	notifier.SetEncounterLookup(encLookup)
	notifier.NotifyEnemyTurnExecuted(context.Background(), encounterID, "log body")

	want := "\u2694\ufe0f Cavern Skirmish \u2014 Round 4"
	if !strings.Contains(sentMapContent, want) {
		t.Errorf("expected combat-map post to include label %q, got: %s", want, sentMapContent)
	}
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
