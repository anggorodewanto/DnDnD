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

// InteractCombatService runs the full /interact flow: free→action fallback,
// auto-resolvable pattern detection, and pending_actions row insertion so the
// DM dashboard / #dm-queue can resolve non-auto interactions. *combat.Service
// implements it.
type InteractCombatService interface {
	Interact(ctx context.Context, cmd combat.InteractCommand) (combat.InteractResult, error)
}

// InteractHandler handles /interact <description>.
//
// SR-005: when a combat service is wired (SetCombatService) the handler
// routes through combat.Interact, which implements the spec'd free→action
// fallback (Phase 74 spec line 1200) and creates a pending_actions row for
// the DM. When nil, the handler keeps the legacy UseResource(turn,
// ResourceFreeInteract) path used by handler test fixtures that pre-date
// SR-005 — second /interact in the legacy path is rejected outright, which
// is the spec-violating behavior SR-005 fixes.
type InteractHandler struct {
	session           Session
	encounterProvider InteractEncounterProvider
	turnStore         InteractTurnStore
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
	combatSvc         InteractCombatService
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

// SetCombatService wires the combat.Interact service so /interact runs the
// spec'd free→action fallback and creates a pending_actions row for the DM
// (SR-005). With no service wired the handler keeps the legacy
// UseResource-only path.
func (h *InteractHandler) SetCombatService(svc InteractCombatService) {
	h.combatSvc = svc
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

	// SR-005: route through combat.Interact when wired. The service owns
	// turn updates + free→action fallback + pending_actions insertion.
	if h.combatSvc != nil {
		h.handleViaCombat(ctx, interaction, encounterID, turn, desc)
		return
	}

	h.handleLegacy(ctx, interaction, encounterID, turn, desc)
}

// handleViaCombat is the SR-005 path. combat.Interact owns the resource
// gating (free→action fallback), pending_actions insertion, and the human-
// readable combat log. The handler just looks up the combatant for the
// service command, posts the resulting log line to #combat-log, and echoes
// it back to the invoker.
func (h *InteractHandler) handleViaCombat(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	turn refdata.Turn,
	desc string,
) {
	combatant, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return
	}

	result, err := h.combatSvc.Interact(ctx, combat.InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: desc,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot interact: %v", err))
		return
	}

	if result.CombatLog != "" {
		h.postCombatLog(ctx, encounterID, result.CombatLog)
	}
	respondEphemeral(h.session, interaction, result.CombatLog)
}

// handleLegacy is the pre-SR-005 path used when no combat service is wired
// (handler test fixtures). It marks FreeInteractUsed directly and rejects
// the second interact outright. Spec-divergent on purpose for backward
// compatibility; production wiring always passes through handleViaCombat.
func (h *InteractHandler) handleLegacy(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	turn refdata.Turn,
	desc string,
) {
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
