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

// CastCombatService is the combat-side surface /cast needs. *combat.Service
// satisfies it structurally. GetCasterConcentrationName returns the human-
// readable spell name (or "") that the caster is currently concentrating on
// so the service can detect concentration-replacement.
type CastCombatService interface {
	Cast(ctx context.Context, cmd combat.CastCommand, roller *dice.Roller) (combat.CastResult, error)
	CastAoE(ctx context.Context, cmd combat.AoECastCommand) (combat.AoECastResult, error)
	GetCasterConcentrationName(ctx context.Context, casterID uuid.UUID) (string, error)
}

// CastEncounterProvider is the lookup surface /cast needs. /cast also needs
// the spell catalog (GetSpell) and the map (GetMapByID) so it can decide
// AoE-vs-single-target dispatch and parse walls for cover calculations.
type CastEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	GetSpell(ctx context.Context, id string) (refdata.Spell, error)
	GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
}

// CastHandler handles the /cast slash command. Dispatches to either the
// single-target Cast service method or the AoE CastAoE method based on
// whether the resolved spell has area_of_effect data set.
//
// Out of scope (separate tasks): zone auto-creation (med-26), Silence
// rejection at cast time (med-25), Counterspell prompt (med-29), interactive
// metamagic prompts (med-30). Handler delegates straight to the existing
// service methods; bug fixes there are tracked separately.
type CastHandler struct {
	session           Session
	combatService     CastCombatService
	encounterProvider CastEncounterProvider
	roller            *dice.Roller
	channelIDProvider CampaignSettingsProvider
	turnGate          TurnGate
}

// NewCastHandler constructs a /cast handler.
func NewCastHandler(
	session Session,
	combatService CastCombatService,
	encounterProvider CastEncounterProvider,
	roller *dice.Roller,
) *CastHandler {
	return &CastHandler{
		session:           session,
		combatService:     combatService,
		encounterProvider: encounterProvider,
		roller:            roller,
	}
}

// SetChannelIDProvider wires the campaign settings provider for combat-log
// mirroring.
func (h *CastHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetTurnGate wires the Phase 27 turn-ownership gate. /cast costs an
// action (or bonus action) so the gate is invoked.
func (h *CastHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// Handle processes the /cast command interaction.
func (h *CastHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	spellID := strings.TrimSpace(optionString(interaction, "spell"))
	if spellID == "" {
		respondEphemeral(h.session, interaction, "Please specify a spell (e.g. `/cast spell:fire-bolt target:G2`).")
		return
	}
	targetStr := strings.TrimSpace(optionString(interaction, "target"))

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

	if !combat.IsExemptCommand("cast") && h.turnGate != nil {
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

	caster, err := h.encounterProvider.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return
	}

	spell, err := h.encounterProvider.GetSpell(ctx, spellID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Spell %q not found.", spellID))
		return
	}

	if spell.AreaOfEffect.Valid && len(spell.AreaOfEffect.RawMessage) > 0 {
		h.dispatchAoE(ctx, interaction, encounter, encounterID, caster, turn, spell, targetStr)
		return
	}

	h.dispatchSingleTarget(ctx, interaction, encounter, encounterID, caster, turn, spell, targetStr)
}

// dispatchSingleTarget runs the single-target /cast path: resolves the named
// target (when present), reads current concentration, calls Service.Cast,
// and posts the formatted log line.
func (h *CastHandler) dispatchSingleTarget(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	encounterID uuid.UUID,
	caster refdata.Combatant,
	turn refdata.Turn,
	spell refdata.Spell,
	targetStr string,
) {
	var targetID uuid.UUID
	if targetStr != "" {
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
		targetID = target.ID
	}

	currentConc, _ := h.combatService.GetCasterConcentrationName(ctx, caster.ID)

	cmd := combat.CastCommand{
		SpellID:              spell.ID,
		CasterID:             caster.ID,
		TargetID:             targetID,
		Turn:                 turn,
		CurrentConcentration: currentConc,
		SlotLevel:            optionInt(interaction, "level"),
		UseSpellSlot:         optionBool(interaction, "spell-slot"),
		IsRitual:             optionBool(interaction, "ritual"),
		EncounterStatus:      encounter.Status,
		EncounterID:          encounterID,
		Metamagic:            collectMetamagic(interaction),
	}

	result, err := h.combatService.Cast(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cast failed: %v", err))
		return
	}

	logLine := combat.FormatCastLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// dispatchAoE runs the AoE /cast path: parses the target coordinate, loads
// walls from the encounter map, calls Service.CastAoE, and posts the log.
func (h *CastHandler) dispatchAoE(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	encounterID uuid.UUID,
	caster refdata.Combatant,
	turn refdata.Turn,
	spell refdata.Spell,
	targetStr string,
) {
	if targetStr == "" {
		respondEphemeral(h.session, interaction, "AoE spells require a target coordinate (e.g. `target:G5`).")
		return
	}
	col, row, err := renderer.ParseCoordinate(targetStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid coordinate %q: %v", targetStr, err))
		return
	}

	walls := h.loadWalls(ctx, encounter)
	currentConc, _ := h.combatService.GetCasterConcentrationName(ctx, caster.ID)

	cmd := combat.AoECastCommand{
		SpellID:     spell.ID,
		CasterID:    caster.ID,
		EncounterID: encounterID,
		TargetCol:   indexToCol(col),
		// renderer.ParseCoordinate returns 0-based row; AoECastCommand expects
		// 1-based PositionRow convention so add 1 back.
		TargetRow:            int32(row + 1),
		Turn:                 turn,
		CurrentConcentration: currentConc,
		Walls:                walls,
		SlotLevel:            optionInt(interaction, "level"),
	}

	result, err := h.combatService.CastAoE(ctx, cmd)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cast failed: %v", err))
		return
	}

	logLine := combat.FormatAoECastLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// loadWalls best-effort loads wall segments from the encounter's map. If the
// encounter has no map, the parser fails, or the lookup errors, we return
// nil — cover calculation degrades to "no cover bonus" rather than failing
// the cast.
func (h *CastHandler) loadWalls(ctx context.Context, encounter refdata.Encounter) []renderer.WallSegment {
	if !encounter.MapID.Valid {
		return nil
	}
	mapData, err := h.encounterProvider.GetMapByID(ctx, encounter.MapID.UUID)
	if err != nil {
		return nil
	}
	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		return nil
	}
	return md.Walls
}

// postCombatLog mirrors a combat log line to #combat-log when wired.
func (h *CastHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}

// optionInt extracts a named integer option from an interaction's command
// data. Missing or non-int options return 0.
func optionInt(interaction *discordgo.Interaction, name string) int {
	data, ok := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if !ok {
		return 0
	}
	for _, opt := range data.Options {
		if opt.Name == name {
			return int(opt.IntValue())
		}
	}
	return 0
}

// metamagicFlags lists the boolean option names that map to metamagic
// option strings the combat service understands.
var metamagicFlags = []string{
	"subtle", "twin", "careful", "heightened", "distant",
	"quickened", "empowered", "extended",
}

// collectMetamagic walks the well-known boolean flags on the /cast
// interaction and returns the metamagic option list as the service expects.
func collectMetamagic(interaction *discordgo.Interaction) []string {
	var opts []string
	for _, name := range metamagicFlags {
		if optionBool(interaction, name) {
			opts = append(opts, normalizeMetamagicName(name))
		}
	}
	return opts
}

// normalizeMetamagicName remaps the discord option name to the service's
// canonical metamagic key (e.g. "twin" -> "twinned").
func normalizeMetamagicName(name string) string {
	if name == "twin" {
		return "twinned"
	}
	return name
}

// indexToCol converts a 0-based column index to its A-Z letter form. Used
// to translate `renderer.ParseCoordinate`'s 0-based output back into the
// service's letter-based PositionCol convention.
func indexToCol(idx int) string {
	if idx < 0 {
		return ""
	}
	return string(rune('A' + idx))
}
