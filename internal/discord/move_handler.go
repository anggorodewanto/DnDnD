package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// MoveService defines the combat operations needed by the move handler.
type MoveService interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error)
}

// MoveMapProvider retrieves map data for pathfinding.
type MoveMapProvider interface {
	GetByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
}

// MoveTurnProvider retrieves and updates turn data.
type MoveTurnProvider interface {
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

// MoveEncounterProvider resolves encounters for the current guild/channel context.
type MoveEncounterProvider interface {
	GetActiveEncounterID(ctx context.Context, guildID string) (uuid.UUID, error)
}

// MoveHandler handles the /move slash command.
type MoveHandler struct {
	session           Session
	combatService     MoveService
	mapProvider       MoveMapProvider
	turnProvider      MoveTurnProvider
	encounterProvider MoveEncounterProvider
	campaignProv      CampaignProvider
}

// NewMoveHandler creates a new MoveHandler.
func NewMoveHandler(
	session Session,
	combatService MoveService,
	mapProvider MoveMapProvider,
	turnProvider MoveTurnProvider,
	encounterProvider MoveEncounterProvider,
	campaignProv CampaignProvider,
) *MoveHandler {
	return &MoveHandler{
		session:           session,
		combatService:     combatService,
		mapProvider:       mapProvider,
		turnProvider:      turnProvider,
		encounterProvider: encounterProvider,
		campaignProv:      campaignProv,
	}
}

// Handle processes the /move command interaction.
func (h *MoveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if len(data.Options) == 0 {
		respondEphemeral(h.session, interaction, "Please provide a coordinate (e.g. `/move D4`).")
		return
	}

	coordStr := data.Options[0].StringValue()
	destCol, destRow, err := renderer.ParseCoordinate(coordStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid coordinate %q: %v", coordStr, err))
		return
	}

	userID := interactionUserID(interaction)
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

	_ = userID // TODO: turn ownership validation will be wired when full turn lock is available

	// Get map data
	if !encounter.MapID.Valid {
		respondEphemeral(h.session, interaction, "This encounter has no map.")
		return
	}

	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load map data.")
		return
	}

	// Parse tiled JSON to get terrain and walls
	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to parse map data.")
		return
	}

	// Build occupants list from all combatants
	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	occupants := buildOccupants(allCombatants, combatant)

	grid := &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: occupants,
	}

	// Determine mover size category
	sizeCategory := pathfinding.SizeMedium
	if combatant.CreatureRefID.Valid {
		// For creatures we'd look up size; default to Medium for now
		sizeCategory = pathfinding.SizeMedium
	}

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: sizeCategory,
	}

	result, err := combat.ValidateMove(moveReq)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	// Build confirmation with buttons
	confirmMsg := combat.FormatMoveConfirmation(result)

	// Encode move data in custom IDs for button callback
	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d",
		turn.ID.String(), combatant.ID.String(), destCol, destRow, result.CostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turn.ID.String())

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

// HandleMoveConfirm processes the move confirmation button click.
func (h *MoveHandler) HandleMoveConfirm(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, costFt int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	// Deduct movement
	updatedTurn, err := combat.UseMovement(turn, int32(costFt))
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot move: %v", err))
		return
	}

	// Persist turn resources
	_, err = h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn))
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update turn resources.")
		return
	}

	// Update combatant position
	destLabel := renderer.ColumnLabel(destCol)
	_, err = h.combatService.UpdateCombatantPosition(ctx, combatantID, destLabel, int32(destRow+1), 0)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	remaining := combat.FormatRemainingResources(updatedTurn)
	msg := fmt.Sprintf("\U0001f3c3 Moved to %s%d. %s", destLabel, destRow+1, remaining)

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    msg,
			Components: []discordgo.MessageComponent{}, // remove buttons
		},
	})
}

// HandleMoveCancel processes the move cancel button click.
func (h *MoveHandler) HandleMoveCancel(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Move cancelled.",
			Components: []discordgo.MessageComponent{},
		},
	})
}

// buildOccupants creates a pathfinding occupant list from combatants, excluding the mover.
func buildOccupants(all []refdata.Combatant, mover refdata.Combatant) []pathfinding.Occupant {
	var occupants []pathfinding.Occupant
	for _, c := range all {
		if c.ID == mover.ID {
			continue
		}
		if !c.IsAlive {
			continue
		}
		col, row, err := renderer.ParseCoordinate(c.PositionCol + fmt.Sprintf("%d", c.PositionRow))
		if err != nil {
			continue
		}
		occupants = append(occupants, pathfinding.Occupant{
			Col:          col,
			Row:          row,
			IsAlly:       !c.IsNpc == !mover.IsNpc, // ally if same faction
			SizeCategory: pathfinding.SizeMedium,    // default; would look up creature size
		})
	}
	return occupants
}

// ParseMoveConfirmData parses the custom ID of a move confirm button.
func ParseMoveConfirmData(customID string) (turnID, combatantID uuid.UUID, destCol, destRow, costFt int, err error) {
	var turnStr, combatantStr string
	n, scanErr := fmt.Sscanf(customID, "move_confirm:%36s:%36s:%d:%d:%d",
		&turnStr, &combatantStr, &destCol, &destRow, &costFt)
	if scanErr != nil || n != 5 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid move confirm data: %q", customID)
	}
	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, costFt, nil
}

