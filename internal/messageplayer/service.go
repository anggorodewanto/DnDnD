// Package messageplayer implements DM-initiated private messages delivered
// to a player via a Discord DM, logged in the dashboard for the DM's reference.
//
// The flow:
//  1. DM picks a player (player_character_id) and types a body.
//  2. Service validates input, looks up the PC's discord_user_id and checks
//     it belongs to the campaign, then sends a DM via the Messenger.
//  3. On successful send, the message is recorded in dm_player_messages for
//     later display in the dashboard history view.
//
// If Discord delivery fails, no log row is written — mirroring the narration
// service's post behavior.
package messageplayer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidInput is returned when a SendMessage call fails validation.
var ErrInvalidInput = errors.New("invalid message player input")

// ErrPlayerNotFound is returned when the player character cannot be found
// or does not belong to the requested campaign.
var ErrPlayerNotFound = errors.New("player character not found")

// ErrMessengerUnavailable is returned when no Discord Messenger has been
// wired up (e.g. the bot session is not connected).
var ErrMessengerUnavailable = errors.New("discord messenger not configured")

// Message is a logged DM-to-player message row.
type Message struct {
	ID                uuid.UUID `json:"id"`
	CampaignID        uuid.UUID `json:"campaign_id"`
	PlayerCharacterID uuid.UUID `json:"player_character_id"`
	AuthorUserID      string    `json:"author_user_id"`
	Body              string    `json:"body"`
	DiscordMessageIDs []string  `json:"discord_message_ids"`
	SentAt            time.Time `json:"sent_at"`
}

// InsertParams is the normalized payload a Store uses to record a message.
type InsertParams struct {
	CampaignID        uuid.UUID
	PlayerCharacterID uuid.UUID
	AuthorUserID      string
	Body              string
	DiscordMessageIDs []string
}

// Store is the persistence interface for DM-to-player messages.
type Store interface {
	InsertDMMessage(ctx context.Context, p InsertParams) (Message, error)
	ListDMMessages(ctx context.Context, campaignID, playerCharacterID uuid.UUID, limit, offset int) ([]Message, error)
}

// PlayerInfo is the resolved player character data the service needs to
// route a DM.
type PlayerInfo struct {
	DiscordUserID string
	CampaignID    uuid.UUID
}

// PlayerLookup resolves a player_character_id to the data needed to DM them.
type PlayerLookup interface {
	LookupPlayer(ctx context.Context, playerCharacterID uuid.UUID) (PlayerInfo, error)
}

// Messenger sends a DM to a Discord user and returns the resulting message
// IDs (multiple when the body is split across >2000 char messages).
type Messenger interface {
	SendDirectMessage(discordUserID, body string) ([]string, error)
}

// SendMessageInput is the caller-provided payload for SendMessage.
type SendMessageInput struct {
	CampaignID        uuid.UUID
	PlayerCharacterID uuid.UUID
	AuthorUserID      string
	Body              string
}

// Service orchestrates sending and recording DM-to-player messages.
type Service struct {
	store     Store
	lookup    PlayerLookup
	messenger Messenger
}

// NewService constructs a messageplayer Service with its dependencies.
func NewService(store Store, lookup PlayerLookup, messenger Messenger) *Service {
	return &Service{store: store, lookup: lookup, messenger: messenger}
}

// SendMessage validates input, looks up the player, sends the DM, and records
// the log row. On any failure before the Discord send completes, no row is
// recorded.
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (Message, error) {
	if s.messenger == nil {
		return Message{}, ErrMessengerUnavailable
	}
	if err := validateSend(in); err != nil {
		return Message{}, err
	}

	info, err := s.lookup.LookupPlayer(ctx, in.PlayerCharacterID)
	if err != nil {
		if errors.Is(err, ErrPlayerNotFound) {
			return Message{}, err
		}
		return Message{}, fmt.Errorf("looking up player character: %w", err)
	}
	if info.CampaignID != in.CampaignID {
		return Message{}, ErrPlayerNotFound
	}
	if strings.TrimSpace(info.DiscordUserID) == "" {
		return Message{}, ErrPlayerNotFound
	}

	messageIDs, err := s.messenger.SendDirectMessage(info.DiscordUserID, in.Body)
	if err != nil {
		return Message{}, fmt.Errorf("sending direct message: %w", err)
	}

	return s.store.InsertDMMessage(ctx, InsertParams{
		CampaignID:        in.CampaignID,
		PlayerCharacterID: in.PlayerCharacterID,
		AuthorUserID:      in.AuthorUserID,
		Body:              in.Body,
		DiscordMessageIDs: messageIDs,
	})
}

// History returns recent messages for a given player, newest-first.
func (s *Service) History(ctx context.Context, campaignID, playerCharacterID uuid.UUID, limit, offset int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.store.ListDMMessages(ctx, campaignID, playerCharacterID, limit, offset)
}

// validateSend enforces required fields on a SendMessage call.
func validateSend(in SendMessageInput) error {
	if in.CampaignID == uuid.Nil {
		return fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	if in.PlayerCharacterID == uuid.Nil {
		return fmt.Errorf("%w: player_character_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.AuthorUserID) == "" {
		return fmt.Errorf("%w: author_user_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.Body) == "" {
		return fmt.Errorf("%w: body required", ErrInvalidInput)
	}
	return nil
}
