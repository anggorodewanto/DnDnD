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

// AttackClassFeatureService is the slice of *combat.Service the attack
// handler needs to apply Stunning Strike, Divine Smite, and Bardic
// Inspiration after a successful hit. *combat.Service satisfies this
// structurally. The interface is optional — when no service is wired the
// post-hit prompts simply skip the on-choice mechanic application.
// (D-48b/D-49/D-51 follow-up)
type AttackClassFeatureService interface {
	StunningStrike(ctx context.Context, cmd combat.StunningStrikeCommand, roller *dice.Roller) (combat.StunningStrikeResult, error)
	DivineSmite(ctx context.Context, cmd combat.DivineSmiteCommand, roller *dice.Roller) (combat.DivineSmiteResult, error)
	UseBardicInspiration(ctx context.Context, cmd combat.UseBardicInspirationCommand, roller *dice.Roller) (combat.UseBardicInspirationResult, error)
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
	// D-48b/D-49/D-51 follow-up: optional post-hit prompt poster + service.
	// Both nil-safe: when either is unset, the eligibility flags on
	// AttackResult are simply ignored and no prompt fires.
	classFeaturePrompts *ClassFeaturePromptPoster
	classFeatureService AttackClassFeatureService
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

// HasMapProvider reports whether a non-nil AttackMapProvider has been
// wired. Production wiring tests use this to detect the C-33-followup
// regression (nil map provider → no wall cover on /attack).
func (h *AttackHandler) HasMapProvider() bool { return h.mapProvider != nil }

// SetClassFeaturePromptPoster wires the optional post-hit prompt poster
// used to fire Stunning Strike / Divine Smite / Bardic Inspiration buttons
// after a successful /attack hit. nil-safe.
func (h *AttackHandler) SetClassFeaturePromptPoster(p *ClassFeaturePromptPoster) {
	h.classFeaturePrompts = p
}

// SetClassFeatureService wires the combat-side service the post-hit
// prompt callbacks invoke (StunningStrike / DivineSmite / UseBardicInspiration).
// nil-safe.
func (h *AttackHandler) SetClassFeatureService(s AttackClassFeatureService) {
	h.classFeatureService = s
}

// HasClassFeaturePromptPoster reports whether the post-hit prompt poster
// has been wired. Used by production-wiring tests to detect the
// D-48b/D-49/D-51 follow-up regression.
func (h *AttackHandler) HasClassFeaturePromptPoster() bool { return h.classFeaturePrompts != nil }

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
		h.dispatchOffhand(ctx, interaction, encounterID, attacker, *target, turn, walls, encounter)
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

	// D-48b/D-49/D-51 follow-up: surface post-hit class-feature prompts
	// (Stunning Strike / Divine Smite / Bardic Inspiration) when the service
	// flagged the attacker as eligible. nil-safe: no-op when no poster is wired.
	h.postClassFeaturePrompts(ctx, interaction, encounterID, attacker, *target, encounter, result)
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
	encounter refdata.Encounter,
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
	h.postClassFeaturePrompts(ctx, interaction, encounterID, attacker, target, encounter, result)
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

// postClassFeaturePrompts reads the post-hit eligibility flags from result
// and fires the corresponding Stunning Strike / Divine Smite / Bardic
// Inspiration prompts. Each prompt's OnChoice closure invokes the wired
// combat service (StunningStrike / DivineSmite / UseBardicInspiration) and
// mirrors the resulting combat log line to #combat-log. Forfeit / Skip
// branches consume no resources. Nil prompt-poster or service is a no-op.
// (D-48b/D-49/D-51 follow-up)
func (h *AttackHandler) postClassFeaturePrompts(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	attacker, target refdata.Combatant,
	encounter refdata.Encounter,
	result combat.AttackResult,
) {
	if h.classFeaturePrompts == nil {
		return
	}
	channelID := h.resolvePromptChannel(ctx, interaction, encounterID)
	if channelID == "" {
		return
	}

	if result.PromptStunningStrikeEligible {
		_ = h.classFeaturePrompts.PromptStunningStrike(StunningStrikePromptArgs{
			ChannelID:    channelID,
			AttackerName: attacker.DisplayName,
			TargetName:   target.DisplayName,
			KiAvailable:  result.PromptStunningStrikeKiAvailable,
		}, func(res StunningStrikePromptResult) {
			if res.Forfeited || !res.UseKi || h.classFeatureService == nil {
				return
			}
			ssResult, err := h.classFeatureService.StunningStrike(ctx, combat.StunningStrikeCommand{
				Attacker:     attacker,
				Target:       target,
				CurrentRound: int(encounter.RoundNumber),
			}, h.roller)
			if err == nil && ssResult.CombatLog != "" {
				h.postCombatLog(ctx, encounterID, ssResult.CombatLog)
			}
		})
	}

	if result.PromptDivineSmiteEligible && len(result.PromptDivineSmiteSlots) > 0 {
		_ = h.classFeaturePrompts.PromptDivineSmite(DivineSmitePromptArgs{
			ChannelID:      channelID,
			AttackerName:   attacker.DisplayName,
			TargetName:     target.DisplayName,
			AvailableSlots: result.PromptDivineSmiteSlots,
		}, func(res DivineSmitePromptResult) {
			if res.Forfeited || !res.UseSlot || h.classFeatureService == nil {
				return
			}
			dsResult, err := h.classFeatureService.DivineSmite(ctx, combat.DivineSmiteCommand{
				Attacker:     attacker,
				Target:       target,
				SlotLevel:    res.SlotLevel,
				IsCritical:   result.CriticalHit,
				AttackResult: result,
			}, h.roller)
			if err == nil && dsResult.CombatLog != "" {
				h.postCombatLog(ctx, encounterID, dsResult.CombatLog)
			}
		})
	}

	if result.PromptBardicInspirationEligible {
		_ = h.classFeaturePrompts.PromptBardicInspiration(BardicInspirationPromptArgs{
			ChannelID:  channelID,
			HolderName: attacker.DisplayName,
			Die:        result.PromptBardicInspirationDie,
			Context:    "attack roll",
		}, func(res BardicInspirationPromptResult) {
			if res.Forfeited || !res.UseDie || h.classFeatureService == nil {
				return
			}
			biResult, err := h.classFeatureService.UseBardicInspiration(ctx, combat.UseBardicInspirationCommand{
				Combatant:     attacker,
				OriginalTotal: result.D20Roll.Total,
			}, h.roller)
			if err == nil && biResult.CombatLog != "" {
				h.postCombatLog(ctx, encounterID, biResult.CombatLog)
			}
		})
	}
}

// resolvePromptChannel picks a Discord channel to render post-hit class-
// feature prompts in. Priority: combat-log channel (so the encounter sees
// the follow-up), then the interaction's channel as a last resort. Mirrors
// CastHandler.resolvePromptChannel — kept package-private to each handler
// so changes can diverge without coordinated edits.
func (h *AttackHandler) resolvePromptChannel(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID) string {
	if h.channelIDProvider != nil {
		channels, err := h.channelIDProvider.GetChannelIDs(ctx, encounterID)
		if err == nil {
			if ch, ok := channels["combat-log"]; ok && ch != "" {
				return ch
			}
		}
	}
	return interaction.ChannelID
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
