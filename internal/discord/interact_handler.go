package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// InteractTurnStore persists a single turn's resource flags after the
// "free object interaction" toggle is consumed by /interact.
type InteractTurnStore interface {
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

// InteractEncounterProvider resolves the active encounter for the invoker.
type InteractEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

// InteractHandler handles /interact <description>. Free Object Interaction
// (Phase 74) is freeform: the player declares one item-handling action per
// turn at no resource cost beyond the per-turn FreeInteractUsed flag. No
// dedicated combat service exists; this handler simply marks the flag and
// posts the description to #combat-log.
type InteractHandler struct {
	session           Session
	encounterProvider InteractEncounterProvider
	turnStore         InteractTurnStore
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
}

// NewInteractHandler constructs a /interact handler.
func NewInteractHandler(
	session Session,
	encounterProvider InteractEncounterProvider,
	turnStore InteractTurnStore,
) *InteractHandler {
	return &InteractHandler{
		session:           session,
		encounterProvider: encounterProvider,
		turnStore:         turnStore,
	}
}

// SetChannelIDProvider wires the campaign settings provider for combat-log
// channel resolution. When nil, /interact responds ephemerally only.
func (h *InteractHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetTurnGate wires the Phase 27 turn-ownership gate. /interact is NOT in
// combat.IsExemptCommand's exempt list (it's an on-turn declaration), so a
// non-owner invoker is rejected up front when the gate is wired.
func (h *InteractHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// Handle processes the /interact command interaction.
func (h *InteractHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	desc := optionString(interaction, "description")
	if desc == "" {
		respondEphemeral(h.session, interaction, "Please describe what you do (e.g. `/interact draw longsword`).")
		return
	}

	userID := discordUserID(interaction)
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "You are not in an active encounter.")
		return
	}

	encounter, err := h.encounterProvider.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load encounter.")
		return
	}

	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	if !combat.IsExemptCommand("interact") && h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRelease(ctx, encounterID, userID); gateErr != nil {
			respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			return
		}
	}

	turn, err := h.turnStore.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load turn.")
		return
	}

	updatedTurn, err := combat.UseResource(turn, combat.ResourceFreeInteract)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot interact: %v", err))
		return
	}

	if _, err := h.turnStore.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn)); err != nil {
		respondEphemeral(h.session, interaction, "Failed to record interaction.")
		return
	}

	combatant, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	name := "You"
	if err == nil {
		name = combatant.DisplayName
	}

	logLine := fmt.Sprintf("\U0001f9d0  %s interacts with the environment: \"%s\"", name, desc)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// postCombatLog mirrors a combat log line to #combat-log when wired.
func (h *InteractHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}
