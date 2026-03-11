package discord

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRegisterCommands_FetchError(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, errors.New("fetch failed")
		},
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterCommands_BulkOverwriteError(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return nil, errors.New("overwrite failed")
		},
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterCommands_DeleteError(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return []*discordgo.ApplicationCommand{{ID: "stale-1", Name: "stale-cmd"}}, nil
		},
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return cmds, nil
		},
		ApplicationCommandDeleteFunc: func(appID, guildID, cmdID string) error {
			return errors.New("delete failed")
		},
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBot_HandleGuildCreate_RegisterError(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, errors.New("fetch failed")
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	// Should not panic, just log the error.
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-1"}})

	// Guild should still be tracked even if registration fails.
	if !bot.HasGuild("g-1") {
		t.Fatal("expected guild to be tracked even after registration error")
	}
}

func TestBot_HandleGuildMemberAdd_DMError(t *testing.T) {
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return nil, errors.New("cannot DM")
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	// Should not panic, just log the error.
	bot.HandleGuildMemberAdd(nil, &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			User:    &discordgo.User{ID: "user-1"},
			GuildID: "g-1",
		},
	})
}

func TestWelcomeMessage_ContainsAllContent(t *testing.T) {
	msg := WelcomeMessage("Test Campaign")
	expected := []string{
		"Welcome to Test Campaign",
		"/create-character",
		"/import",
		"/register",
		"/help",
		"#character-cards",
		"#the-story",
		"DM approval",
	}
	for _, s := range expected {
		found := false
		for i := 0; i < len(msg)-len(s)+1; i++ {
			if msg[i:i+len(s)] == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in welcome message", s)
		}
	}
}
