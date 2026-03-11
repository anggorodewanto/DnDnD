package discord

import (
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
