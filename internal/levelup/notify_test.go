package levelup

import (
	"strings"
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
	if !strings.Contains(msg, "Aria") {
		t.Error("expected character name in message")
	}
	if !strings.Contains(msg, "Level: 6") {
		t.Error("expected new level in message")
	}
	if !strings.Contains(msg, "HP") {
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
	if !strings.Contains(msg, "ASI") {
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
	if !strings.Contains(msg, "subclass") || !strings.Contains(msg, "Subclass") {
		t.Error("expected subclass mention in message")
	}
}

func TestFormatASIPromptMessage(t *testing.T) {
	msg := FormatASIPromptMessage("Aria")
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if !strings.Contains(msg, "Ability Score Improvement") {
		t.Error("expected ASI mention in message")
	}
	if !strings.Contains(msg, "Aria") {
		t.Error("expected character name in message")
	}
}

func TestFormatASIApprovedMessage(t *testing.T) {
	msg := FormatASIApprovedMessage("Aria", "+2 STR (16 -> 18)")
	if !strings.Contains(msg, "Aria") {
		t.Error("expected character name")
	}
	if !strings.Contains(msg, "+2 STR") {
		t.Error("expected choice description")
	}
	if !strings.Contains(msg, "approved") && !strings.Contains(msg, "applied") {
		t.Error("expected approval indicator")
	}
}

