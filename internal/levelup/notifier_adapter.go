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

// asiPromptMessenger is an optional capability: a DirectMessenger that can
// deliver the ASI prompt WITH interactive buttons (carrying the character ID
// so clicks route back correctly). *discord.DirectMessenger implements it.
// Messengers without this method (headless/nil, test fakes) fall back to a
// plain-text DM — which renders no buttons, so the buttoned path is required
// for the player to actually make an ASI/Feat choice.
type asiPromptMessenger interface {
	SendASIPrompt(discordUserID string, characterID uuid.UUID, body string) ([]string, error)
}

// StoryPoster abstracts the public-channel surface (#the-story) used for
// level-up announcements. The adapter in cmd/dndnd resolves the
// character's campaign → guild and routes through narration.Poster, so the
// levelup package stays free of guild-resolution concerns.
type StoryPoster interface {
	PostPublicLevelUp(ctx context.Context, characterID uuid.UUID, characterName string, newLevel int) error
}

// notifierAdapter bridges a DM-capable messenger onto the Notifier contract.
// A nil messenger (or empty discord user id) makes every method a silent
// no-op so main.go can wire the adapter even when DISCORD_BOT_TOKEN is unset.
// The optional StoryPoster wires the public-channel level-up announcement;
// when nil, SendPublicLevelUp degrades to a silent no-op (same headless
// contract as the DM path).
type notifierAdapter struct {
	messenger DirectMessenger
	story     StoryPoster
}

// NewNotifierAdapter returns a Notifier backed by the given DM messenger.
// SendPublicLevelUp degrades to a no-op until a StoryPoster is wired via
// NewNotifierAdapterWithStory.
func NewNotifierAdapter(m DirectMessenger) Notifier {
	return &notifierAdapter{messenger: m}
}

// NewNotifierAdapterWithStory returns a Notifier that posts public
// level-up announcements through the given StoryPoster and DMs through m.
// Either dependency may be nil; the corresponding surface no-ops.
func NewNotifierAdapterWithStory(m DirectMessenger, s StoryPoster) Notifier {
	return &notifierAdapter{messenger: m, story: s}
}

// SendPublicLevelUp posts a public level-up announcement via the wired
// StoryPoster. When no StoryPoster is configured (headless deploys), the
// call degrades to a silent no-op so HTTP-driven level-ups still succeed.
func (a *notifierAdapter) SendPublicLevelUp(ctx context.Context, characterID uuid.UUID, characterName string, newLevel int) error {
	if a.story == nil {
		return nil
	}
	return a.story.PostPublicLevelUp(ctx, characterID, characterName, newLevel)
}

func (a *notifierAdapter) SendPrivateLevelUp(_ context.Context, discordUserID string, details LevelUpDetails) error {
	if a.messenger == nil || discordUserID == "" {
		return nil
	}
	body := FormatPrivateLevelUpMessage(details)
	_, err := a.messenger.SendDirectMessage(discordUserID, body)
	return err
}

func (a *notifierAdapter) SendASIPrompt(_ context.Context, discordUserID string, characterID uuid.UUID, characterName string) error {
	if a.messenger == nil || discordUserID == "" {
		return nil
	}
	body := FormatASIPromptMessage(characterName)
	if pm, ok := a.messenger.(asiPromptMessenger); ok {
		_, err := pm.SendASIPrompt(discordUserID, characterID, body)
		return err
	}
	// Fallback: no button capability — send plain text (renders no buttons).
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
