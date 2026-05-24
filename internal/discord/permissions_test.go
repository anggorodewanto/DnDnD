package discord

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRequiredPermissions_Value(t *testing.T) {
	perms := RequiredPermissions()
	// Must include all six required permissions.
	expected := discordgo.PermissionSendMessages |
		discordgo.PermissionAttachFiles |
		discordgo.PermissionManageMessages |
		discordgo.PermissionManageChannels |
		discordgo.PermissionUseSlashCommands |
		discordgo.PermissionMentionEveryone

	if perms != int64(expected) {
		t.Fatalf("expected %d, got %d", expected, perms)
	}
}

func TestValidatePermissions_AllPresent(t *testing.T) {
	perms := RequiredPermissions()
	missing := ValidatePermissions(perms)
	if len(missing) != 0 {
		t.Fatalf("expected no missing permissions, got %v", missing)
	}
}

func TestValidatePermissions_SomeMissing(t *testing.T) {
	// Only send messages permission.
	granted := int64(discordgo.PermissionSendMessages)
	missing := ValidatePermissions(granted)
	if len(missing) != 5 {
		t.Fatalf("expected 5 missing, got %d: %v", len(missing), missing)
	}
}

func TestValidatePermissions_NonePresent(t *testing.T) {
	missing := ValidatePermissions(0)
	if len(missing) != 6 {
		t.Fatalf("expected 6 missing, got %d: %v", len(missing), missing)
	}
}

func TestValidatePermissions_AdminBypassesAll(t *testing.T) {
	// Admin has all permissions.
	granted := int64(discordgo.PermissionAdministrator) | RequiredPermissions()
	missing := ValidatePermissions(granted)
	if len(missing) != 0 {
		t.Fatalf("expected no missing with admin, got %v", missing)
	}
}

// botGuild builds a GUILD_CREATE-shaped guild where the bot ("app-1") is a
// member carrying a single role whose Permissions bitfield is `rolePerms`.
// This mirrors the real gateway payload: Guild.Permissions is left zero
// (Discord never populates it for bots) and effective perms are derived from
// role definitions instead.
func botGuild(guildID, botUserID string, rolePerms int64) *discordgo.Guild {
	const roleID = "role-bot"
	return &discordgo.Guild{
		ID: guildID,
		Roles: []*discordgo.Role{
			{ID: guildID, Permissions: 0}, // @everyone
			{ID: roleID, Permissions: rolePerms},
		},
		Members: []*discordgo.Member{
			{User: &discordgo.User{ID: botUserID}, Roles: []string{roleID}},
		},
	}
}

func TestBot_HandleGuildCreate_LogsMissingPermissions(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := newTestMock()
	bot := NewBot(mock, "app-1", logger)

	// Bot's role grants only SendMessages — missing the other five.
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{
		Guild: botGuild("guild-1", "app-1", int64(discordgo.PermissionSendMessages)),
	})

	output := buf.String()
	if output == "" {
		t.Fatal("expected warning log for missing permissions")
	}
	for _, name := range []string{"Attach Files", "Manage Messages", "Use Application Commands", "Mention Everyone"} {
		if !bytes.Contains(buf.Bytes(), []byte(name)) {
			t.Errorf("expected missing permission %q in log output", name)
		}
	}
}

func TestBot_HandleGuildCreate_NoLogWhenAllPermsPresent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := newTestMock()
	bot := NewBot(mock, "app-1", logger)

	// Bot's role grants every required permission via role definitions.
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{
		Guild: botGuild("guild-1", "app-1", RequiredPermissions()),
	})

	if buf.Len() > 0 {
		t.Fatalf("expected no warning log when all permissions present, got: %s", buf.String())
	}
}

func TestGuildPermissionsFor_BotRoleGrantsRequired(t *testing.T) {
	guild := botGuild("guild-1", "app-1", RequiredPermissions())

	perms, ok := guildPermissionsFor(guild, "app-1")
	if !ok {
		t.Fatal("expected ok=true when bot member is present")
	}
	if missing := ValidatePermissions(perms); len(missing) != 0 {
		t.Fatalf("expected no missing permissions, got %v", missing)
	}
}

func TestGuildPermissionsFor_EveryoneRoleCounts(t *testing.T) {
	// @everyone (ID == guild ID) carries the required perms; the bot's own
	// role grants nothing. Effective perms must still include @everyone's.
	guild := &discordgo.Guild{
		ID: "guild-1",
		Roles: []*discordgo.Role{
			{ID: "guild-1", Permissions: RequiredPermissions()}, // @everyone
			{ID: "role-bot", Permissions: 0},
		},
		Members: []*discordgo.Member{
			{User: &discordgo.User{ID: "app-1"}, Roles: []string{"role-bot"}},
		},
	}

	perms, ok := guildPermissionsFor(guild, "app-1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if missing := ValidatePermissions(perms); len(missing) != 0 {
		t.Fatalf("expected @everyone perms to count, got missing %v", missing)
	}
}

func TestGuildPermissionsFor_OwnerHasEverything(t *testing.T) {
	guild := &discordgo.Guild{
		ID:      "guild-1",
		OwnerID: "app-1",
	}

	perms, ok := guildPermissionsFor(guild, "app-1")
	if !ok {
		t.Fatal("expected ok=true for guild owner")
	}
	if missing := ValidatePermissions(perms); len(missing) != 0 {
		t.Fatalf("expected owner to have all permissions, got missing %v", missing)
	}
}

func TestGuildPermissionsFor_AdminRoleBypasses(t *testing.T) {
	// Bot role carries only the Administrator bit; ValidatePermissions must
	// report zero missing thanks to the admin bypass.
	guild := botGuild("guild-1", "app-1", int64(discordgo.PermissionAdministrator))

	perms, ok := guildPermissionsFor(guild, "app-1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if missing := ValidatePermissions(perms); len(missing) != 0 {
		t.Fatalf("expected admin bit to bypass all checks, got missing %v", missing)
	}
}

func TestGuildPermissionsFor_BotMemberAbsent(t *testing.T) {
	guild := &discordgo.Guild{
		ID: "guild-1",
		Roles: []*discordgo.Role{
			{ID: "guild-1", Permissions: RequiredPermissions()},
		},
		Members: []*discordgo.Member{
			{User: &discordgo.User{ID: "someone-else"}, Roles: nil},
		},
	}

	if _, ok := guildPermissionsFor(guild, "app-1"); ok {
		t.Fatal("expected ok=false when bot member is not in the payload")
	}
}

func TestGuildPermissionsFor_NilGuild(t *testing.T) {
	if _, ok := guildPermissionsFor(nil, "app-1"); ok {
		t.Fatal("expected ok=false for nil guild")
	}
}

func TestBot_ValidateGuildPermissions_LogsMissing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := newTestMock()
	bot := NewBot(mock, "app-1", logger)

	bot.ValidateGuildPermissions("guild-1", 0)

	output := buf.String()
	if output == "" {
		t.Fatal("expected warning log for missing permissions")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Send Messages")) {
		t.Error("expected 'Send Messages' in log output")
	}
}

func TestBot_ValidateGuildPermissions_NoLogWhenFull(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := newTestMock()
	bot := NewBot(mock, "app-1", logger)

	bot.ValidateGuildPermissions("guild-1", RequiredPermissions())

	if buf.Len() > 0 {
		t.Fatalf("expected no log output, got: %s", buf.String())
	}
}
