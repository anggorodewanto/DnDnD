package discord

import "github.com/bwmarrin/discordgo"

// permissionEntry pairs a permission bit with a human-readable label.
type permissionEntry struct {
	bit  int64
	name string
}

var requiredPerms = []permissionEntry{
	{discordgo.PermissionSendMessages, "Send Messages"},
	{discordgo.PermissionAttachFiles, "Attach Files"},
	{discordgo.PermissionManageMessages, "Manage Messages"},
	{int64(discordgo.PermissionUseSlashCommands), "Use Application Commands"},
	{discordgo.PermissionMentionEveryone, "Mention Everyone"},
}

// RequiredPermissions returns the combined permission bitfield the bot needs.
func RequiredPermissions() int64 {
	var perms int64
	for _, p := range requiredPerms {
		perms |= p.bit
	}
	return perms
}

// ValidatePermissions checks which required permissions are missing from the granted bitfield.
// Returns a slice of human-readable permission names that are not present.
func ValidatePermissions(granted int64) []string {
	var missing []string
	for _, p := range requiredPerms {
		if granted&p.bit == 0 {
			missing = append(missing, p.name)
		}
	}
	return missing
}
