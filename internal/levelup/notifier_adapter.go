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
// Public level-up announcements are deferred (they require a guild/channel
// resolution path that is not wired up yet) — SendPublicLevelUp is a
// no-op until a follow-up phase plumbs the public channel through.
type notifierAdapter struct {
	messenger DirectMessenger
}

// NewNotifierAdapter returns a Notifier backed by the given DM messenger.
// A nil messenger is tolerated so main.go can wire a functional adapter
// even when DISCORD_BOT_TOKEN is unset (every method becomes a no-op).
func NewNotifierAdapter(m DirectMessenger) Notifier {
	return &notifierAdapter{messenger: m}
}

// SendPublicLevelUp is a deliberate no-op: Phase 104c does not yet wire a
// public-channel poster for level-up announcements. A follow-up phase can
// resolve the campaign's story channel and route the formatted message
// through narration.Poster.
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
