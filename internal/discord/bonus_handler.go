package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// BonusCombatService is the slice of *combat.Service used by /bonus.
// *combat.Service satisfies it structurally; tests inject a mock to
// assert the right method is dispatched per subcommand.
type BonusCombatService interface {
	ActivateRage(ctx context.Context, cmd combat.RageCommand) (combat.RageResult, error)
	EndRage(ctx context.Context, cmd combat.RageCommand) (combat.RageResult, error)
	MartialArtsBonusAttack(ctx context.Context, cmd combat.MartialArtsBonusAttackCommand, roller *dice.Roller) (combat.AttackResult, error)
	StepOfTheWind(ctx context.Context, cmd combat.StepOfTheWindCommand) (combat.KiAbilityResult, error)
	PatientDefense(ctx context.Context, cmd combat.KiAbilityCommand) (combat.KiAbilityResult, error)
	FontOfMagicConvertSlot(ctx context.Context, cmd combat.FontOfMagicCommand) (combat.FontOfMagicResult, error)
	FontOfMagicCreateSlot(ctx context.Context, cmd combat.FontOfMagicCommand) (combat.FontOfMagicResult, error)
	LayOnHands(ctx context.Context, cmd combat.LayOnHandsCommand) (combat.LayOnHandsResult, error)
	GrantBardicInspiration(ctx context.Context, cmd combat.BardicInspirationCommand) (combat.BardicInspirationResult, error)
}

// BonusEncounterProvider is the lookup surface /bonus needs.
type BonusEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// BonusHandler handles the /bonus slash command. The first option is the
// subcommand name (rage / end-rage / martial-arts / step-of-the-wind /
// patient-defense / font-of-magic / lay-on-hands / bardic-inspiration);
// the second option is a freeform args string parsed per subcommand.
//
// Auto-prompts (Stunning Strike, Smite slot picker, Bardic Inspiration
// 30s usage timeout) live in their own task (med-43) and are NOT wired
// here per the crit-01a scope.
type BonusHandler struct {
	session           Session
	combatService     BonusCombatService
	encounterProvider BonusEncounterProvider
	roller            *dice.Roller
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
}

// NewBonusHandler constructs a /bonus handler.
func NewBonusHandler(
	session Session,
	combatService BonusCombatService,
	encounterProvider BonusEncounterProvider,
	roller *dice.Roller,
) *BonusHandler {
	return &BonusHandler{
		session:           session,
		combatService:     combatService,
		encounterProvider: encounterProvider,
		roller:            roller,
	}
}

// SetChannelIDProvider wires the campaign settings provider for
// combat-log mirroring.
func (h *BonusHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetTurnGate wires the Phase 27 turn-ownership gate.
func (h *BonusHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// bonusContext is the resolved per-invocation state shared by every
// subcommand dispatch. Keeping it local prevents each subcommand handler
// from re-running the same lookups.
type bonusContext struct {
	encounter   refdata.Encounter
	encounterID uuid.UUID
	turn        refdata.Turn
	actor       refdata.Combatant
	combatants  []refdata.Combatant
}

// Handle processes the /bonus command interaction.
func (h *BonusHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	action := strings.ToLower(strings.TrimSpace(optionString(interaction, "action")))
	if action == "" {
		respondEphemeral(h.session, interaction, "Please specify a bonus action (e.g. `/bonus rage`).")
		return
	}
	args := strings.TrimSpace(optionString(interaction, "args"))

	bctx, ok := h.resolveContext(ctx, interaction)
	if !ok {
		return
	}

	switch action {
	case "rage":
		h.dispatchRage(ctx, interaction, bctx)
	case "end-rage", "endrage":
		h.dispatchEndRage(ctx, interaction, bctx)
	case "martial-arts", "martialarts":
		h.dispatchMartialArts(ctx, interaction, bctx, args)
	case "step-of-the-wind", "stepofthewind":
		h.dispatchStepOfTheWind(ctx, interaction, bctx, args)
	case "patient-defense", "patientdefense":
		h.dispatchPatientDefense(ctx, interaction, bctx)
	case "font-of-magic", "fontofmagic":
		h.dispatchFontOfMagic(ctx, interaction, bctx, args)
	case "lay-on-hands", "layonhands":
		h.dispatchLayOnHands(ctx, interaction, bctx, args)
	case "bardic-inspiration", "bardicinspiration":
		h.dispatchBardicInspiration(ctx, interaction, bctx, args)
	default:
		respondEphemeral(h.session, interaction, fmt.Sprintf("Unknown bonus action %q. Try rage, end-rage, martial-arts, step-of-the-wind, patient-defense, font-of-magic, lay-on-hands, bardic-inspiration.", action))
	}
}

// resolveContext loads the encounter / current turn / acting combatant
// and runs the turn-ownership gate. Sends an ephemeral and returns
// (zero, false) on any failure so callers can early-return.
func (h *BonusHandler) resolveContext(ctx context.Context, interaction *discordgo.Interaction) (bonusContext, bool) {
	userID := discordUserID(interaction)
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "You are not in an active encounter.")
		return bonusContext{}, false
	}
	encounter, err := h.encounterProvider.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load encounter.")
		return bonusContext{}, false
	}
	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return bonusContext{}, false
	}
	if !combat.IsExemptCommand("bonus") && h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRelease(ctx, encounterID, userID); gateErr != nil {
			respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			return bonusContext{}, false
		}
	}
	turn, err := h.encounterProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load turn.")
		return bonusContext{}, false
	}
	actor, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return bonusContext{}, false
	}
	combatants, err := h.encounterProvider.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return bonusContext{}, false
	}
	return bonusContext{
		encounter:   encounter,
		encounterID: encounterID,
		turn:        turn,
		actor:       actor,
		combatants:  combatants,
	}, true
}

func (h *BonusHandler) dispatchRage(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext) {
	result, err := h.combatService.ActivateRage(ctx, combat.RageCommand{
		Combatant: bctx.actor,
		Turn:      bctx.turn,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Rage failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

func (h *BonusHandler) dispatchEndRage(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext) {
	result, err := h.combatService.EndRage(ctx, combat.RageCommand{
		Combatant: bctx.actor,
		Turn:      bctx.turn,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("End rage failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

func (h *BonusHandler) dispatchMartialArts(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext, args string) {
	target, ok := h.resolveTargetArg(interaction, bctx.combatants, args, "martial-arts <target>")
	if !ok {
		return
	}
	result, err := h.combatService.MartialArtsBonusAttack(ctx, combat.MartialArtsBonusAttackCommand{
		Attacker: bctx.actor,
		Target:   *target,
		Turn:     bctx.turn,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Martial Arts failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, combat.FormatAttackLog(result))
}

func (h *BonusHandler) dispatchStepOfTheWind(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext, args string) {
	mode := strings.ToLower(args)
	if mode != "dash" && mode != "disengage" {
		respondEphemeral(h.session, interaction, "Step of the Wind requires `dash` or `disengage` (e.g. `/bonus step-of-the-wind dash`).")
		return
	}
	result, err := h.combatService.StepOfTheWind(ctx, combat.StepOfTheWindCommand{
		KiAbilityCommand: combat.KiAbilityCommand{Combatant: bctx.actor, Turn: bctx.turn},
		Mode:             mode,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Step of the Wind failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

func (h *BonusHandler) dispatchPatientDefense(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext) {
	result, err := h.combatService.PatientDefense(ctx, combat.KiAbilityCommand{
		Combatant: bctx.actor,
		Turn:      bctx.turn,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Patient Defense failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

// dispatchFontOfMagic parses `convert <slotLevel>` or `create <slotLevel>`
// and routes to the matching service method. Per crit-01a scope only the
// two existing service methods are wired; new metamagic UI is out of scope.
func (h *BonusHandler) dispatchFontOfMagic(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext, args string) {
	tokens := strings.Fields(args)
	if len(tokens) != 2 {
		respondEphemeral(h.session, interaction, "Font of Magic requires `convert <slotLevel>` or `create <slotLevel>` (e.g. `/bonus font-of-magic convert 2`).")
		return
	}
	mode := strings.ToLower(tokens[0])
	level, err := strconv.Atoi(tokens[1])
	if err != nil || level < 1 {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid slot level %q.", tokens[1]))
		return
	}

	switch mode {
	case "convert":
		result, err := h.combatService.FontOfMagicConvertSlot(ctx, combat.FontOfMagicCommand{
			CasterID:  bctx.actor.ID,
			Turn:      bctx.turn,
			SlotLevel: level,
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Font of Magic failed: %v", err))
			return
		}
		h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
	case "create":
		result, err := h.combatService.FontOfMagicCreateSlot(ctx, combat.FontOfMagicCommand{
			CasterID:        bctx.actor.ID,
			Turn:            bctx.turn,
			CreateSlotLevel: level,
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Font of Magic failed: %v", err))
			return
		}
		h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
	default:
		respondEphemeral(h.session, interaction, fmt.Sprintf("Unknown Font of Magic mode %q (use convert or create).", mode))
	}
}

// dispatchLayOnHands parses `<target> <hp> [poison] [disease]` and dispatches.
// Lay on Hands is technically an action; per crit-01a the slash entrypoint
// lives under /bonus by directive.
func (h *BonusHandler) dispatchLayOnHands(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext, args string) {
	tokens := strings.Fields(args)
	if len(tokens) < 2 {
		respondEphemeral(h.session, interaction, "Lay on Hands requires `<target> <hp> [poison] [disease]` (e.g. `/bonus lay-on-hands AR 10`).")
		return
	}
	target, err := combat.ResolveTarget(tokens[0], bctx.combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", tokens[0]))
		return
	}
	hp, err := strconv.Atoi(tokens[1])
	if err != nil || hp < 0 {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid HP value %q.", tokens[1]))
		return
	}
	curePoison, cureDisease := parseFlagTokens(tokens[2:])

	result, err := h.combatService.LayOnHands(ctx, combat.LayOnHandsCommand{
		Paladin:     bctx.actor,
		Target:      *target,
		Turn:        bctx.turn,
		HP:          hp,
		CurePoison:  curePoison,
		CureDisease: cureDisease,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Lay on Hands failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

func (h *BonusHandler) dispatchBardicInspiration(ctx context.Context, interaction *discordgo.Interaction, bctx bonusContext, args string) {
	target, ok := h.resolveTargetArg(interaction, bctx.combatants, args, "bardic-inspiration <target>")
	if !ok {
		return
	}
	result, err := h.combatService.GrantBardicInspiration(ctx, combat.BardicInspirationCommand{
		Bard:   bctx.actor,
		Target: *target,
		Turn:   bctx.turn,
		Now:    time.Now(),
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Bardic Inspiration failed: %v", err))
		return
	}
	h.respondAndLog(interaction, bctx.encounterID, result.CombatLog)
}

// resolveTargetArg parses a `<target>` arg-string and resolves it via
// combat.ResolveTarget. Sends an ephemeral and returns (nil, false) if
// the string is empty or no combatant matches.
func (h *BonusHandler) resolveTargetArg(interaction *discordgo.Interaction, combatants []refdata.Combatant, args, usage string) (*refdata.Combatant, bool) {
	tokens := strings.Fields(args)
	if len(tokens) == 0 {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Missing target — usage: %s.", usage))
		return nil, false
	}
	target, err := combat.ResolveTarget(tokens[0], combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", tokens[0]))
		return nil, false
	}
	return target, true
}

// respondAndLog sends the ephemeral confirmation to the invoker and
// mirrors the same log line to #combat-log when the channel provider is
// wired. Centralized so every subcommand uses the same shape.
func (h *BonusHandler) respondAndLog(interaction *discordgo.Interaction, encounterID uuid.UUID, log string) {
	if log == "" {
		log = "Bonus action resolved."
	}
	h.postCombatLog(context.Background(), encounterID, log)
	respondEphemeral(h.session, interaction, log)
}

// postCombatLog mirrors a combat log line to #combat-log when wired.
func (h *BonusHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}

// parseFlagTokens scans the trailing tokens for "poison" and "disease"
// flags (case-insensitive). Both flags are independent.
func parseFlagTokens(tokens []string) (curePoison, cureDisease bool) {
	for _, t := range tokens {
		switch strings.ToLower(t) {
		case "poison":
			curePoison = true
		case "disease":
			cureDisease = true
		}
	}
	return curePoison, cureDisease
}
