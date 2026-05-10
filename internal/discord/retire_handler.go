package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/dmqueue"
)

// RetireHandler handles the /retire slash command. It does NOT call
// registration.Service.Retire — that runs DM-side from the dashboard's
// retire-approval handler. This handler only routes the request.
type RetireHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	notifier        dmqueue.Notifier
}

// NewRetireHandler constructs a RetireHandler.
func NewRetireHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	notifier dmqueue.Notifier,
) *RetireHandler {
	return &RetireHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		notifier:        notifier,
	}
}

// Handle processes a /retire interaction.
func (h *RetireHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	reason := optionString(interaction, "reason")
	if reason == "" {
		reason = "(no reason given)"
	}

	if h.campaignProv == nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}
	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	if h.characterLookup == nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	if h.notifier != nil {
		_, _ = h.notifier.Post(ctx, dmqueue.Event{
			Kind:       dmqueue.KindRetireRequest,
			PlayerName: char.Name,
			Summary:    fmt.Sprintf("requests retirement: %s", reason),
			GuildID:    interaction.GuildID,
		})
	}

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("🪦 Retire request sent to the DM. They'll review and approve from the dashboard.\n_Reason: %s_", reason))
}
