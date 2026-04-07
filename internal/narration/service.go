package narration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidInput is returned when a Post call fails validation (e.g. empty body).
var ErrInvalidInput = errors.New("invalid narration input")

// ErrAttachmentNotFound is returned when one of the attachment asset IDs does
// not resolve to a known asset URL.
var ErrAttachmentNotFound = errors.New("attachment asset not found")

// ErrPosterUnavailable is returned by Post when no Discord Poster has been
// wired up yet (e.g. the bot session is not connected).
var ErrPosterUnavailable = errors.New("discord poster not configured")

// Post is a recorded narration post row returned from the service.
type Post struct {
	ID                 uuid.UUID   `json:"id"`
	CampaignID         uuid.UUID   `json:"campaign_id"`
	AuthorUserID       string      `json:"author_user_id"`
	Body               string      `json:"body"`
	AttachmentAssetIDs []uuid.UUID `json:"attachment_asset_ids"`
	DiscordMessageIDs  []string    `json:"discord_message_ids"`
	PostedAt           time.Time   `json:"posted_at"`
}

// InsertPostParams is the normalized input the Store uses to record a post.
type InsertPostParams struct {
	CampaignID         uuid.UUID
	AuthorUserID       string
	Body               string
	AttachmentAssetIDs []uuid.UUID
	DiscordMessageIDs  []string
}

// Store is the persistence interface for narration posts.
type Store interface {
	InsertNarrationPost(ctx context.Context, p InsertPostParams) (Post, error)
	ListNarrationPostsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Post, error)
}

// Poster sends a rendered narration to the guild's #the-story channel and
// returns the resulting Discord message IDs (multiple when the body was split
// across >2000 char messages).
type Poster interface {
	PostToStory(guildID, body string, embeds []DiscordEmbed, attachmentURLs []string) ([]string, error)
}

// AttachmentResolver turns an asset UUID into the URL that will be embedded
// in the Discord message. Implementations typically wrap asset.Service.URL
// and verify the asset exists and belongs to the campaign.
type AttachmentResolver interface {
	AttachmentURL(assetID uuid.UUID) (string, bool)
}

// CampaignResolver looks up the Discord guild ID owning a campaign, so that
// the service can route posts to the right server's #the-story channel.
type CampaignResolver interface {
	GuildIDForCampaign(ctx context.Context, id uuid.UUID) (string, error)
}

// PostInput holds the caller-provided narration post parameters.
type PostInput struct {
	CampaignID         uuid.UUID
	AuthorUserID       string
	Body               string
	AttachmentAssetIDs []uuid.UUID
}

// Service orchestrates composing, posting, and recording narrations.
type Service struct {
	store     Store
	poster    Poster
	assets    AttachmentResolver
	campaigns CampaignResolver
}

// NewService constructs a narration Service with its dependencies.
func NewService(store Store, poster Poster, assets AttachmentResolver, campaigns CampaignResolver) *Service {
	return &Service{
		store:     store,
		poster:    poster,
		assets:    assets,
		campaigns: campaigns,
	}
}

// Post validates input, renders the body to Discord format, posts to the
// guild's #the-story channel, and records the post on success. On any
// failure before the Discord call completes successfully, no post row is
// created.
func (s *Service) Post(ctx context.Context, in PostInput) (Post, error) {
	if s.poster == nil {
		return Post{}, ErrPosterUnavailable
	}
	if err := validatePost(in); err != nil {
		return Post{}, err
	}

	urls, err := s.resolveAttachments(in.AttachmentAssetIDs)
	if err != nil {
		return Post{}, err
	}

	guildID, err := s.campaigns.GuildIDForCampaign(ctx, in.CampaignID)
	if err != nil {
		return Post{}, fmt.Errorf("looking up campaign guild: %w", err)
	}

	rendered := RenderDiscord(in.Body)
	messageIDs, err := s.poster.PostToStory(guildID, rendered.Body, rendered.Embeds, urls)
	if err != nil {
		return Post{}, fmt.Errorf("posting to #the-story: %w", err)
	}

	return s.store.InsertNarrationPost(ctx, InsertPostParams{
		CampaignID:         in.CampaignID,
		AuthorUserID:       in.AuthorUserID,
		Body:               in.Body,
		AttachmentAssetIDs: in.AttachmentAssetIDs,
		DiscordMessageIDs:  messageIDs,
	})
}

// History returns recent posts for a campaign, newest-first.
func (s *Service) History(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Post, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.store.ListNarrationPostsByCampaign(ctx, campaignID, limit, offset)
}

// resolveAttachments maps asset IDs to their public URLs, returning
// ErrAttachmentNotFound if any are missing.
func (s *Service) resolveAttachments(ids []uuid.UUID) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		url, ok := s.assets.AttachmentURL(id)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrAttachmentNotFound, id)
		}
		urls = append(urls, url)
	}
	return urls, nil
}

// validatePost enforces required fields on a Post call.
func validatePost(in PostInput) error {
	if in.CampaignID == uuid.Nil {
		return fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.AuthorUserID) == "" {
		return fmt.Errorf("%w: author_user_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.Body) == "" {
		return fmt.Errorf("%w: body required", ErrInvalidInput)
	}
	return nil
}
