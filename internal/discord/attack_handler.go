package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// AttackCombatService is the slice of *combat.Service that /attack uses.
// *combat.Service satisfies it structurally.
type AttackCombatService interface {
	Attack(ctx context.Context, cmd combat.AttackCommand, roller *dice.Roller) (combat.AttackResult, error)
	OffhandAttack(ctx context.Context, cmd combat.OffhandAttackCommand, roller *dice.Roller) (combat.AttackResult, error)
}

// AttackEncounterProvider is the lookup surface /attack needs.
type AttackEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// AttackMapProvider resolves the encounter map so the attack handler can
// load wall segments for cover calculation. C-33-followup: the attack
// service uses `AttackCommand.Walls` to compute attacker→target cover, so
// the slash-command pipeline must populate Walls. nil-safe — when unset,
// the handler degrades to "no wall cover" rather than failing the attack.
type AttackMapProvider interface {
	GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
}

// AttackHandler handles the /attack slash command. Wires Phases 34-38 of
// the combat spec: weapon override, two-handed grip, GWM/Sharpshooter/
// Reckless modifier flags, and the off-hand bonus-action attack
// (toggled via the `offhand` option, which redirects to OffhandAttack).
type AttackHandler struct {
	session           Session
	combatService     AttackCombatService
	encounterProvider AttackEncounterProvider
	roller            *dice.Roller
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
	// C-33-followup: optional map provider used to load wall segments and
	// populate AttackCommand.Walls so attacker→target cover applies on
	// slash-command attacks. nil-safe — when unset the cover degrades to
	// "no wall cover".
	mapProvider AttackMapProvider
}

// NewAttackHandler constructs an /attack handler.
func NewAttackHandler(
	session Session,
	combatService AttackCombatService,
	encounterProvider AttackEncounterProvider,
	roller *dice.Roller,
) *AttackHandler {
	return &AttackHandler{
		session:           session,
		combatService:     combatService,
		encounterProvider: encounterProvider,
		roller:            roller,
	}
}

// SetChannelIDProvider wires the campaign settings provider for
// combat-log mirroring.
func (h *AttackHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetTurnGate wires the Phase 27 turn-ownership gate. /attack costs an
// attack (and possibly the action) so the gate is invoked.
func (h *AttackHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// SetMapProvider wires the optional map lookup used by C-33-followup to
// populate AttackCommand.Walls. Pass nil to disable wall-based cover.
func (h *AttackHandler) SetMapProvider(p AttackMapProvider) {
	h.mapProvider = p
}

// loadWalls best-effort fetches map wall segments for an encounter, mirroring
// cast_handler.loadWalls. Any failure path returns nil so the cover calc
// degrades to "no wall cover" rather than failing the attack.
func (h *AttackHandler) loadWalls(ctx context.Context, encounter refdata.Encounter) []renderer.WallSegment {
	if h.mapProvider == nil {
		return nil
	}
	if !encounter.MapID.Valid {
		return nil
	}
	mapData, err := h.mapProvider.GetMapByID(ctx, encounter.MapID.UUID)
	if err != nil {
		return nil
	}
	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		return nil
	}
	return md.Walls
}

// Handle processes the /attack command interaction.
func (h *AttackHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	targetStr := optionString(interaction, "target")
	if targetStr == "" {
		respondEphemeral(h.session, interaction, "Please specify a target (e.g. `/attack G2`).")
		return
	}

	weapon := optionString(interaction, "weapon")
	gwm := optionBool(interaction, "gwm")
	sharpshooter := optionBool(interaction, "sharpshooter")
	reckless := optionBool(interaction, "reckless")
	twoHanded := optionBool(interaction, "twohanded")
	offhand := optionBool(interaction, "offhand")
	thrown := optionBool(interaction, "thrown")
	improvised := optionBool(interaction, "improvised")

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

	if !combat.IsExemptCommand("attack") && h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRelease(ctx, encounterID, userID); gateErr != nil {
			respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			return
		}
	}

	turn, err := h.encounterProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load turn.")
		return
	}

	attacker, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return
	}

	// C-43-block-commands: a dying or incapacitated combatant cannot
	// take actions; reject before the service runs.
	if msg, blocked := incapacitatedRejection(attacker); blocked {
		respondEphemeral(h.session, interaction, msg)
		return
	}

	combatants, err := h.encounterProvider.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	target, err := combat.ResolveTarget(targetStr, combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", targetStr))
		return
	}

	walls := h.loadWalls(ctx, encounter)

	if offhand {
		h.dispatchOffhand(ctx, interaction, encounterID, attacker, *target, turn, walls)
		return
	}

	cmd := combat.AttackCommand{
		Attacker:       attacker,
		Target:         *target,
		Turn:           turn,
		WeaponOverride: weapon,
		GWM:            gwm,
		Sharpshooter:   sharpshooter,
		Reckless:       reckless,
		TwoHanded:      twoHanded,
		Thrown:         thrown,
		IsImprovised:   improvised,
		Walls:          walls,
	}

	result, err := h.combatService.Attack(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, formatAttackError(err))
		return
	}

	logLine := combat.FormatAttackLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// dispatchOffhand routes the off-hand bonus-action attack through the
// dedicated OffhandAttack service so two-weapon fighting bookkeeping
// (no ability modifier on damage unless TWF style) is handled correctly.
func (h *AttackHandler) dispatchOffhand(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	attacker, target refdata.Combatant,
	turn refdata.Turn,
	walls []renderer.WallSegment,
) {
	cmd := combat.OffhandAttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
		Walls:    walls,
	}
	result, err := h.combatService.OffhandAttack(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, formatOffhandAttackError(err))
		return
	}
	logLine := combat.FormatAttackLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// formatAttackError translates a service-level attack error into the
// user-visible string. C-32: when the service reports an out-of-range
// rejection, route through combat.FormatRangeRejection so the slash-command
// pipeline emits the same "Target is out of range — Xft away (max Yft)."
// string the helper renders elsewhere. Falls back to the legacy
// "Attack failed: <err>" wording for all other errors.
func formatAttackError(err error) string {
	if msg, ok := rangeRejectionMessage(err); ok {
		return msg
	}
	return fmt.Sprintf("Attack failed: %v", err)
}

// formatOffhandAttackError mirrors formatAttackError for the off-hand path.
func formatOffhandAttackError(err error) string {
	if msg, ok := rangeRejectionMessage(err); ok {
		return msg
	}
	return fmt.Sprintf("Off-hand attack failed: %v", err)
}

// rangeRejectionMessage parses the attack service's "out of range: Xft away
// (max Yft)" sentinel and returns the formatted helper string. Returns
// (_, false) for any error that isn't a range rejection so the caller can
// fall back to its default wording.
func rangeRejectionMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	const prefix = "out of range: "
	idx := strings.Index(err.Error(), prefix)
	if idx < 0 {
		return "", false
	}
	rest := err.Error()[idx+len(prefix):]
	var dist, maxR int
	if _, scanErr := fmt.Sscanf(rest, "%dft away (max %dft)", &dist, &maxR); scanErr != nil {
		return "", false
	}
	return combat.FormatRangeRejection(dist, maxR), true
}

// postCombatLog mirrors a combat log line to #combat-log when wired.
func (h *AttackHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}

// optionBool extracts a named boolean option from an interaction's
// command data. Missing or non-bool options return false.
func optionBool(interaction *discordgo.Interaction, name string) bool {
	data, ok := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if !ok {
		return false
	}
	for _, opt := range data.Options {
		if opt.Name == name {
			return opt.BoolValue()
		}
	}
	return false
}
