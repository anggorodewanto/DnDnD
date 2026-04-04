package levelup

import (
	"testing"

	"github.com/google/uuid"
)

func TestFormatPublicLevelUpMessage(t *testing.T) {
	msg := FormatPublicLevelUpMessage("Aria", 6)
	want := "\U0001f389 Aria has reached Level 6!"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}

func TestFormatPrivateLevelUpMessage(t *testing.T) {
	details := LevelUpDetails{
		CharacterName:       "Aria",
		CharacterID:         uuid.New(),
		OldLevel:            5,
		NewLevel:            6,
		HPGained:            8,
		NewProficiencyBonus: 3,
		LeveledClass:        "fighter",
		LeveledClassLevel:   6,
		GrantsASI:           false,
		NeedsSubclass:       false,
	}
	msg := FormatPrivateLevelUpMessage(details)

	if msg == "" {
		t.Error("expected non-empty message")
	}
	// Should contain key info
	if !containsStr(msg, "Aria") {
		t.Error("expected character name in message")
	}
	if !containsStr(msg, "Level: 6") {
		t.Error("expected new level in message")
	}
	if !containsStr(msg, "HP") {
		t.Error("expected HP info in message")
	}
}

func TestFormatPrivateLevelUpMessage_WithASI(t *testing.T) {
	details := LevelUpDetails{
		CharacterName:     "Brom",
		CharacterID:       uuid.New(),
		NewLevel:          4,
		HPGained:          7,
		LeveledClass:      "fighter",
		LeveledClassLevel: 4,
		GrantsASI:         true,
	}
	msg := FormatPrivateLevelUpMessage(details)
	if !containsStr(msg, "ASI") {
		t.Error("expected ASI mention in message")
	}
}

func TestFormatPrivateLevelUpMessage_WithSubclass(t *testing.T) {
	details := LevelUpDetails{
		CharacterName:     "Cira",
		CharacterID:       uuid.New(),
		NewLevel:          3,
		HPGained:          6,
		LeveledClass:      "fighter",
		LeveledClassLevel: 3,
		NeedsSubclass:     true,
	}
	msg := FormatPrivateLevelUpMessage(details)
	if !containsStr(msg, "subclass") || !containsStr(msg, "Subclass") {
		t.Error("expected subclass mention in message")
	}
}

func TestFormatASIPromptMessage(t *testing.T) {
	msg := FormatASIPromptMessage("Aria", uuid.New())
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if !containsStr(msg, "+2") {
		t.Error("expected +2 option in message")
	}
	if !containsStr(msg, "+1") {
		t.Error("expected +1 option in message")
	}
	if !containsStr(msg, "Feat") || !containsStr(msg, "feat") {
		t.Error("expected feat option in message")
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
