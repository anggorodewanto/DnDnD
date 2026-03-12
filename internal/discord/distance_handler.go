package discord

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// DistanceHandler handles the /distance slash command.
type DistanceHandler struct {
	session           Session
	combatService     MoveService
	turnProvider      MoveTurnProvider
	encounterProvider MoveEncounterProvider
}

// NewDistanceHandler creates a new DistanceHandler.
func NewDistanceHandler(
	session Session,
	combatService MoveService,
	turnProvider MoveTurnProvider,
	encounterProvider MoveEncounterProvider,
) *DistanceHandler {
	return &DistanceHandler{
		session:           session,
		combatService:     combatService,
		turnProvider:      turnProvider,
		encounterProvider: encounterProvider,
	}
}

// Handle processes the /distance command interaction.
func (h *DistanceHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if len(data.Options) == 0 {
		respondEphemeral(h.session, interaction, "Please provide a target (e.g. `/distance G1`).")
		return
	}

	target1Str := data.Options[0].StringValue()
	var target2Str string
	if len(data.Options) > 1 {
		target2Str = data.Options[1].StringValue()
	}

	encounterID, err := h.encounterProvider.GetActiveEncounterID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No active encounter in this server.")
		return
	}

	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get encounter data.")
		return
	}

	combatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	if target2Str != "" {
		h.handleTwoTargets(interaction, target1Str, target2Str, combatants)
		return
	}

	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	turn, err := h.turnProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get turn data.")
		return
	}

	selfCombatant, err := h.combatService.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	target, err := combat.ResolveTarget(target1Str, combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", target1Str))
		return
	}

	dist := computeCombatantDistance(selfCombatant, *target)
	msg := combat.FormatDistance(dist, "You", combatantLabel(*target))
	respondEphemeral(h.session, interaction, msg)
}

func (h *DistanceHandler) handleTwoTargets(interaction *discordgo.Interaction, t1Str, t2Str string, combatants []refdata.Combatant) {
	t1, err := combat.ResolveTarget(t1Str, combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", t1Str))
		return
	}

	t2, err := combat.ResolveTarget(t2Str, combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", t2Str))
		return
	}

	dist := computeCombatantDistance(*t1, *t2)
	msg := combat.FormatDistance(dist, combatantLabel(*t1), combatantLabel(*t2))
	respondEphemeral(h.session, interaction, msg)
}

// combatantLabel formats a combatant as "DisplayName (ShortID)".
func combatantLabel(c refdata.Combatant) string {
	return fmt.Sprintf("%s (%s)", c.DisplayName, c.ShortID)
}

// computeCombatantDistance calculates the 3D Euclidean distance between two combatants.
func computeCombatantDistance(a, b refdata.Combatant) int {
	aCol, aRow := parseCombatantPos(a)
	bCol, bRow := parseCombatantPos(b)
	return combat.Distance3D(aCol, aRow, int(a.AltitudeFt), bCol, bRow, int(b.AltitudeFt))
}

// parseCombatantPos converts a combatant's position to 0-based col/row.
func parseCombatantPos(c refdata.Combatant) (col, row int) {
	col, row, err := renderer.ParseCoordinate(c.PositionCol + strconv.Itoa(int(c.PositionRow)))
	if err != nil {
		return 0, 0
	}
	return col, row
}
