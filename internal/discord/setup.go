package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// CategoryDef defines a category and its child channels for server setup.
type CategoryDef struct {
	Name     string
	Channels []ChannelDef
}

// ChannelDef defines a text channel within a category.
type ChannelDef struct {
	Name string
	// PermissionFunc returns permission overwrites for this channel.
	// Parameters: guildID (used as @everyone role ID), botUserID, dmUserID.
	// If nil, no special permissions are applied.
	PermissionFunc func(guildID, botUserID, dmUserID string) []*discordgo.PermissionOverwrite
}

// ChannelStructure returns the full channel structure the bot creates via /setup.
func ChannelStructure() []CategoryDef {
	return []CategoryDef{
		{
			Name: "SYSTEM",
			Channels: []ChannelDef{
				{Name: "initiative-tracker"},
				{Name: "combat-log"},
				{Name: "roll-history"},
			},
		},
		{
			Name: "NARRATION",
			Channels: []ChannelDef{
				{Name: "the-story", PermissionFunc: theStoryPerms},
				{Name: "in-character"},
				{Name: "player-chat"},
			},
		},
		{
			Name: "COMBAT",
			Channels: []ChannelDef{
				{Name: "combat-map", PermissionFunc: combatMapPerms},
				{Name: "your-turn"},
			},
		},
		{
			Name: "REFERENCE",
			Channels: []ChannelDef{
				{Name: "character-cards"},
				{Name: "dm-queue"},
			},
		},
	}
}

// theStoryPerms returns overwrites making #the-story DM-write-only.
// @everyone is denied SendMessages; the DM is explicitly allowed.
func theStoryPerms(guildID, _, dmUserID string) []*discordgo.PermissionOverwrite {
	return []*discordgo.PermissionOverwrite{
		{
			ID:   guildID, // @everyone role ID == guild ID
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionSendMessages,
		},
		{
			ID:    dmUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages,
		},
	}
}

// combatMapPerms returns overwrites making #combat-map bot-write-only.
// @everyone is denied SendMessages; the bot is explicitly allowed.
func combatMapPerms(guildID, botUserID, _ string) []*discordgo.PermissionOverwrite {
	return []*discordgo.PermissionOverwrite{
		{
			ID:   guildID,
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionSendMessages,
		},
		{
			ID:    botUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages,
		},
	}
}

// SetupChannels creates the full category/channel structure for a guild.
// It skips categories and channels that already exist (matched by name).
// Returns a map of channel name -> channel ID for storage in campaign settings.
func SetupChannels(s Session, guildID, botUserID, dmUserID string) (map[string]string, error) {
	existing, err := s.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("fetching guild channels: %w", err)
	}

	// Build lookup maps for existing categories and channels.
	existingCategories := make(map[string]*discordgo.Channel)
	existingChannels := make(map[string]*discordgo.Channel) // key: "parentID/name"
	for _, ch := range existing {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			existingCategories[ch.Name] = ch
			continue
		}
		if ch.Type == discordgo.ChannelTypeGuildText {
			key := ch.ParentID + "/" + ch.Name
			existingChannels[key] = ch
		}
	}

	result := make(map[string]string)
	structure := ChannelStructure()

	for _, catDef := range structure {
		catID, err := ensureCategory(s, guildID, catDef.Name, existingCategories)
		if err != nil {
			return nil, err
		}

		for _, chDef := range catDef.Channels {
			key := catID + "/" + chDef.Name
			if existingCh, ok := existingChannels[key]; ok {
				result[chDef.Name] = existingCh.ID
				continue
			}

			var overwrites []*discordgo.PermissionOverwrite
			if chDef.PermissionFunc != nil {
				overwrites = chDef.PermissionFunc(guildID, botUserID, dmUserID)
			}

			ch, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
				Name:                 chDef.Name,
				Type:                 discordgo.ChannelTypeGuildText,
				ParentID:             catID,
				PermissionOverwrites: overwrites,
			})
			if err != nil {
				return nil, fmt.Errorf("creating channel %s: %w", chDef.Name, err)
			}
			result[chDef.Name] = ch.ID
		}
	}

	return result, nil
}

// SetupCampaignInfo holds the campaign info needed by the setup handler.
type SetupCampaignInfo struct {
	DMUserID string
}

// CampaignLookup provides campaign data for the setup handler.
type CampaignLookup interface {
	GetCampaignForSetup(guildID string) (SetupCampaignInfo, error)
	SaveChannelIDs(guildID string, channelIDs map[string]string) error
}

// SetupHandler handles the /setup slash command interaction.
type SetupHandler struct {
	bot            *Bot
	campaignLookup CampaignLookup
}

// NewSetupHandler creates a new SetupHandler.
func NewSetupHandler(bot *Bot, campaignLookup CampaignLookup) *SetupHandler {
	return &SetupHandler{bot: bot, campaignLookup: campaignLookup}
}

// Handle processes a /setup interaction. It defers the response, creates channels, and edits the response.
func (h *SetupHandler) Handle(interaction *discordgo.Interaction) {
	s := h.bot.session
	guildID := interaction.GuildID

	// Acknowledge with deferred response
	_ = s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Look up campaign for this guild
	info, err := h.campaignLookup.GetCampaignForSetup(guildID)
	if err != nil {
		msg := fmt.Sprintf("Error: no campaign found for this server. Create a campaign first.")
		_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	// Get bot user ID from state
	botUserID := ""
	if state := s.GetState(); state != nil && state.User != nil {
		botUserID = state.User.ID
	}

	// Create channels
	channelIDs, err := SetupChannels(s, guildID, botUserID, info.DMUserID)
	if err != nil {
		msg := fmt.Sprintf("Failed to create channels: %s", err)
		_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	// Save channel IDs to campaign settings
	if err := h.campaignLookup.SaveChannelIDs(guildID, channelIDs); err != nil {
		h.bot.logger.Error("failed to save channel IDs", "guild_id", guildID, "error", err)
		msg := fmt.Sprintf("Channels created successfully, but failed to save channel references: %s", err)
		_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	msg := fmt.Sprintf("Channel structure created successfully! %d channels set up.", len(channelIDs))
	_, _ = s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &msg})
}

// ensureCategory returns the ID of an existing category or creates a new one.
func ensureCategory(s Session, guildID, name string, existing map[string]*discordgo.Channel) (string, error) {
	if cat, ok := existing[name]; ok {
		return cat.ID, nil
	}

	cat, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		return "", fmt.Errorf("creating category %s: %w", name, err)
	}

	// Cache it so subsequent calls see it
	existing[name] = cat
	return cat.ID, nil
}
