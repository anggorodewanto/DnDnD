package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// UndoEncounterResolver resolves the active encounter for a Discord user.
type UndoEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// UndoCombatantLookup resolves a Discord user to their combatant in an
// encounter (returns the combatantID + display name used in the dm-queue post).
type UndoCombatantLookup interface {
	GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error)
}

// UndoActionLog returns the encounter's action log, ordered chronologically.
// The handler picks the most recent row for the player's combatant to
// summarise what they want undone.
type UndoActionLog interface {
	ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error)
}

// UndoHandler handles the /undo slash command. It does NOT roll back state —
// it routes a request to the DM via #dm-queue. The DM uses the dashboard's
// Phase 97b undo to actually rollback.
type UndoHandler struct {
	session         Session
	resolver        UndoEncounterResolver
	combatantLookup UndoCombatantLookup
	actionLog       UndoActionLog
	notifier        dmqueue.Notifier
	campaignProv    CheckCampaignProvider
}

// SetCampaignProvider wires the campaign-by-guild lookup so /undo dm-queue
// posts carry the campaign UUID required by PgStore.Insert (SR-002). Nil-safe.
func (h *UndoHandler) SetCampaignProvider(p CheckCampaignProvider) {
	h.campaignProv = p
}

// NewUndoHandler constructs an UndoHandler.
func NewUndoHandler(
	session Session,
	resolver UndoEncounterResolver,
	combatantLookup UndoCombatantLookup,
	actionLog UndoActionLog,
	notifier dmqueue.Notifier,
) *UndoHandler {
	return &UndoHandler{
		session:         session,
		resolver:        resolver,
		combatantLookup: combatantLookup,
		actionLog:       actionLog,
		notifier:        notifier,
	}
}

// Handle processes a /undo interaction. It posts a dmqueue.KindUndoRequest
// event with a one-line summary of the player's most recent action, then
// replies ephemerally to confirm the request was sent.
func (h *UndoHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	userID := discordUserID(interaction)
	reason := optionString(interaction, "reason")
	if reason == "" {
		reason = "(no reason given)"
	}

	playerName, lastAction := h.lookupPlayerContext(ctx, interaction.GuildID, userID)
	summary := fmt.Sprintf("requests undo: %s — last action: %s", reason, lastAction)

	if h.notifier != nil {
		_, _ = h.notifier.Post(ctx, dmqueue.Event{
			Kind:       dmqueue.KindUndoRequest,
			PlayerName: playerName,
			Summary:    summary,
			GuildID:    interaction.GuildID,
			CampaignID: h.resolveCampaignID(ctx, interaction.GuildID),
		})
	}

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("⏪ Undo request sent to the DM. They'll resolve it from the dashboard.\n_Reason: %s_", reason))
}

// resolveCampaignID returns the campaign UUID string for the guild, or ""
// when the provider is unwired or the lookup fails. Used to thread
// CampaignID through dm-queue posts (SR-002).
func (h *UndoHandler) resolveCampaignID(ctx context.Context, guildID string) string {
	if h.campaignProv == nil {
		return ""
	}
	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return ""
	}
	return campaign.ID.String()
}

// lookupPlayerContext returns the player's display name (defaulting to "Player"
// when the lookup fails) and a one-line summary of their most recent action
// (defaulting to "(no recent action)" when no encounter / no rows are found).
func (h *UndoHandler) lookupPlayerContext(ctx context.Context, guildID, userID string) (string, string) {
	playerName := "Player"
	lastAction := "(no recent action)"
	if h.resolver == nil {
		return playerName, lastAction
	}
	encID, err := h.resolver.ActiveEncounterForUser(ctx, guildID, userID)
	if err != nil || encID == uuid.Nil {
		return playerName, lastAction
	}
	if h.combatantLookup != nil {
		if combID, displayName, err := h.combatantLookup.GetCombatantIDByDiscordUser(ctx, encID, userID); err == nil {
			if displayName != "" {
				playerName = displayName
			}
			if h.actionLog != nil {
				if logs, err := h.actionLog.ListActionLogByEncounterID(ctx, encID); err == nil {
					if summary := mostRecentActionFor(logs, combID); summary != "" {
						lastAction = summary
					}
				}
			}
		}
	}
	return playerName, lastAction
}

// mostRecentActionFor returns a one-line summary of the most recent action_log
// row whose actor_id matches actorID, or "" if none. Logs are scanned right-
// to-left because ListActionLogByEncounterID returns rows in ascending order.
func mostRecentActionFor(logs []refdata.ActionLog, actorID uuid.UUID) string {
	for i := len(logs) - 1; i >= 0; i-- {
		if logs[i].ActorID != actorID {
			continue
		}
		if logs[i].Description.Valid && logs[i].Description.String != "" {
			return logs[i].Description.String
		}
		if logs[i].ActionType != "" {
			return fmt.Sprintf("(%s)", logs[i].ActionType)
		}
		return "(unknown action)"
	}
	return ""
}
