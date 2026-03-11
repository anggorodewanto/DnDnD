package discord

import "fmt"

// WelcomeMessage returns the welcome DM text for a given campaign name.
func WelcomeMessage(campaignName string) string {
	return fmt.Sprintf(`Welcome to %s! Here's how to get started:

1. Create or import your character:
   • /create-character — build a character in the web portal
   • /import <ddb-url> — import from D&D Beyond
   • /register <name> — link to a character your DM already created

2. Wait for DM approval (you'll be pinged when approved)

3. Once approved, check #character-cards for your sheet and #the-story to catch up

Type /help for a full command list.`, campaignName)
}

// SendWelcomeDM sends the welcome message to a user via DM.
func SendWelcomeDM(s Session, userID, campaignName string) error {
	ch, err := s.UserChannelCreate(userID)
	if err != nil {
		return fmt.Errorf("creating DM channel for user %s: %w", userID, err)
	}

	msg := WelcomeMessage(campaignName)
	_, err = s.ChannelMessageSend(ch.ID, msg)
	if err != nil {
		return fmt.Errorf("sending welcome DM to user %s: %w", userID, err)
	}

	return nil
}
