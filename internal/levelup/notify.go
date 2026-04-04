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
func FormatASIPromptMessage(characterName string, characterID uuid.UUID) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s** has earned an Ability Score Improvement!\n\n", characterName)
	b.WriteString("Choose one of the following:\n")
	b.WriteString("  [+2 to One Score] - Increase one ability score by 2 (max 20)\n")
	b.WriteString("  [+1 to Two Scores] - Increase two different ability scores by 1 each (max 20)\n")
	b.WriteString("  [Choose a Feat] - Select a feat from the available list\n")
	fmt.Fprintf(&b, "\nCharacter ID: %s\n", characterID.String())

	return b.String()
}

// FormatASIDeniedMessage returns the message sent when a DM denies an ASI choice.
func FormatASIDeniedMessage(characterName, reason string) string {
	return fmt.Sprintf("Your ASI/Feat choice for **%s** was not approved.\nReason: %s\nPlease make a new selection.", characterName, reason)
}
