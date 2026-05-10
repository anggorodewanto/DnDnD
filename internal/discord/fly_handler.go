package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

// FlyHandler handles the /fly slash command.
type FlyHandler struct {
	session           Session
	combatService     MoveService
	turnProvider      MoveTurnProvider
	encounterProvider MoveEncounterProvider
	turnGate          TurnGate
}

// SetTurnGate wires the Phase 27 turn-ownership / advisory-lock gate.
// A nil gate disables the check; production wiring always supplies one.
func (h *FlyHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// NewFlyHandler creates a new FlyHandler.
func NewFlyHandler(
	session Session,
	combatService MoveService,
	turnProvider MoveTurnProvider,
	encounterProvider MoveEncounterProvider,
) *FlyHandler {
	return &FlyHandler{
		session:           session,
		combatService:     combatService,
		turnProvider:      turnProvider,
		encounterProvider: encounterProvider,
	}
}

// Handle processes the /fly command interaction.
func (h *FlyHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if len(data.Options) == 0 {
		respondEphemeral(h.session, interaction, "Please provide an altitude (e.g. `/fly 30`).")
		return
	}

	targetAltitude := int32(data.Options[0].IntValue())

	guildID := interaction.GuildID

	// Phase 105: route to the invoker's own combat encounter.
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

	// Phase 27 turn-ownership + advisory-lock gate. /fly mutates altitude
	// and burns movement; a non-owner must be rejected and concurrent
	// /fly invocations on the same turn must serialize.
	if h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRelease(ctx, encounterID, discordUserID(interaction)); gateErr != nil {
			respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			return
		}
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

	// Validate fly
	flyReq := combat.FlyRequest{
		TargetAltitude:      targetAltitude,
		CurrentAltitude:     combatant.AltitudeFt,
		MovementRemainingFt: turn.MovementRemainingFt,
	}

	result := combat.ValidateFly(flyReq)
	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	// Build confirmation with buttons
	confirmMsg := combat.FormatFlyConfirmation(result)

	confirmID := fmt.Sprintf("fly_confirm:%s:%s:%d:%d",
		turn.ID.String(), combatant.ID.String(), targetAltitude, result.CostFt)
	cancelID := fmt.Sprintf("fly_cancel:%s", turn.ID.String())

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: confirmMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm",
							Style:    discordgo.SuccessButton,
							CustomID: confirmID,
							Emoji: &discordgo.ComponentEmoji{
								Name: "\u2705",
							},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: cancelID,
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

// HandleFlyConfirm processes the fly confirmation button click.
func (h *FlyHandler) HandleFlyConfirm(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, newAltitude int32, costFt int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	// Deduct movement
	updatedTurn, err := combat.UseMovement(turn, int32(costFt))
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot fly: %v", err))
		return
	}

	// Persist turn resources
	_, err = h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn))
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update turn resources.")
		return
	}

	// Get current combatant for position
	combatant, err := h.combatService.GetCombatant(ctx, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	// Update combatant altitude
	_, err = h.combatService.UpdateCombatantPosition(ctx, combatantID, combatant.PositionCol, combatant.PositionRow, newAltitude)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	remaining := combat.FormatRemainingResources(updatedTurn, nil)
	var msg string
	if newAltitude == 0 {
		msg = fmt.Sprintf("\U0001f985 Descended to ground level. %s", remaining)
	} else {
		msg = fmt.Sprintf("\U0001f985 Flying at %dft altitude. %s", newAltitude, remaining)
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    msg,
			Components: []discordgo.MessageComponent{},
		},
	})
}

// HandleFlyCancel processes the fly cancel button click.
func (h *FlyHandler) HandleFlyCancel(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Fly cancelled.",
			Components: []discordgo.MessageComponent{},
		},
	})
}

// ParseFlyConfirmData parses the custom ID of a fly confirm button.
func ParseFlyConfirmData(customID string) (turnID, combatantID uuid.UUID, newAltitude int32, costFt int, err error) {
	var turnStr, combatantStr string
	var alt, cost int
	n, scanErr := fmt.Sscanf(customID, "fly_confirm:%36s:%36s:%d:%d",
		&turnStr, &combatantStr, &alt, &cost)
	if scanErr != nil || n != 4 {
		return uuid.Nil, uuid.Nil, 0, 0, fmt.Errorf("invalid fly confirm data: %q", customID)
	}
	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, int32(alt), cost, nil
}
