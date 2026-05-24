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
	{discordgo.PermissionManageChannels, "Manage Channels"},
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
	// Administrator grants every permission in Discord, so nothing is missing.
	if granted&discordgo.PermissionAdministrator != 0 {
		return nil
	}
	var missing []string
	for _, p := range requiredPerms {
		if granted&p.bit == 0 {
			missing = append(missing, p.name)
		}
	}
	return missing
}

// guildPermissionsFor computes the bot's effective guild-level permission
// bitfield from the guild's role definitions and the bot member's roles.
// The bool is false when the bot's own member is not present in the guild
// payload (e.g. members truncated) — callers should skip validation rather
// than emit a false "missing permissions" warning.
func guildPermissionsFor(guild *discordgo.Guild, botUserID string) (int64, bool) {
	if guild == nil {
		return 0, false
	}
	if guild.OwnerID == botUserID {
		return discordgo.PermissionAdministrator, true
	}

	var botMember *discordgo.Member
	for _, m := range guild.Members {
		if m.User != nil && m.User.ID == botUserID {
			botMember = m
			break
		}
	}
	if botMember == nil {
		return 0, false
	}

	rolePerms := make(map[string]int64, len(guild.Roles))
	for _, r := range guild.Roles {
		rolePerms[r.ID] = r.Permissions
	}

	// The @everyone role shares the guild's ID and applies to every member.
	perms := rolePerms[guild.ID]
	for _, roleID := range botMember.Roles {
		perms |= rolePerms[roleID]
	}
	return perms, true
}
