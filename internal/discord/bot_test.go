package discord

import (
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewBot(t *testing.T) {
	mock := &MockSession{
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	if bot == nil {
		t.Fatal("expected non-nil bot")
	}
	if bot.AppID() != "app-1" {
		t.Fatalf("expected app-1, got %s", bot.AppID())
	}
}

func TestBot_HandleGuildCreate_RegistersCommands(t *testing.T) {
	var registeredGuilds []string
	var mu sync.Mutex

	mock := &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			mu.Lock()
			registeredGuilds = append(registeredGuilds, guildID)
			mu.Unlock()
			return cmds, nil
		},
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())

	event := &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: "guild-1"},
	}
	bot.HandleGuildCreate(nil, event)

	mu.Lock()
	defer mu.Unlock()
	if len(registeredGuilds) != 1 || registeredGuilds[0] != "guild-1" {
		t.Fatalf("expected guild-1, got %v", registeredGuilds)
	}

	if !bot.HasGuild("guild-1") {
		t.Fatal("expected guild-1 to be tracked")
	}
}

func TestBot_HandleGuildCreate_TracksMultipleGuilds(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return cmds, nil
		},
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())

	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-1"}})
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-2"}})

	if !bot.HasGuild("g-1") || !bot.HasGuild("g-2") {
		t.Fatal("expected both guilds to be tracked")
	}
	if bot.HasGuild("g-3") {
		t.Fatal("g-3 should not be tracked")
	}
}

func TestBot_HandleGuildMemberAdd_SendsWelcomeDM(t *testing.T) {
	var sentUserID string
	var sentContent string
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			sentUserID = recipientID
			return &discordgo.Channel{ID: "dm-ch"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return cmds, nil
		},
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	bot.SetCampaignName("guild-1", "Curse of Strahd")

	event := &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			User:    &discordgo.User{ID: "user-1"},
			GuildID: "guild-1",
		},
	}
	bot.HandleGuildMemberAdd(nil, event)

	if sentUserID != "user-1" {
		t.Fatalf("expected user-1, got %s", sentUserID)
	}
	if sentContent == "" {
		t.Fatal("expected welcome message to be sent")
	}
}

func TestBot_HandleGuildMemberAdd_IgnoresBotUsers(t *testing.T) {
	var dmCalled bool
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			dmCalled = true
			return &discordgo.Channel{ID: "dm-ch"}, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())

	event := &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			User:    &discordgo.User{ID: "bot-user", Bot: true},
			GuildID: "guild-1",
		},
	}
	bot.HandleGuildMemberAdd(nil, event)

	if dmCalled {
		t.Fatal("should not DM bot users")
	}
}

func TestBot_HandleGuildMemberAdd_DefaultCampaignName(t *testing.T) {
	var sentContent string
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-ch"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	// No campaign name set for guild-1

	event := &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			User:    &discordgo.User{ID: "user-1"},
			GuildID: "guild-1",
		},
	}
	bot.HandleGuildMemberAdd(nil, event)

	if sentContent == "" {
		t.Fatal("expected welcome message to be sent")
	}
}

func TestBot_Guilds(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return cmds, nil
		},
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-1"}})
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-2"}})

	guilds := bot.Guilds()
	if len(guilds) != 2 {
		t.Fatalf("expected 2 guilds, got %d", len(guilds))
	}
}
