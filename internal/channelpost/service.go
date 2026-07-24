// Package channelpost lets a DM broadcast arbitrary text to any of a
// campaign's configured Discord channels as the bot, from the dashboard. It
// reuses the narration read-aloud renderer so :::read-aloud fences still render
// as gold embeds, and resolves the target channel by name key against the
// campaign's settings.channel_ids rather than a live guild scan.
package channelpost

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/narration"
)

// ErrInvalidInput is returned when a Post/Channels call fails validation.
var ErrInvalidInput = errors.New("invalid channel-post input")

// ErrUnknownChannel is returned when the requested channel key is not one of
// the campaign's configured channels (or maps to an empty ID).
var ErrUnknownChannel = errors.New("unknown channel")

// ErrPosterUnavailable is returned when no Discord poster is wired up (bot
// offline / DB-only test deploys).
var ErrPosterUnavailable = errors.New("discord poster not configured")

// ChannelLookup resolves a campaign's channel name -> Discord channel ID map.
// campaignChannelLookup in cmd/dndnd already satisfies this.
type ChannelLookup interface {
	GetChannelIDsForCampaign(ctx context.Context, campaignID uuid.UUID) (map[string]string, error)
}

// Poster sends a rendered message to a Discord channel by ID as the bot,
// returning the resulting message IDs (multiple when the body was split across
// >2000-char messages). discord.ChannelPoster satisfies this.
type Poster interface {
	PostToChannel(channelID, body string, embeds []narration.DiscordEmbed) ([]string, error)
}

// PostInput holds the caller-provided parameters for a channel post.
type PostInput struct {
	CampaignID uuid.UUID
	Channel    string // channel name key, e.g. "in-character"
	Body       string
}

// PostResult reports what was sent, for the dashboard to display.
type PostResult struct {
	Channel           string   `json:"channel"`
	ChannelID         string   `json:"channel_id"`
	DiscordMessageIDs []string `json:"discord_message_ids"`
}

// Service posts arbitrary DM text to a campaign's channels as the bot.
type Service struct {
	channels ChannelLookup
	poster   Poster
}

// NewService constructs a Service. poster may be nil (bot offline) in which
// case Post returns ErrPosterUnavailable.
func NewService(channels ChannelLookup, poster Poster) *Service {
	return &Service{channels: channels, poster: poster}
}

// Post validates input, resolves the channel name key to a Discord channel ID
// via the campaign's settings.channel_ids, renders the body (read-aloud fences
// -> embeds), and sends it as the bot. Nothing is persisted — this is a
// fire-and-forward broadcast, unlike narration which records a post row.
func (s *Service) Post(ctx context.Context, in PostInput) (PostResult, error) {
	if s.poster == nil {
		return PostResult{}, ErrPosterUnavailable
	}
	if err := validatePost(in); err != nil {
		return PostResult{}, err
	}

	channelIDs, err := s.channels.GetChannelIDsForCampaign(ctx, in.CampaignID)
	if err != nil {
		return PostResult{}, fmt.Errorf("looking up campaign channels: %w", err)
	}
	channelID, ok := channelIDs[in.Channel]
	if !ok || channelID == "" {
		return PostResult{}, fmt.Errorf("%w: %s", ErrUnknownChannel, in.Channel)
	}

	rendered := narration.RenderDiscord(in.Body)
	ids, err := s.poster.PostToChannel(channelID, rendered.Body, rendered.Embeds)
	if err != nil {
		return PostResult{}, fmt.Errorf("posting to #%s: %w", in.Channel, err)
	}
	return PostResult{Channel: in.Channel, ChannelID: channelID, DiscordMessageIDs: ids}, nil
}

// Channels returns the campaign's configured channel name keys (those with a
// non-empty ID), sorted, for populating the dashboard dropdown.
func (s *Service) Channels(ctx context.Context, campaignID uuid.UUID) ([]string, error) {
	if campaignID == uuid.Nil {
		return nil, fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	channelIDs, err := s.channels.GetChannelIDsForCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("looking up campaign channels: %w", err)
	}
	keys := make([]string, 0, len(channelIDs))
	for k, v := range channelIDs {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func validatePost(in PostInput) error {
	if in.CampaignID == uuid.Nil {
		return fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.Channel) == "" {
		return fmt.Errorf("%w: channel required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.Body) == "" {
		return fmt.Errorf("%w: body required", ErrInvalidInput)
	}
	return nil
}
