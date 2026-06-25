package discord

import (
	"fmt"
	"strings"

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
	// Topic is the channel description shown under the channel name, explaining
	// what the channel is for. Set on creation via /setup.
	Topic string
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
				{Name: "initiative-tracker", Topic: "Bot-maintained turn order for the current encounter — who's up now and who's next."},
				{Name: "combat-log", Topic: "Auto-logged combat events: attack rolls, damage, saves, conditions, and deaths."},
				{Name: "roll-history", Topic: "A running record of dice rolls (checks, saves, attacks) for table transparency."},
			},
		},
		{
			Name: "NARRATION",
			Channels: []ChannelDef{
				{Name: "the-story", Topic: "DM narration and scene-setting. Only the DM and bot post here — read along and react in #in-character.", PermissionFunc: theStoryPerms},
				{Name: "in-character", Topic: "Speak and act as your character — in-world dialogue and roleplay."},
				{Name: "player-chat", Topic: "Out-of-character table talk: questions, scheduling, and banter."},
			},
		},
		{
			Name: "COMBAT",
			Channels: []ChannelDef{
				{Name: "combat-map", Topic: "The rendered battle map for the current encounter, posted and updated by the bot.", PermissionFunc: combatMapPerms},
				{Name: "your-turn", Topic: "Turn prompts and reminders — the bot pings you here when it's your turn to act."},
			},
		},
		{
			Name: "REFERENCE",
			Channels: []ChannelDef{
				{Name: "character-cards", Topic: "Live character sheets — HP, AC, spell slots, and equipment, kept current by the bot."},
				{Name: "dm-queue", Topic: "DM-only: character approvals and admin items awaiting the Dungeon Master.", PermissionFunc: dmQueuePerms},
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
// On partial failure, returns channels created so far alongside the error.
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
			return result, err
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
				Topic:                chDef.Topic,
				ParentID:             catID,
				PermissionOverwrites: overwrites,
			})
			if err != nil {
				return result, fmt.Errorf("creating channel %s: %w", chDef.Name, err)
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
//
// Find and Create are split so the handler can enforce the server-admin gate
// BEFORE any campaign row is persisted. A non-admin who runs /setup on a guild
// with no campaign must be rejected without a row being created — otherwise the
// rejected user is silently made DM of a real campaign (the auto-create
// originally happened inside the lookup, before the gate ran).
//
// When no campaign exists for the guild yet, the invoker of /setup (if a server
// admin) is taken to be the DM and a row with default settings is inserted via
// CreateCampaignForSetup. This closes the "no campaign found for this server"
// dead-end the playtest quickstart used to hit before any encounter could be
// built (med-41 / Phase 11).
type CampaignLookup interface {
	// FindCampaignForSetup returns the existing campaign's setup info.
	// exists is false (with a nil error) when no campaign row exists for the
	// guild yet; the handler then gates on server-admin before calling
	// CreateCampaignForSetup. Implementations MUST NOT create a row here.
	FindCampaignForSetup(guildID string) (info SetupCampaignInfo, exists bool, err error)
	// CreateCampaignForSetup inserts a new campaign with invokerUserID as DM and
	// default settings, returning its setup info. Called only after the
	// server-admin gate passes.
	CreateCampaignForSetup(guildID, invokerUserID string) (SetupCampaignInfo, error)
	SaveChannelIDs(guildID string, channelIDs map[string]string) error
}

// SetupHandler handles the /setup slash command interaction.
type SetupHandler struct {
	bot            *Bot
	campaignLookup CampaignLookup
	// baseURL is the operator-configured public base (BASE_URL) used to build
	// the dashboard link in the success message. Empty disables the link.
	baseURL string
}

// NewSetupHandler creates a new SetupHandler.
func NewSetupHandler(bot *Bot, campaignLookup CampaignLookup) *SetupHandler {
	return &SetupHandler{bot: bot, campaignLookup: campaignLookup}
}

// WithBaseURL sets the public base URL used to build the dashboard next-step
// link in the /setup success message and returns the handler for chaining.
func (h *SetupHandler) WithBaseURL(baseURL string) *SetupHandler {
	h.baseURL = baseURL
	return h
}

// Handle processes a /setup interaction. It defers the response, creates channels, and edits the response.
func (h *SetupHandler) Handle(interaction *discordgo.Interaction) {
	s := h.bot.session
	guildID := interaction.GuildID
	invokerUserID := setupInvokerUserID(interaction)

	_ = s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	info, exists, err := h.campaignLookup.FindCampaignForSetup(guildID)
	if err != nil {
		h.editResponse(interaction, fmt.Sprintf("Error resolving campaign: %s", err))
		return
	}

	if exists && invokerUserID != info.DMUserID {
		h.editResponse(interaction, "⛔ Only the campaign DM can run /setup for this server.")
		return
	}
	if !exists && !setupInvokerIsAdmin(interaction) {
		h.editResponse(interaction, "⛔ Only a server administrator can create a new campaign via /setup.")
		return
	}
	if !exists {
		created, createErr := h.campaignLookup.CreateCampaignForSetup(guildID, invokerUserID)
		if createErr != nil {
			h.editResponse(interaction, fmt.Sprintf("Error creating campaign: %s", createErr))
			return
		}
		info = created
	}

	channelIDs, err := SetupChannels(s, guildID, botUserIDFromState(s), info.DMUserID)
	if err != nil {
		// Persist any channels that were created before the failure so
		// a re-run of /setup can reconcile without orphaned channels.
		if len(channelIDs) > 0 {
			_ = h.campaignLookup.SaveChannelIDs(guildID, channelIDs)
		}
		h.editResponse(interaction, fmt.Sprintf("Failed to create channels: %s", err))
		return
	}

	if err := h.campaignLookup.SaveChannelIDs(guildID, channelIDs); err != nil {
		h.bot.logger.Error("failed to save channel IDs", "guild_id", guildID, "error", err)
		h.editResponse(interaction, fmt.Sprintf("Channels created successfully, but failed to save channel references: %s", err))
		return
	}

	prefix := "Channel structure created successfully!"
	if !exists {
		prefix = "Campaign created and channel structure set up!"
	}
	h.editResponse(interaction, fmt.Sprintf("%s %d channels set up.\n\n%s", prefix, len(channelIDs), h.nextStep()))
}

// nextStep returns the post-/setup guidance shown to the DM. When a base URL
// is configured it links straight to the dashboard; otherwise it names the
// step without rendering a broken link.
func (h *SetupHandler) nextStep() string {
	if h.baseURL == "" {
		return "Next: open the dashboard to build a map, then create an encounter."
	}
	dashboard := strings.TrimRight(h.baseURL, "/") + "/dashboard/"
	return fmt.Sprintf("Next: open the dashboard to build a map — %s", dashboard)
}

// setupInvokerIsAdmin returns true when the invoker has the Administrator
// permission bit set in the interaction's resolved member permissions.
func setupInvokerIsAdmin(interaction *discordgo.Interaction) bool {
	if interaction.Member == nil {
		return false
	}
	return interaction.Member.Permissions&int64(discordgo.PermissionAdministrator) != 0
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
