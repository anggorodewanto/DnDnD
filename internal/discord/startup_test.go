package discord

import (
	"errors"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRegisterAllGuilds_RegistersEachGuild(t *testing.T) {
	var mu sync.Mutex
	registered := map[string]bool{}

	mock := &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			mu.Lock()
			registered[guildID] = true
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

	guildIDs := []string{"g-1", "g-2", "g-3"}
	errs := bot.RegisterAllGuilds(guildIDs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, id := range guildIDs {
		if !registered[id] {
			t.Fatalf("guild %s not registered", id)
		}
	}
}

func TestRegisterAllGuilds_CollectsErrors(t *testing.T) {
	mock := &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			if guildID == "g-bad" {
				return nil, errors.New("bulk overwrite failed")
			}
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

	errs := bot.RegisterAllGuilds([]string{"g-ok", "g-bad"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestRegisterAllGuilds_EmptyList(t *testing.T) {
	mock := &MockSession{
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	errs := bot.RegisterAllGuilds(nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestRegisterAllGuilds_TracksGuilds(t *testing.T) {
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
	bot.RegisterAllGuilds([]string{"g-1", "g-2"})

	if !bot.HasGuild("g-1") || !bot.HasGuild("g-2") {
		t.Fatal("expected guilds to be tracked after RegisterAllGuilds")
	}
}
