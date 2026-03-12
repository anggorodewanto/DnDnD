package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/combat"
)

// DoneHandler handles the /done slash command.
type DoneHandler struct {
	session           Session
	combatService     MoveService
	turnProvider      MoveTurnProvider
	encounterProvider MoveEncounterProvider
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

	// Position is valid — proceed with ending the turn (stub for now)
	respondEphemeral(h.session, interaction, "Turn ended. Use /done is not yet fully implemented.")
}
