package dashboard

import (
	"context"

	"github.com/google/uuid"
)

// ApprovalEntry represents a pending player character with character name for the approval list.
type ApprovalEntry struct {
	ID            uuid.UUID `json:"id"`
	CharacterID   uuid.UUID `json:"character_id"`
	CharacterName string    `json:"character_name"`
	DiscordUserID string    `json:"discord_user_id"`
	Status        string    `json:"status"`
	CreatedVia    string    `json:"created_via"`
	DmFeedback    string    `json:"dm_feedback,omitempty"`
}

// ApprovalDetail represents a full character sheet for DM review.
type ApprovalDetail struct {
	ApprovalEntry
	Race          string `json:"race"`
	Level         int32  `json:"level"`
	Classes       string `json:"classes"`
	HpMax         int32  `json:"hp_max"`
	HpCurrent     int32  `json:"hp_current"`
	Ac            int32  `json:"ac"`
	SpeedFt       int32  `json:"speed_ft"`
	AbilityScores string `json:"ability_scores"`
	Languages     string `json:"languages"`
	DdbURL        string `json:"ddb_url,omitempty"`
}

// ApprovalStore is the interface for approval queue data access.
type ApprovalStore interface {
	ListPendingApprovals(ctx context.Context, campaignID uuid.UUID) ([]ApprovalEntry, error)
	GetApprovalDetail(ctx context.Context, id uuid.UUID) (*ApprovalDetail, error)
	ApproveCharacter(ctx context.Context, id uuid.UUID) error
	RetireCharacter(ctx context.Context, id uuid.UUID) error
	RequestChanges(ctx context.Context, id uuid.UUID, feedback string) error
	RejectCharacter(ctx context.Context, id uuid.UUID, feedback string) error
}

// CharacterCardPoster posts and updates character cards in the #character-cards channel.
type CharacterCardPoster interface {
	PostCharacterCard(ctx context.Context, characterID uuid.UUID, characterName, discordUserID string) error
	UpdateCardRetired(ctx context.Context, characterID uuid.UUID, characterName, discordUserID string) error
}

// PlayerNotifier is an interface for sending notifications to players.
// This will be connected to Discord DM functionality later.
type PlayerNotifier interface {
	NotifyApproval(ctx context.Context, discordUserID string, characterName string) error
	NotifyChangesRequested(ctx context.Context, discordUserID string, characterName string, feedback string) error
	NotifyRejection(ctx context.Context, discordUserID string, characterName string, feedback string) error
}
