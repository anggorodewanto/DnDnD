package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

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

	// Phase 105: route the /done to the encounter the invoking player
	// currently belongs to (via their combatant entry). The DM typically
	// uses the dashboard instead; slash /done by a DM is best-effort only.
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, guildID, discordUserID(interaction))
	if err != nil {
		respondEphemeral(h.session, interaction, "No active encounter for you in this server.")
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
	h.endTurn(ctx, interaction, encounterID)
}

// HandleDoneConfirm processes the "End Turn" button click.
func (h *DoneHandler) HandleDoneConfirm(interaction *discordgo.Interaction, encounterID uuid.UUID) {
	ctx := context.Background()

	if _, err := h.combatService.GetEncounter(ctx, encounterID); err != nil {
		respondEphemeral(h.session, interaction, "Failed to get encounter data.")
		return
	}

	h.endTurnFromComponent(ctx, interaction, encounterID)
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

// endTurn advances the turn and responds via ephemeral slash command response.
func (h *DoneHandler) endTurn(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID) {
	respond := func(msg string) {
		respondEphemeral(h.session, interaction, msg)
	}
	h.advanceAndRespond(ctx, encounterID, respond, false)
}

// endTurnFromComponent advances the turn and responds via component update.
func (h *DoneHandler) endTurnFromComponent(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID) {
	respond := func(msg string) {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    msg,
				Components: []discordgo.MessageComponent{},
			},
		})
	}
	h.advanceAndRespond(ctx, encounterID, respond, true)
}

// advanceAndRespond contains the shared logic for ending a turn: advance, notify, respond.
// When includeSkipInfo is true, skipped combatant messages are prepended to the response.
func (h *DoneHandler) advanceAndRespond(ctx context.Context, encounterID uuid.UUID, respond func(string), includeSkipInfo bool) {
	if h.turnAdvancer == nil {
		respond("Turn ended. Use /done is not yet fully implemented.")
		return
	}

	nextTurnInfo, err := h.turnAdvancer.AdvanceTurn(ctx, encounterID)
	if err != nil {
		respond(fmt.Sprintf("Failed to advance turn: %v", err))
		return
	}

	nextCombatant, err := h.combatService.GetCombatant(ctx, nextTurnInfo.CombatantID)
	if err != nil {
		respond("Turn ended, but failed to get next combatant info.")
		return
	}

	h.sendTurnNotifications(ctx, encounterID, nextTurnInfo, nextCombatant)

	msg := fmt.Sprintf("\u2705 Turn ended. Next up: **%s** (Round %d)", nextCombatant.DisplayName, nextTurnInfo.RoundNumber)

	if includeSkipInfo && nextTurnInfo.Skipped {
		skipMsg := fmt.Sprintf("\u23ed\ufe0f  %s's turn was skipped", nextCombatant.DisplayName)
		msg = skipMsg + "\n" + msg
	}

	respond(msg)
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

	// Resolve encounter display name up front so shared-channel messages
	// (auto-skip, turn-start, combat-map) can all be labeled with the
	// Phase 105 prefix: "⚔️ <name> — Round N".
	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		return
	}
	encounterName := combat.EncounterDisplayName(encounter)
	label := combat.FormatEncounterLabel(encounterName, turnInfo.RoundNumber)

	// Post auto-skip messages to #combat-log
	if combatLogCh, ok := channelIDs["combat-log"]; ok && combatLogCh != "" {
		for _, skipped := range turnInfo.SkippedCombatants {
			msg := combat.FormatAutoSkipMessage(skipped.DisplayName, skipped.ConditionName)
			h.turnNotifier.NotifyAutoSkip(h.session, combatLogCh, label+"\n"+msg)
		}
	}

	// Post turn-start notification to #your-turn
	yourTurnCh, ok := channelIDs["your-turn"]
	if !ok || yourTurnCh == "" {
		return
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

	// Regenerate and post combat map with the Phase 105 encounter label so
	// simultaneous encounters sharing #combat-map are distinguishable.
	PostCombatMap(ctx, h.session, h.mapRegenerator, encounterID, channelIDs, label)
}

// PostCombatMap regenerates the combat map and posts it to #combat-map.
// Best-effort: failures are silently ignored.
//
// Phase 105: the label (e.g. "⚔️ Rooftop Ambush — Round 3") is required
// and included as the message content so players in a shared channel can
// tell which simultaneous encounter the map belongs to. Pass "" when no
// label is available (e.g. encounter lookup failed).
func PostCombatMap(ctx context.Context, session Session, mr MapRegenerator, encounterID uuid.UUID, channelIDs map[string]string, label string) {
	if mr == nil {
		return
	}

	combatMapCh, ok := channelIDs["combat-map"]
	if !ok || combatMapCh == "" {
		return
	}

	pngData, err := mr.RegenerateMap(ctx, encounterID)
	if err != nil {
		return
	}

	_, _ = session.ChannelMessageSendComplex(combatMapCh, &discordgo.MessageSend{
		Content: label,
		Files: []*discordgo.File{{
			Name:   "combat-map.png",
			Reader: bytes.NewReader(pngData),
		}},
	})
}
