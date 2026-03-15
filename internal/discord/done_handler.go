package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// DoneTurnAdvancer advances the turn in combat.
type DoneTurnAdvancer interface {
	AdvanceTurn(ctx context.Context, encounterID uuid.UUID) (combat.TurnInfo, error)
}

// DoneCampaignProvider provides campaign lookup for DM ownership check.
type DoneCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// DonePlayerLookup resolves combatant -> discord user ID.
type DonePlayerLookup interface {
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
}

// DefaultCampaignSettingsProvider looks up channel IDs via campaign settings from the encounter.
type DefaultCampaignSettingsProvider struct {
	getCampaign func(ctx context.Context, encounterID uuid.UUID) (refdata.Campaign, error)
}

// GetChannelIDs returns the channel_ids map from the campaign associated with the encounter.
func (p *DefaultCampaignSettingsProvider) GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error) {
	camp, err := p.getCampaign(ctx, encounterID)
	if err != nil {
		return nil, err
	}
	if !camp.Settings.Valid {
		return nil, nil
	}
	var settings campaign.Settings
	if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
		return nil, err
	}
	return settings.ChannelIDs, nil
}

// DefaultTurnNotifier sends messages via ChannelMessageSend.
type DefaultTurnNotifier struct{}

// NotifyTurnStart sends the turn-start notification to the given channel.
func (d *DefaultTurnNotifier) NotifyTurnStart(session Session, channelID string, content string) {
	_, _ = session.ChannelMessageSend(channelID, content)
}

// NotifyAutoSkip sends an auto-skip notification to the given channel.
func (d *DefaultTurnNotifier) NotifyAutoSkip(session Session, channelID string, content string) {
	_, _ = session.ChannelMessageSend(channelID, content)
}

// TurnNotifier sends notifications to Discord channels on turn events.
type TurnNotifier interface {
	NotifyTurnStart(session Session, channelID string, content string)
	NotifyAutoSkip(session Session, channelID string, content string)
}

// CampaignSettingsProvider looks up channel IDs from campaign settings.
type CampaignSettingsProvider interface {
	GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error)
}

// ImpactSummaryProvider gets the impact summary for a combatant.
type ImpactSummaryProvider interface {
	GetImpactSummary(ctx context.Context, encounterID, combatantID uuid.UUID) string
}

// MapRegenerator generates a combat map image for an encounter.
type MapRegenerator interface {
	RegenerateMap(ctx context.Context, encounterID uuid.UUID) ([]byte, error)
}

// DoneHandler handles the /done slash command.
type DoneHandler struct {
	session                  Session
	combatService            MoveService
	turnProvider             MoveTurnProvider
	encounterProvider        MoveEncounterProvider
	turnAdvancer             DoneTurnAdvancer
	campaignProvider         DoneCampaignProvider
	playerLookup             DonePlayerLookup
	turnNotifier             TurnNotifier
	campaignSettingsProvider CampaignSettingsProvider
	impactSummaryProvider    ImpactSummaryProvider
	mapRegenerator           MapRegenerator
}

// NewDoneHandler creates a new DoneHandler.
func NewDoneHandler(
	session Session,
	combatService MoveService,
	turnProvider MoveTurnProvider,
	encounterProvider MoveEncounterProvider,
) *DoneHandler {
	return &DoneHandler{
		session:           session,
		combatService:     combatService,
		turnProvider:      turnProvider,
		encounterProvider: encounterProvider,
	}
}

// SetTurnAdvancer sets the turn advancement dependency.
func (h *DoneHandler) SetTurnAdvancer(ta DoneTurnAdvancer) {
	h.turnAdvancer = ta
}

// SetCampaignProvider sets the campaign provider for DM ownership check.
func (h *DoneHandler) SetCampaignProvider(cp DoneCampaignProvider) {
	h.campaignProvider = cp
}

// SetPlayerLookup sets the player lookup for ownership check.
func (h *DoneHandler) SetPlayerLookup(pl DonePlayerLookup) {
	h.playerLookup = pl
}

// SetTurnNotifier sets the turn notification dependency.
func (h *DoneHandler) SetTurnNotifier(tn TurnNotifier) {
	h.turnNotifier = tn
}

// SetCampaignSettingsProvider sets the campaign settings provider.
func (h *DoneHandler) SetCampaignSettingsProvider(csp CampaignSettingsProvider) {
	h.campaignSettingsProvider = csp
}

// SetImpactSummaryProvider sets the impact summary provider.
func (h *DoneHandler) SetImpactSummaryProvider(isp ImpactSummaryProvider) {
	h.impactSummaryProvider = isp
}

// SetMapRegenerator sets the map regenerator for posting combat maps on turn end.
func (h *DoneHandler) SetMapRegenerator(mr MapRegenerator) {
	h.mapRegenerator = mr
}

// Handle processes the /done command interaction.
func (h *DoneHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	guildID := interaction.GuildID

	// Get active encounter
	encounterID, err := h.encounterProvider.GetActiveEncounterID(ctx, guildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No active encounter in this server.")
		return
	}

	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get encounter data.")
		return
	}

	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	// Get turn and combatant
	turn, err := h.turnProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get turn data.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	// Ownership validation
	userID := ""
	if interaction.Member != nil && interaction.Member.User != nil {
		userID = interaction.Member.User.ID
	}

	if !h.isAuthorized(ctx, userID, guildID, encounter, combatant) {
		respondEphemeral(h.session, interaction, "Only the current turn's player or the DM can end this turn.")
		return
	}

	// Check if sharing a tile with another creature
	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	if msg := combat.ValidateEndTurnPosition(combatant, allCombatants); msg != "" {
		respondEphemeral(h.session, interaction, msg)
		return
	}

	// Check unused resources
	unused := combat.CheckUnusedResources(turn)
	if len(unused) > 0 {
		warning := combat.FormatUnusedResourcesWarning(unused)
		h.sendConfirmation(interaction, warning, encounterID)
		return
	}

	// All resources spent — end turn immediately
	h.endTurn(ctx, interaction, encounterID, encounter)
}

// HandleDoneConfirm processes the "End Turn" button click.
func (h *DoneHandler) HandleDoneConfirm(interaction *discordgo.Interaction, encounterID uuid.UUID) {
	ctx := context.Background()

	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get encounter data.")
		return
	}

	h.endTurnFromComponent(ctx, interaction, encounterID, encounter)
}

// HandleDoneCancel processes the "Cancel" button click.
func (h *DoneHandler) HandleDoneCancel(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Turn end cancelled.",
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// isAuthorized checks if the user can end this turn (DM or current combatant's player).
func (h *DoneHandler) isAuthorized(ctx context.Context, userID, guildID string, encounter refdata.Encounter, combatant refdata.Combatant) bool {
	if userID == "" {
		return false
	}

	// Check if DM
	if h.campaignProvider != nil {
		campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, guildID)
		if err == nil && campaign.DmUserID == userID {
			return true
		}
	}

	// Check if NPC turn — only DM can end NPC turns
	if combatant.IsNpc {
		return false
	}

	// Check if this user owns the combatant's character
	if h.playerLookup != nil && combatant.CharacterID.Valid {
		pc, err := h.playerLookup.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
			CampaignID:  encounter.CampaignID,
			CharacterID: combatant.CharacterID.UUID,
		})
		if err == nil && pc.DiscordUserID == userID {
			return true
		}
	}

	// Fallback: if no providers are set, allow anyone (backward compat with existing tests)
	if h.campaignProvider == nil && h.playerLookup == nil {
		return true
	}

	return false
}

// sendConfirmation sends the ephemeral confirmation prompt with buttons.
func (h *DoneHandler) sendConfirmation(interaction *discordgo.Interaction, warning string, encounterID uuid.UUID) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: warning,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "End Turn",
							Style:    discordgo.SuccessButton,
							CustomID: fmt.Sprintf("done_confirm:%s", encounterID.String()),
							Emoji: &discordgo.ComponentEmoji{
								Name: "\u2705",
							},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.SecondaryButton,
							CustomID: "done_cancel",
							Emoji: &discordgo.ComponentEmoji{
								Name: "\u274c",
							},
						},
					},
				},
			},
		},
	})
}

// endTurn advances the turn and responds.
func (h *DoneHandler) endTurn(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter) {
	if h.turnAdvancer == nil {
		// Backward compat: no advancer set, send stub message
		respondEphemeral(h.session, interaction, "Turn ended. Use /done is not yet fully implemented.")
		return
	}

	nextTurnInfo, err := h.turnAdvancer.AdvanceTurn(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to advance turn: %v", err))
		return
	}

	// Get next combatant info
	nextCombatant, err := h.combatService.GetCombatant(ctx, nextTurnInfo.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn ended, but failed to get next combatant info.")
		return
	}

	h.sendTurnNotifications(ctx, encounterID, nextTurnInfo, nextCombatant)

	msg := fmt.Sprintf("\u2705 Turn ended. Next up: **%s** (Round %d)", nextCombatant.DisplayName, nextTurnInfo.RoundNumber)
	respondEphemeral(h.session, interaction, msg)
}

// endTurnFromComponent is like endTurn but responds as a component update.
func (h *DoneHandler) endTurnFromComponent(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter) {
	if h.turnAdvancer == nil {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Turn ended.",
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	nextTurnInfo, err := h.turnAdvancer.AdvanceTurn(ctx, encounterID)
	if err != nil {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("Failed to advance turn: %v", err),
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	nextCombatant, err := h.combatService.GetCombatant(ctx, nextTurnInfo.CombatantID)
	if err != nil {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Turn ended, but failed to get next combatant info.",
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	h.sendTurnNotifications(ctx, encounterID, nextTurnInfo, nextCombatant)

	var skippedParts []string
	// Collect skip messages from the TurnInfo
	if nextTurnInfo.Skipped {
		skippedParts = append(skippedParts, fmt.Sprintf("\u23ed\ufe0f  %s's turn was skipped", nextCombatant.DisplayName))
	}

	msg := fmt.Sprintf("\u2705 Turn ended. Next up: **%s** (Round %d)", nextCombatant.DisplayName, nextTurnInfo.RoundNumber)
	if len(skippedParts) > 0 {
		msg = strings.Join(skippedParts, "\n") + "\n" + msg
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    msg,
			Components: []discordgo.MessageComponent{},
		},
	})
}

// sendTurnNotifications posts auto-skip messages to #combat-log and
// the turn-start notification to #your-turn. Best-effort: failures are silently ignored.
func (h *DoneHandler) sendTurnNotifications(ctx context.Context, encounterID uuid.UUID, turnInfo combat.TurnInfo, nextCombatant refdata.Combatant) {
	if h.turnNotifier == nil || h.campaignSettingsProvider == nil {
		return
	}

	channelIDs, err := h.campaignSettingsProvider.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}

	// Post auto-skip messages to #combat-log
	if combatLogCh, ok := channelIDs["combat-log"]; ok && combatLogCh != "" {
		for _, skipped := range turnInfo.SkippedCombatants {
			msg := combat.FormatAutoSkipMessage(skipped.DisplayName, skipped.ConditionName)
			h.turnNotifier.NotifyAutoSkip(h.session, combatLogCh, msg)
		}
	}

	// Post turn-start notification to #your-turn
	yourTurnCh, ok := channelIDs["your-turn"]
	if !ok || yourTurnCh == "" {
		return
	}

	// Get encounter for name/round info
	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		return
	}

	encounterName := encounter.Name
	if encounter.DisplayName.Valid && encounter.DisplayName.String != "" {
		encounterName = encounter.DisplayName.String
	}

	var impactSummary string
	if h.impactSummaryProvider != nil {
		impactSummary = h.impactSummaryProvider.GetImpactSummary(ctx, encounterID, nextCombatant.ID)
	}

	content := combat.FormatTurnStartPromptWithImpact(
		encounterName,
		turnInfo.RoundNumber,
		nextCombatant.DisplayName,
		turnInfo.Turn,
		&nextCombatant,
		impactSummary,
	)
	h.turnNotifier.NotifyTurnStart(h.session, yourTurnCh, content)

	// Regenerate and post combat map
	h.postCombatMap(ctx, encounterID, channelIDs)
}

// postCombatMap regenerates the combat map and posts it to #combat-map.
// Best-effort: failures are silently ignored.
func (h *DoneHandler) postCombatMap(ctx context.Context, encounterID uuid.UUID, channelIDs map[string]string) {
	if h.mapRegenerator == nil {
		return
	}

	combatMapCh, ok := channelIDs["combat-map"]
	if !ok || combatMapCh == "" {
		return
	}

	pngData, err := h.mapRegenerator.RegenerateMap(ctx, encounterID)
	if err != nil {
		return
	}

	_, _ = h.session.ChannelMessageSendComplex(combatMapCh, &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:   "combat-map.png",
			Reader: bytes.NewReader(pngData),
		}},
	})
}
