package discord

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRequiredPermissions_Value(t *testing.T) {
	perms := RequiredPermissions()
	// Must include all five required permissions.
	expected := discordgo.PermissionSendMessages |
		discordgo.PermissionAttachFiles |
		discordgo.PermissionManageMessages |
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
	if len(missing) != 4 {
		t.Fatalf("expected 4 missing, got %d: %v", len(missing), missing)
	}
}

func TestValidatePermissions_NonePresent(t *testing.T) {
	missing := ValidatePermissions(0)
	if len(missing) != 5 {
		t.Fatalf("expected 5 missing, got %d: %v", len(missing), missing)
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

func TestBot_HandleGuildCreate_LogsMissingPermissions(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := newTestMock()
	bot := NewBot(mock, "app-1", logger)

	// Grant only SendMessages — missing 4 others.
	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{
			ID:          "guild-1",
			Permissions: int64(discordgo.PermissionSendMessages),
		},
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

	bot.HandleGuildCreate(nil, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{
			ID:          "guild-1",
			Permissions: RequiredPermissions(),
		},
	})

	if buf.Len() > 0 {
		t.Fatalf("expected no warning log when all permissions present, got: %s", buf.String())
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
