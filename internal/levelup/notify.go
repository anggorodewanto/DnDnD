package levelup

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// FormatPublicLevelUpMessage returns the public announcement for #the-story.
func FormatPublicLevelUpMessage(characterName string, newLevel int) string {
	return fmt.Sprintf("\U0001f389 %s has reached Level %d!", characterName, newLevel)
}

// FormatPrivateLevelUpMessage returns the detailed private message for the player.
func FormatPrivateLevelUpMessage(details LevelUpDetails) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s** leveled up! (%s %d)\n", details.CharacterName, details.LeveledClass, details.LeveledClassLevel)
	fmt.Fprintf(&b, "Total Level: %d\n", details.NewLevel)
	fmt.Fprintf(&b, "HP gained: +%d\n", details.HPGained)

	if details.NewProficiencyBonus > 0 {
		fmt.Fprintf(&b, "Proficiency Bonus: +%d\n", details.NewProficiencyBonus)
	}

	if details.GrantsASI {
		b.WriteString("\n\u23f3 **ASI/Feat pending** - Choose your ability score improvement or feat!\n")
	}

	if details.NeedsSubclass {
		b.WriteString("\n\u2728 **Subclass selection needed** - The DM will help you choose a subclass.\n")
	}

	return b.String()
}

// FormatASIPromptMessage returns the interactive prompt message for ASI/Feat choice.
// This text accompanies interactive button components in the Discord message.
func FormatASIPromptMessage(characterName string, characterID uuid.UUID) string {
	return fmt.Sprintf("\U0001f393 **%s** — Ability Score Improvement available! Choose below:", characterName)
}

// FormatASIApprovedMessage returns the confirmation message when an ASI/Feat is approved.
func FormatASIApprovedMessage(characterName, choiceDescription string) string {
	return fmt.Sprintf("\u2705 %s applied for **%s**!", choiceDescription, characterName)
}

// FormatASIDeniedMessage returns the message sent when a DM denies an ASI choice.
func FormatASIDeniedMessage(characterName, reason string) string {
	return fmt.Sprintf("Your ASI/Feat choice for **%s** was not approved.\nReason: %s\nPlease make a new selection.", characterName, reason)
}
