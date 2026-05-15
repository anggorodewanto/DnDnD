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
				{Name: "dm-queue", PermissionFunc: dmQueuePerms},
			},
		},
	}
}

// exclusiveWritePerms returns overwrites that deny @everyone SendMessages
// and allow only the specified user to send messages.
func exclusiveWritePerms(guildID, allowedUserID string) []*discordgo.PermissionOverwrite {
	return []*discordgo.PermissionOverwrite{
		{
			ID:   guildID, // @everyone role ID == guild ID
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionSendMessages,
		},
		{
			ID:    allowedUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages,
		},
	}
}

// theStoryPerms returns overwrites making #the-story DM-and-bot-write-only.
func theStoryPerms(guildID, botUserID, dmUserID string) []*discordgo.PermissionOverwrite {
	return []*discordgo.PermissionOverwrite{
		{
			ID:   guildID,
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionSendMessages,
		},
		{
			ID:    dmUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages,
		},
		{
			ID:    botUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages,
		},
	}
}

// combatMapPerms returns overwrites making #combat-map bot-write-only.
func combatMapPerms(guildID, botUserID, _ string) []*discordgo.PermissionOverwrite {
	return exclusiveWritePerms(guildID, botUserID)
}

// dmQueuePerms returns overwrites making #dm-queue DM-only: @everyone is
// denied ViewChannel, and only the DM (and the bot for posting) can see it.
func dmQueuePerms(guildID, botUserID, dmUserID string) []*discordgo.PermissionOverwrite {
	return []*discordgo.PermissionOverwrite{
		{
			ID:   guildID, // @everyone role ID == guild ID
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory,
		},
		{
			ID:    dmUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory | discordgo.PermissionSendMessages,
		},
		{
			ID:    botUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory | discordgo.PermissionSendMessages,
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
	// AutoCreated is true when the lookup had to create the campaign row
	// because none existed for this guild yet (med-41 / Phase 11 wiring).
	// The setup handler uses this purely to vary the success message.
	AutoCreated bool
}

// CampaignLookup provides campaign data for the setup handler.
//
// Implementations MUST auto-create the campaign row when no row exists for
// the guild yet (med-41 / Phase 11): the invoker of /setup is taken to be
// the DM and a row with default settings is inserted. This closes the
// "no campaign found for this server" dead-end the playtest quickstart used
// to hit before any encounter could be built.
type CampaignLookup interface {
	GetCampaignForSetup(guildID, invokerUserID string) (SetupCampaignInfo, error)
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
	invokerUserID := setupInvokerUserID(interaction)

	_ = s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	info, err := h.campaignLookup.GetCampaignForSetup(guildID, invokerUserID)
	if err != nil {
		h.editResponse(interaction, fmt.Sprintf("Error resolving campaign: %s", err))
		return
	}

	channelIDs, err := SetupChannels(s, guildID, botUserIDFromState(s), info.DMUserID)
	if err != nil {
		h.editResponse(interaction, fmt.Sprintf("Failed to create channels: %s", err))
		return
	}

	if err := h.campaignLookup.SaveChannelIDs(guildID, channelIDs); err != nil {
		h.bot.logger.Error("failed to save channel IDs", "guild_id", guildID, "error", err)
		h.editResponse(interaction, fmt.Sprintf("Channels created successfully, but failed to save channel references: %s", err))
		return
	}

	prefix := "Channel structure created successfully!"
	if info.AutoCreated {
		prefix = "Campaign created and channel structure set up!"
	}
	h.editResponse(interaction, fmt.Sprintf("%s %d channels set up.", prefix, len(channelIDs)))
}

// setupInvokerUserID extracts the Discord user ID of the user who invoked
// /setup. Returns "" when neither Member.User nor User is populated (only
// happens in unit tests with a hand-rolled Interaction).
func setupInvokerUserID(interaction *discordgo.Interaction) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	if interaction.User != nil {
		return interaction.User.ID
	}
	return ""
}

// editResponse is a convenience wrapper for editing a deferred interaction response.
func (h *SetupHandler) editResponse(interaction *discordgo.Interaction, msg string) {
	_, _ = h.bot.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &msg})
}

// botUserIDFromState extracts the bot's user ID from the session state, returning "" if unavailable.
func botUserIDFromState(s Session) string {
	if state := s.GetState(); state != nil && state.User != nil {
		return state.User.ID
	}
	return ""
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
