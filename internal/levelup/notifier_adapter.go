package levelup

import (
	"context"

	"github.com/google/uuid"
)

// DirectMessenger abstracts the subset of a Discord DM client the level-up
// notifier needs. *discord.DirectMessenger satisfies it out of the box.
type DirectMessenger interface {
	SendDirectMessage(discordUserID, body string) ([]string, error)
}

// notifierAdapter bridges a DM-capable messenger onto the Notifier contract.
// A nil messenger (or empty discord user id) makes every method a silent
// no-op so main.go can wire the adapter even when DISCORD_BOT_TOKEN is unset.
type notifierAdapter struct {
	messenger DirectMessenger
}

// NewNotifierAdapter returns a Notifier backed by the given DM messenger.
func NewNotifierAdapter(m DirectMessenger) Notifier {
	return &notifierAdapter{messenger: m}
}

// SendPublicLevelUp is deliberately a no-op: public-channel posting is
// deferred to a follow-up phase that resolves the campaign's story channel
// and routes through narration.Poster.
func (a *notifierAdapter) SendPublicLevelUp(_ context.Context, _ string, _ int) error {
	return nil
}

func (a *notifierAdapter) SendPrivateLevelUp(_ context.Context, discordUserID string, details LevelUpDetails) error {
	if a.messenger == nil || discordUserID == "" {
		return nil
	}
	body := FormatPrivateLevelUpMessage(details)
	_, err := a.messenger.SendDirectMessage(discordUserID, body)
	return err
}

func (a *notifierAdapter) SendASIPrompt(_ context.Context, discordUserID string, _ uuid.UUID, characterName string) error {
	if a.messenger == nil || discordUserID == "" {
		return nil
	}
	body := FormatASIPromptMessage(characterName)
	_, err := a.messenger.SendDirectMessage(discordUserID, body)
	return err
}

func (a *notifierAdapter) SendASIDenied(_ context.Context, discordUserID string, characterName string, reason string) error {
	if a.messenger == nil || discordUserID == "" {
		return nil
	}
	body := FormatASIDeniedMessage(characterName, reason)
	_, err := a.messenger.SendDirectMessage(discordUserID, body)
	return err
}
