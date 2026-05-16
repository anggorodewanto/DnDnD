package discord

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// WhisperCampaignProvider resolves a guild to its campaign.
type WhisperCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// WhisperCharacterLookup resolves a Discord user to their character.
type WhisperCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// WhisperPoster posts a whisper event to the dm-queue.
type WhisperPoster interface {
	PostWhisper(ctx context.Context, player, message, discordUserID, guildID, campaignID string) (string, error)
}

// WhisperHandler handles the /whisper slash command.
type WhisperHandler struct {
	session          Session
	campaignProvider WhisperCampaignProvider
	characterLookup  WhisperCharacterLookup
	poster           WhisperPoster
}

// NewWhisperHandler creates a new WhisperHandler.
func NewWhisperHandler(
	session Session,
	campaignProvider WhisperCampaignProvider,
	characterLookup WhisperCharacterLookup,
) *WhisperHandler {
	return &WhisperHandler{
		session:          session,
		campaignProvider: campaignProvider,
		characterLookup:  characterLookup,
	}
}

// SetNotifier wires the WhisperPoster so /whisper posts to the dm-queue.
func (h *WhisperHandler) SetNotifier(p WhisperPoster) { h.poster = p }

// Handle processes the /whisper command interaction.
func (h *WhisperHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	message := optionString(interaction, "message")
	if strings.TrimSpace(message) == "" {
		respondEphemeral(h.session, interaction, "Please provide a message.")
		return
	}
	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	if h.poster == nil {
		respondEphemeral(h.session, interaction, "Whisper service is not available.")
		return
	}

	_, err = h.poster.PostWhisper(ctx, char.Name, message, userID, interaction.GuildID, campaign.ID.String())
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to send whisper. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, "🤫 Whisper sent to the DM.")
}
