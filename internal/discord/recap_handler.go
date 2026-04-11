package discord

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// RecapService defines the service methods needed by the /recap handler.
type RecapService interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	ListActionLogWithRounds(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error)
	GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error)
	GetLastCompletedTurnByCombatant(ctx context.Context, encounterID, combatantID uuid.UUID) (refdata.Turn, error)
}

// RecapEncounterProvider resolves the active encounter the invoking user is a
// combatant in. Phase 105 ensures /recap in a guild with simultaneous
// encounters picks the caller's encounter; if the caller is not in any
// active encounter, the handler falls back to the most recent completed
// encounter for the campaign.
type RecapEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// RecapPlayerLookup resolves a Discord user to their combatant ID.
type RecapPlayerLookup interface {
	GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, error)
}

// RecapCampaignProvider provides the campaign for a guild.
type RecapCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// RecapHandler handles the /recap slash command.
type RecapHandler struct {
	session           Session
	svc               RecapService
	encounterProvider RecapEncounterProvider
	playerLookup      RecapPlayerLookup
	campaignProvider  RecapCampaignProvider
}

// NewRecapHandler creates a new RecapHandler.
func NewRecapHandler(session Session, svc RecapService, ep RecapEncounterProvider, pl RecapPlayerLookup) *RecapHandler {
	return &RecapHandler{
		session:           session,
		svc:               svc,
		encounterProvider: ep,
		playerLookup:      pl,
	}
}

// SetCampaignProvider sets the campaign provider for post-combat recap fallback.
func (h *RecapHandler) SetCampaignProvider(cp RecapCampaignProvider) {
	h.campaignProvider = cp
}

// Handle processes the /recap command interaction.
func (h *RecapHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	var roundsArg int64
	hasRoundsArg := false
	for _, opt := range data.Options {
		if opt.Name == "rounds" {
			roundsArg = opt.IntValue()
			hasRoundsArg = true
		}
	}

	// Find encounter: active (routed via invoking player's combatant) first,
	// then fall back to most recent completed encounter for the campaign.
	encounterID, encounter, err := h.findEncounter(ctx, interaction.GuildID, discordUserID(interaction))
	if err != nil {
		respondEphemeral(h.session, interaction, "No encounter found for recap.")
		return
	}

	// Fetch all action logs with round info
	logs, err := h.svc.ListActionLogWithRounds(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to retrieve combat logs.")
		return
	}

	var filtered []refdata.ListActionLogWithRoundsRow
	var subtitle string

	if hasRoundsArg {
		// /recap N — last N rounds
		filtered = combat.FilterLogsLastNRounds(logs, int(roundsArg))
		subtitle = combat.RecapRoundRange(filtered)
	} else if encounter.Status == "active" {
		// /recap with no args during active combat — since player's last turn
		filtered, subtitle = h.filterSinceLastTurn(ctx, encounterID, interaction, logs)
	} else {
		// Post-combat: show all rounds
		filtered = logs
		subtitle = combat.RecapRoundRange(filtered)
	}

	msg := combat.FormatRecap(filtered, subtitle)
	msg = combat.TruncateRecap(msg, 2000)
	respondEphemeral(h.session, interaction, msg)
}

// findEncounter tries to find an active encounter for the invoking user, then
// falls back to the most recent completed encounter in the campaign.
func (h *RecapHandler) findEncounter(ctx context.Context, guildID, discordUserID string) (uuid.UUID, refdata.Encounter, error) {
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, guildID, discordUserID)
	if err == nil {
		enc, encErr := h.svc.GetEncounter(ctx, encounterID)
		if encErr == nil {
			return encounterID, enc, nil
		}
	}

	if h.campaignProvider == nil {
		return uuid.Nil, refdata.Encounter{}, errors.New("no encounter available")
	}

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return uuid.Nil, refdata.Encounter{}, err
	}

	enc, err := h.svc.GetMostRecentCompletedEncounter(ctx, campaign.ID)
	if err != nil {
		return uuid.Nil, refdata.Encounter{}, err
	}

	return enc.ID, enc, nil
}

// filterSinceLastTurn returns logs since the player's last completed turn.
func (h *RecapHandler) filterSinceLastTurn(
	ctx context.Context,
	encounterID uuid.UUID,
	interaction *discordgo.Interaction,
	logs []refdata.ListActionLogWithRoundsRow,
) ([]refdata.ListActionLogWithRoundsRow, string) {
	allLogs := func() ([]refdata.ListActionLogWithRoundsRow, string) {
		return logs, combat.RecapRoundRange(logs)
	}

	if h.playerLookup == nil {
		return allLogs()
	}

	userID := discordUserID(interaction)
	combatantID, err := h.playerLookup.GetCombatantIDByDiscordUser(ctx, encounterID, userID)
	if err != nil {
		return allLogs()
	}

	lastTurn, err := h.svc.GetLastCompletedTurnByCombatant(ctx, encounterID, combatantID)
	if err != nil {
		return allLogs()
	}

	// No completed turn timestamp means include all logs
	if !lastTurn.CompletedAt.Valid {
		return logs, "since your last turn"
	}

	var filtered []refdata.ListActionLogWithRoundsRow
	for _, l := range logs {
		if l.CreatedAt.After(lastTurn.CompletedAt.Time) {
			filtered = append(filtered, l)
		}
	}

	rangeStr := combat.RecapRoundRange(filtered)
	if rangeStr != "" {
		rangeStr += " (since your last turn)"
	} else {
		rangeStr = "since your last turn"
	}

	return filtered, rangeStr
}
