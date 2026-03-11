package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandDefinitions_NotEmpty(t *testing.T) {
	cmds := CommandDefinitions()
	if len(cmds) == 0 {
		t.Fatal("expected at least one command definition")
	}
}

func TestCommandDefinitions_AllHaveNames(t *testing.T) {
	for _, cmd := range CommandDefinitions() {
		if cmd.Name == "" {
			t.Fatal("found command with empty name")
		}
	}
}

func TestRegisterCommands_BulkOverwrites(t *testing.T) {
	var overwrittenGuild string
	var overwrittenCmds []*discordgo.ApplicationCommand

	mock := newTestMock()
	mock.ApplicationCommandBulkOverwriteFunc = func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
		overwrittenGuild = guildID
		overwrittenCmds = cmds
		return cmds, nil
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overwrittenGuild != "guild-1" {
		t.Fatalf("expected guild-1, got %s", overwrittenGuild)
	}
	if len(overwrittenCmds) != len(CommandDefinitions()) {
		t.Fatalf("expected %d commands, got %d", len(CommandDefinitions()), len(overwrittenCmds))
	}
}

func TestRegisterCommands_DeletesStaleCommands(t *testing.T) {
	var deletedIDs []string

	mock := newTestMock()
	mock.ApplicationCommandsFunc = func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
		return []*discordgo.ApplicationCommand{
			{ID: "stale-1", Name: "old-removed-command"},
		}, nil
	}
	mock.ApplicationCommandDeleteFunc = func(appID, guildID, cmdID string) error {
		deletedIDs = append(deletedIDs, cmdID)
		return nil
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != "stale-1" {
		t.Fatalf("expected stale-1 to be deleted, got %v", deletedIDs)
	}
}

func TestRegisterCommands_NoDeleteForCurrentCommands(t *testing.T) {
	defs := CommandDefinitions()
	existing := make([]*discordgo.ApplicationCommand, len(defs))
	for i, d := range defs {
		existing[i] = &discordgo.ApplicationCommand{ID: "id-" + d.Name, Name: d.Name}
	}

	var deletedIDs []string
	mock := newTestMock()
	mock.ApplicationCommandsFunc = func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
		return existing, nil
	}
	mock.ApplicationCommandDeleteFunc = func(appID, guildID, cmdID string) error {
		deletedIDs = append(deletedIDs, cmdID)
		return nil
	}

	err := RegisterCommands(mock, "app-1", "guild-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deletedIDs) != 0 {
		t.Fatalf("expected no deletions, got %v", deletedIDs)
	}
}
