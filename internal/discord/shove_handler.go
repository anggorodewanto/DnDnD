package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ShoveCombatService is the slice of *combat.Service the /shove handler
// requires. *combat.Service satisfies it structurally; tests inject a mock
// to assert the right command is dispatched.
type ShoveCombatService interface {
	Shove(ctx context.Context, cmd combat.ShoveCommand, roller *dice.Roller) (combat.ShoveResult, error)
	Grapple(ctx context.Context, cmd combat.GrappleCommand, roller *dice.Roller) (combat.GrappleResult, error)
}

// ShoveEncounterProvider mirrors the lookups /shove needs.
type ShoveEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// ShoveHandler handles the /shove slash command. The slash form accepts a
// target (required) plus an optional `mode` of "push" (default), "prone",
// or "grapple". The grapple alias dispatches to combat.Service.Grapple so
// the player can grapple via /shove without a separate /action grapple.
type ShoveHandler struct {
	session           Session
	combatService     ShoveCombatService
	encounterProvider ShoveEncounterProvider
	roller            *dice.Roller
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
}

// NewShoveHandler constructs a /shove handler.
func NewShoveHandler(
	session Session,
	combatService ShoveCombatService,
	encounterProvider ShoveEncounterProvider,
	roller *dice.Roller,
) *ShoveHandler {
	return &ShoveHandler{
		session:           session,
		combatService:     combatService,
		encounterProvider: encounterProvider,
		roller:            roller,
	}
}

// SetChannelIDProvider wires the campaign settings provider for combat-log
// channel resolution.
func (h *ShoveHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetTurnGate wires the Phase 27 turn-ownership gate. /shove costs an
// action so the gate is invoked unconditionally on a non-exempt command.
func (h *ShoveHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// Handle processes the /shove command interaction.
func (h *ShoveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	targetStr := optionString(interaction, "target")
	if targetStr == "" {
		respondEphemeral(h.session, interaction, "Please specify a target (e.g. `/shove G2`).")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(optionString(interaction, "mode")))
	if mode == "" {
		mode = "push"
	}
	if mode != "push" && mode != "prone" && mode != "grapple" {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Unknown mode %q — use push, prone, or grapple.", mode))
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

	if !combat.IsExemptCommand("shove") && h.turnGate != nil {
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

	shover, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
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

	if mode == "grapple" {
		h.dispatchGrapple(ctx, interaction, encounter, encounterID, shover, *target, turn)
		return
	}

	h.dispatchShove(ctx, interaction, encounter, encounterID, shover, *target, turn, mode)
}

// dispatchShove invokes combat.Service.Shove for push/prone modes and
// posts the resulting log line.
func (h *ShoveHandler) dispatchShove(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	encounterID uuid.UUID,
	shover, target refdata.Combatant,
	turn refdata.Turn,
	mode string,
) {
	shoveMode := combat.ShovePush
	if mode == "prone" {
		shoveMode = combat.ShoveProne
	}

	result, err := h.combatService.Shove(ctx, combat.ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: encounter,
		Mode:      shoveMode,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Shove failed: %v", err))
		return
	}

	h.postCombatLog(ctx, encounterID, result.CombatLog)
	respondEphemeral(h.session, interaction, result.CombatLog)
}

// dispatchGrapple invokes combat.Service.Grapple when /shove was invoked
// with mode=grapple. Reuses the same channel-log + ephemeral pattern.
func (h *ShoveHandler) dispatchGrapple(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	encounterID uuid.UUID,
	grappler, target refdata.Combatant,
	turn refdata.Turn,
) {
	result, err := h.combatService.Grapple(ctx, combat.GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: encounter,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Grapple failed: %v", err))
		return
	}

	h.postCombatLog(ctx, encounterID, result.CombatLog)
	respondEphemeral(h.session, interaction, result.CombatLog)
}

// postCombatLog mirrors a combat log line to #combat-log when wired.
func (h *ShoveHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}
