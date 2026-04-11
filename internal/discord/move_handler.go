package discord

import (
	"context"
	"fmt"
	"strings"

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
	UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
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

// MoveEncounterProvider resolves the active encounter a given Discord user is
// participating in (via their character's combatant entry). Phase 105 routes
// commands to the encounter the invoker belongs to rather than picking an
// arbitrary active encounter in the guild, so simultaneous encounters in a
// single campaign are disambiguated per-player.
type MoveEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
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

	guildID := interaction.GuildID

	// Phase 105: resolve the encounter via the invoking player's combatant
	// entry rather than a guild-wide "active encounter" lookup, so two
	// simultaneous encounters in the same campaign are disambiguated.
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

	// TODO: turn ownership validation will be wired when full turn lock is available

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

	// TODO: look up creature size from CreatureRefID when available
	sizeCategory := pathfinding.SizeMedium

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: sizeCategory,
	}

	// Check if combatant is prone and hasn't already stood this turn
	isProne := combat.HasCondition(combatant.Conditions, "prone")
	if isProne && !turn.HasStoodThisTurn {
		// Show Stand & Move / Crawl choice prompt
		// We use maxSpeed=30 as default; TODO: look up actual speed
		maxSpeed := 30
		standID := fmt.Sprintf("prone_stand:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, maxSpeed)
		crawlID := fmt.Sprintf("prone_crawl:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, maxSpeed)
		cancelID := fmt.Sprintf("move_cancel:%s", turn.ID.String())

		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are prone. How do you want to move?",
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Stand & Move",
								Style:    discordgo.PrimaryButton,
								CustomID: standID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f9cd"},
							},
							discordgo.Button{
								Label:    "Crawl",
								Style:    discordgo.PrimaryButton,
								CustomID: crawlID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f41b"},
							},
							discordgo.Button{
								Label:    "Cancel",
								Style:    discordgo.DangerButton,
								CustomID: cancelID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
							},
						},
					},
				},
			},
		})
		return
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

	// Get combatant to preserve altitude during horizontal movement
	combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
	currentAltitude := int32(0)
	if getErr == nil {
		currentAltitude = combatant.AltitudeFt
	}

	// Update combatant position (preserving current altitude)
	destLabel := renderer.ColumnLabel(destCol)
	_, err = h.combatService.UpdateCombatantPosition(ctx, combatantID, destLabel, int32(destRow+1), currentAltitude)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	remaining := combat.FormatRemainingResources(updatedTurn, nil)
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
		if c.ID == mover.ID || !c.IsAlive {
			continue
		}
		col, row, err := renderer.ParseCoordinate(c.PositionCol + fmt.Sprintf("%d", c.PositionRow))
		if err != nil {
			continue
		}
		occupants = append(occupants, pathfinding.Occupant{
			Col:          col,
			Row:          row,
			IsAlly:       c.IsNpc == mover.IsNpc, // ally if same faction
			SizeCategory: pathfinding.SizeMedium,  // default; would look up creature size
			AltitudeFt:   int(c.AltitudeFt),
		})
	}
	return occupants
}

// buildGridForTurn fetches the map and combatant data needed to build a pathfinding grid.
// Returns an error string for the user if any step fails (empty string on success).
func (h *MoveHandler) buildGridForTurn(ctx context.Context, turn refdata.Turn, mover refdata.Combatant) (*pathfinding.Grid, string) {
	encounter, err := h.combatService.GetEncounter(ctx, turn.EncounterID)
	if err != nil || !encounter.MapID.Valid {
		return nil, "Failed to get map data."
	}

	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		return nil, "Failed to load map data."
	}

	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		return nil, "Failed to parse map data."
	}

	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, turn.EncounterID)
	if err != nil {
		return nil, "Failed to list combatants."
	}

	return &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: buildOccupants(allCombatants, mover),
	}, ""
}

// respondUpdateConfirmCancel sends an update-message response with Confirm and Cancel buttons.
func (h *MoveHandler) respondUpdateConfirmCancel(interaction *discordgo.Interaction, msg, confirmID, cancelID string) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm",
							Style:    discordgo.SuccessButton,
							CustomID: confirmID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u2705"},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: cancelID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
						},
					},
				},
			},
		},
	})
}

// HandleProneStandAndMove handles the Stand & Move button click for a prone combatant.
// It validates the move with stand cost deducted, then shows a confirmation prompt.
func (h *MoveHandler) HandleProneStandAndMove(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, maxSpeed int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	grid, errMsg := h.buildGridForTurn(ctx, turn, combatant)
	if errMsg != "" {
		respondEphemeral(h.session, interaction, errMsg)
		return
	}

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := combat.ValidateProneMoveStandAndMove(moveReq, maxSpeed)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d:stand_and_move:%d",
		turnID.String(), combatantID.String(), destCol, destRow, result.CostFt, result.StandCostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turnID.String())

	h.respondUpdateConfirmCancel(interaction, combat.FormatMoveConfirmation(result), confirmID, cancelID)
}

// HandleProneCrawl handles the Crawl button click for a prone combatant.
// It validates the move with crawl costs, then shows a confirmation prompt.
func (h *MoveHandler) HandleProneCrawl(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	grid, errMsg := h.buildGridForTurn(ctx, turn, combatant)
	if errMsg != "" {
		respondEphemeral(h.session, interaction, errMsg)
		return
	}

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := combat.ValidateProneMoveCrawl(moveReq)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d:crawl:0",
		turnID.String(), combatantID.String(), destCol, destRow, result.CostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turnID.String())

	h.respondUpdateConfirmCancel(interaction, combat.FormatMoveConfirmation(result), confirmID, cancelID)
}

// HandleMoveConfirmWithMode processes a move confirmation with an explicit move mode.
// For stand_and_move, it deducts stand cost + path cost and sets HasStoodThisTurn.
// For crawl, it deducts the crawl cost only.
func (h *MoveHandler) HandleMoveConfirmWithMode(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, costFt int, mode string, standCostFt int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	totalCost := int32(costFt)
	if mode == "stand_and_move" {
		totalCost += int32(standCostFt)
	}

	updatedTurn, err := combat.UseMovement(turn, totalCost)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot move: %v", err))
		return
	}

	if mode == "stand_and_move" {
		updatedTurn.HasStoodThisTurn = true

		// Remove prone condition from combatant
		combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
		if getErr == nil {
			newConds, removeErr := combat.RemoveCondition(combatant.Conditions, "prone")
			if removeErr == nil {
				_, _ = h.combatService.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
					ID:              combatantID,
					Conditions:      newConds,
					ExhaustionLevel: combatant.ExhaustionLevel,
				})
			}
		}
	}

	_, err = h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn))
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update turn resources.")
		return
	}

	combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
	currentAltitude := int32(0)
	if getErr == nil {
		currentAltitude = combatant.AltitudeFt
	}

	destLabel := renderer.ColumnLabel(destCol)
	_, err = h.combatService.UpdateCombatantPosition(ctx, combatantID, destLabel, int32(destRow+1), currentAltitude)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	remaining := combat.FormatRemainingResources(updatedTurn, nil)
	var msg string
	if mode == "stand_and_move" {
		msg = fmt.Sprintf("\U0001f3c3 Stood up and moved to %s%d. %s", destLabel, destRow+1, remaining)
	} else {
		msg = fmt.Sprintf("\U0001f41b Crawled to %s%d. %s", destLabel, destRow+1, remaining)
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    msg,
			Components: []discordgo.MessageComponent{},
		},
	})
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

// ParseMoveConfirmWithModeData parses a move confirm custom ID that includes mode and stand cost.
// Format: move_confirm:<turnID>:<combatantID>:<col>:<row>:<cost>:<mode>:<standCost>
func ParseMoveConfirmWithModeData(customID string) (turnID, combatantID uuid.UUID, destCol, destRow, costFt int, mode string, standCostFt int, err error) {
	var turnStr, combatantStr string
	n, scanErr := fmt.Sscanf(customID, "move_confirm:%36s:%36s:%d:%d:%d:%s",
		&turnStr, &combatantStr, &destCol, &destRow, &costFt, &mode)
	if scanErr != nil || n != 6 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid move confirm with mode data: %q", customID)
	}

	// mode may contain ":standCost" suffix — split at the last colon
	if idx := strings.LastIndex(mode, ":"); idx >= 0 {
		_, scanErr = fmt.Sscanf(mode[idx+1:], "%d", &standCostFt)
		if scanErr != nil {
			return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid stand cost in: %q", customID)
		}
		mode = mode[:idx]
	}

	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, costFt, mode, standCostFt, nil
}

// ParseProneMoveData parses a prone movement button custom ID.
// Format: <prefix>:<turnID>:<combatantID>:<col>:<row>:<maxSpeed>
func ParseProneMoveData(customID string, prefix string) (turnID, combatantID uuid.UUID, destCol, destRow, maxSpeed int, err error) {
	var turnStr, combatantStr string
	format := prefix + ":%36s:%36s:%d:%d:%d"
	n, scanErr := fmt.Sscanf(customID, format, &turnStr, &combatantStr, &destCol, &destRow, &maxSpeed)
	if scanErr != nil || n != 5 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid prone move data: %q", customID)
	}
	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, maxSpeed, nil
}


