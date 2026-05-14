package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRegisterCommands_BulkOverwriteError(t *testing.T) {
	mock := newTestMock()
	mock.ApplicationCommandBulkOverwriteFunc = func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
		return nil, errors.New("overwrite failed")
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBot_HandleGuildCreate_RegisterError(t *testing.T) {
	mock := newTestMock()
	mock.ApplicationCommandsFunc = func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
		return nil, errors.New("fetch failed")
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "g-1"}})

	if !bot.HasGuild("g-1") {
		t.Fatal("expected guild to be tracked even after registration error")
	}
}

func TestBot_HandleGuildMemberAdd_DMError(t *testing.T) {
	mock := newTestMock()
	mock.UserChannelCreateFunc = func(recipientID string) (*discordgo.Channel, error) {
		return nil, errors.New("cannot DM")
	}

	bot := NewBot(mock, "app-1", newTestLogger())
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
		if !strings.Contains(msg, s) {
			t.Errorf("expected %q in welcome message", s)
		}
	}
}
