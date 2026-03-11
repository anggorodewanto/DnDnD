package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// setupPermission requires ManageChannels to run /setup.
var setupPermission int64 = discordgo.PermissionManageChannels

// CommandDefinitions returns the full set of slash commands the bot registers per guild.
func CommandDefinitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "create-character",
			Description: "Build a character in the web portal",
		},
		{
			Name:        "import",
			Description: "Import a character from D&D Beyond",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ddb-url",
					Description: "D&D Beyond character URL",
					Required:    true,
				},
			},
		},
		{
			Name:        "register",
			Description: "Link to a character your DM already created",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Character name",
					Required:    true,
				},
			},
		},
		{
			Name:        "help",
			Description: "Show a full command list",
		},
		{
			Name:                     "setup",
			Description:              "Create the full channel structure for this campaign",
			DefaultMemberPermissions: &setupPermission,
		},
	}
}

// RegisterCommands registers the current command set for a guild and deletes stale commands.
func RegisterCommands(s Session, appID, guildID string) error {
	// Fetch existing commands to detect stale ones.
	existing, err := s.ApplicationCommands(appID, guildID)
	if err != nil {
		return fmt.Errorf("fetching existing commands for guild %s: %w", guildID, err)
	}

	// Bulk overwrite with current definitions.
	defs := CommandDefinitions()
	_, err = s.ApplicationCommandBulkOverwrite(appID, guildID, defs)
	if err != nil {
		return fmt.Errorf("bulk overwriting commands for guild %s: %w", guildID, err)
	}

	// Build set of current command names.
	currentNames := make(map[string]bool, len(defs))
	for _, d := range defs {
		currentNames[d.Name] = true
	}

	// Delete stale commands not in the current set.
	for _, cmd := range existing {
		if currentNames[cmd.Name] {
			continue
		}
		if err := s.ApplicationCommandDelete(appID, guildID, cmd.ID); err != nil {
			return fmt.Errorf("deleting stale command %s in guild %s: %w", cmd.Name, guildID, err)
		}
	}

	return nil
}
