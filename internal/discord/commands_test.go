package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// commandMap builds a lookup map from command name to definition.
func commandMap() map[string]*discordgo.ApplicationCommand {
	cmds := CommandDefinitions()
	m := make(map[string]*discordgo.ApplicationCommand, len(cmds))
	for _, cmd := range cmds {
		m[cmd.Name] = cmd
	}
	return m
}

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

func TestCommandDefinitions_ParameterHints(t *testing.T) {
	cmdMap := commandMap()

	tests := []struct {
		name        string
		optionName  string
		optionType  discordgo.ApplicationCommandOptionType
		required    bool
	}{
		{"move", "coordinate", discordgo.ApplicationCommandOptionString, true},
		{"fly", "altitude", discordgo.ApplicationCommandOptionInteger, true},
		{"attack", "target", discordgo.ApplicationCommandOptionString, true},
		{"attack", "weapon", discordgo.ApplicationCommandOptionString, false},
		{"attack", "gwm", discordgo.ApplicationCommandOptionBoolean, false},
		{"attack", "twohanded", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "spell", discordgo.ApplicationCommandOptionString, true},
		{"cast", "target", discordgo.ApplicationCommandOptionString, false},
		{"cast", "level", discordgo.ApplicationCommandOptionInteger, false},
		{"cast", "subtle", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "twin", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "careful", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "heightened", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "distant", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "quickened", discordgo.ApplicationCommandOptionBoolean, false},
		{"cast", "empowered", discordgo.ApplicationCommandOptionBoolean, false},
		// E-66b: Extended Spell metamagic option must be exposed on /cast so
		// players can trigger it from Discord (handler already reads
		// metamagicFlags including "extended").
		{"cast", "extended", discordgo.ApplicationCommandOptionBoolean, false},
		{"bonus", "action", discordgo.ApplicationCommandOptionString, true},
		{"action", "action", discordgo.ApplicationCommandOptionString, true},
		{"action", "args", discordgo.ApplicationCommandOptionString, false},
		// E-71-followup-discord-ready-spell-flags: optional spell/slot for
		// /action ready threads into combat.ReadyActionCommand so the
		// service expends the slot + sets concentration at ready-time.
		{"action", "spell", discordgo.ApplicationCommandOptionString, false},
		{"action", "slot", discordgo.ApplicationCommandOptionInteger, false},
		{"shove", "target", discordgo.ApplicationCommandOptionString, true},
		{"interact", "description", discordgo.ApplicationCommandOptionString, true},
		{"command", "creature_id", discordgo.ApplicationCommandOptionString, true},
		{"command", "action", discordgo.ApplicationCommandOptionString, true},
		{"command", "target", discordgo.ApplicationCommandOptionString, false},
		{"check", "skill", discordgo.ApplicationCommandOptionString, true},
		{"check", "adv", discordgo.ApplicationCommandOptionBoolean, false},
		{"check", "disadv", discordgo.ApplicationCommandOptionBoolean, false},
		{"check", "target", discordgo.ApplicationCommandOptionString, false},
		{"save", "ability", discordgo.ApplicationCommandOptionString, true},
		{"rest", "type", discordgo.ApplicationCommandOptionString, true},
		{"whisper", "message", discordgo.ApplicationCommandOptionString, true},
		{"equip", "item", discordgo.ApplicationCommandOptionString, true},
		{"equip", "offhand", discordgo.ApplicationCommandOptionBoolean, false},
		{"undo", "reason", discordgo.ApplicationCommandOptionString, false},
		{"use", "item", discordgo.ApplicationCommandOptionString, true},
		{"give", "item", discordgo.ApplicationCommandOptionString, true},
		{"give", "target", discordgo.ApplicationCommandOptionString, true},
		{"attune", "item", discordgo.ApplicationCommandOptionString, true},
		{"unattune", "item", discordgo.ApplicationCommandOptionString, true},
		{"retire", "reason", discordgo.ApplicationCommandOptionString, false},
		{"register", "name", discordgo.ApplicationCommandOptionString, true},
		{"import", "ddb-url", discordgo.ApplicationCommandOptionString, true},
		{"recap", "rounds", discordgo.ApplicationCommandOptionInteger, false},
		{"distance", "target", discordgo.ApplicationCommandOptionString, true},
		{"distance", "target2", discordgo.ApplicationCommandOptionString, false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.optionName, func(t *testing.T) {
			cmd, ok := cmdMap[tt.name]
			if !ok {
				t.Fatalf("command %s not found", tt.name)
			}
			var found *discordgo.ApplicationCommandOption
			for _, opt := range cmd.Options {
				if opt.Name == tt.optionName {
					found = opt
					break
				}
			}
			if found == nil {
				t.Fatalf("option %s not found on command %s", tt.optionName, tt.name)
			}
			if found.Type != tt.optionType {
				t.Errorf("option %s type = %d, want %d", tt.optionName, found.Type, tt.optionType)
			}
			if found.Required != tt.required {
				t.Errorf("option %s required = %v, want %v", tt.optionName, found.Required, tt.required)
			}
		})
	}
}

func TestCommandDefinitions_NoParamCommands(t *testing.T) {
	noParamCmds := []string{
		"done", "deathsave", "status", "inventory", "loot",
		"create-character", "character",
	}

	cmdMap := commandMap()

	for _, name := range noParamCmds {
		t.Run(name, func(t *testing.T) {
			cmd, ok := cmdMap[name]
			if !ok {
				t.Fatalf("command %s not found", name)
			}
			if len(cmd.Options) != 0 {
				t.Errorf("expected no options for %s, got %d", name, len(cmd.Options))
			}
		})
	}
}

// TestCommandDefinitions_ReactionSubcommands verifies /reaction is a
// subcommand group with declare/cancel/cancel-all. Phase 106c restructured
// the command so the Discord handler can route declare vs cancel vs
// cancel-all to the combat.Service reaction APIs.
func TestCommandDefinitions_ReactionSubcommands(t *testing.T) {
	cmdMap := commandMap()
	cmd, ok := cmdMap["reaction"]
	if !ok {
		t.Fatal("/reaction command not found")
	}

	want := map[string]bool{"declare": false, "cancel": false, "cancel-all": false}
	for _, opt := range cmd.Options {
		if opt.Type != discordgo.ApplicationCommandOptionSubCommand {
			t.Fatalf("expected subcommand type for %s, got %d", opt.Name, opt.Type)
		}
		if _, exists := want[opt.Name]; exists {
			want[opt.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing /reaction subcommand: %s", name)
		}
	}

	// declare and cancel must carry a required "description" string option.
	for _, opt := range cmd.Options {
		if opt.Name != "declare" && opt.Name != "cancel" {
			continue
		}
		var desc *discordgo.ApplicationCommandOption
		for _, sub := range opt.Options {
			if sub.Name == "description" {
				desc = sub
				break
			}
		}
		if desc == nil {
			t.Fatalf("%s subcommand missing description option", opt.Name)
		}
		if desc.Type != discordgo.ApplicationCommandOptionString {
			t.Errorf("%s.description type = %d, want string", opt.Name, desc.Type)
		}
		if !desc.Required {
			t.Errorf("%s.description should be required", opt.Name)
		}
	}
}

func TestCommandDefinitions_SetupRequiresManageChannels(t *testing.T) {
	cmds := CommandDefinitions()
	for _, cmd := range cmds {
		if cmd.Name != "setup" {
			continue
		}
		if cmd.DefaultMemberPermissions == nil {
			t.Fatal("setup command should have DefaultMemberPermissions set")
		}
		if *cmd.DefaultMemberPermissions != discordgo.PermissionManageChannels {
			t.Fatalf("expected ManageChannels permission, got %d", *cmd.DefaultMemberPermissions)
		}
		return
	}
	t.Fatal("setup command not found")
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

func TestCommandDefinitions_ContainsAllExpectedCommands(t *testing.T) {
	expected := []string{
		"move", "fly", "attack", "cast", "bonus", "action", "shove",
		"interact", "done", "deathsave", "command", "reaction", "check",
		"save", "rest", "whisper", "status", "equip", "undo", "inventory",
		"use", "give", "loot", "attune", "unattune", "prepare", "retire",
		"register", "import", "create-character", "character", "recap",
		"distance", "help", "setup",
	}

	cmdMap := commandMap()

	for _, name := range expected {
		if _, ok := cmdMap[name]; !ok {
			t.Errorf("missing command: %s", name)
		}
	}

	if len(cmdMap) != len(expected) {
		t.Errorf("expected %d commands, got %d", len(expected), len(cmdMap))
	}
}

func TestCommandDefinitions_AllHaveDescriptions(t *testing.T) {
	for _, cmd := range CommandDefinitions() {
		if cmd.Description == "" {
			t.Errorf("command %s has empty description", cmd.Name)
		}
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
