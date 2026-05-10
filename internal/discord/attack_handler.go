package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
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

	if offhand {
		h.dispatchOffhand(ctx, interaction, encounterID, attacker, *target, turn)
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
	}

	result, err := h.combatService.Attack(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Attack failed: %v", err))
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
) {
	cmd := combat.OffhandAttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}
	result, err := h.combatService.OffhandAttack(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Off-hand attack failed: %v", err))
		return
	}
	logLine := combat.FormatAttackLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
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
