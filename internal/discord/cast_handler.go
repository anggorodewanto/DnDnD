package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/inventory"
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

// CastInventoryAdapter is the per-character lookup + persistence surface used
// by the /cast identify and /cast detect-magic short-circuit paths. These
// spells operate on the caster's inventory rather than going through the
// combat pipeline, so they need direct character access. The adapter is
// optional: if unset, /cast identify and /cast detect-magic fall through to
// the regular pipeline (which will fail because they have no combat target).
type CastInventoryAdapter interface {
	GetCharacterByGuildAndDiscord(ctx context.Context, guildID, discordUserID string) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
	UpdateCharacterSpellSlots(ctx context.Context, charID uuid.UUID, slots pqtype.NullRawMessage) error
}

// CastNearbyInventoryScanner scans the caster's encounter for items on
// nearby combatants (and any dropped-loot store the scanner is wired
// against). Per the Detect Magic spell description (PHB) the spell reveals
// magical auras on items *near* the caster — within 30ft by default.
// F-88c: when wired, /cast detect-magic aggregates the caster's own
// inventory PLUS any items returned by ScanNearby; when unset the handler
// falls back to the historical caster-only behavior.
type CastNearbyInventoryScanner interface {
	ScanNearby(ctx context.Context, guildID, discordUserID string, radiusFt int) ([]NearbyInventory, error)
}

// NearbyInventory groups items found on a single nearby combatant or
// dropped-loot pool. SourceName is the display label rendered to the
// caster (e.g. "Goblin", "Sack on the floor"), Items is the subset already
// filtered to IsMagic = true so the scanner can also pre-filter when it
// wants to limit per-row.
type NearbyInventory struct {
	SourceName string
	Items      []character.InventoryItem
}

// CastHandler handles the /cast slash command. Dispatches to either the
// single-target Cast service method or the AoE CastAoE method based on
// whether the resolved spell has area_of_effect data set.
//
// /cast identify and /cast detect-magic short-circuit BEFORE the combat
// pipeline (these spells are inventory-side, not combat-side); see
// dispatchInventorySpell.
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
	inventoryAdapter  CastInventoryAdapter
	// F-88c: optional nearby-inventory scanner so /cast detect-magic
	// extends beyond the caster's own bag to environment items / dropped
	// loot / nearby PCs and NPCs within 30ft. Nil keeps the historical
	// caster-only behavior.
	nearbyScanner CastNearbyInventoryScanner
	// E-63: prompt store for the gold-fallback "Buy & Cast" / "Cancel"
	// confirmation. When unset, /cast still falls back to posting a plain
	// ephemeral message describing the missing component (no interactive
	// retry available).
	materialPrompts *ReactionPromptStore
	// SR-025: optional Empowered/Careful/Heightened interactive-prompt
	// poster. Built on top of the same ReactionPromptStore used by
	// materialPrompts so button clicks route back through one HandleComponent
	// fan-out. When unset, /cast still applies the metamagic effect using
	// the canonical defaults ("reroll the lowest dice", "no allies
	// protected", "disadvantage on first affected") so production never
	// degrades worse than the previous behaviour.
	metamagicPoster *MetamagicPromptPoster
}

// SetNearbyScanner wires the Detect Magic environmental scanner. When
// wired, /cast detect-magic aggregates the caster's own inventory with
// every nearby combatant's (and dropped-loot's) inventory within radius.
// Nil keeps the prior caster-only behavior. (F-88c)
func (h *CastHandler) SetNearbyScanner(s CastNearbyInventoryScanner) {
	h.nearbyScanner = s
}

// DetectMagicRadiusFt is the default radius of the Detect Magic aura scan,
// matching the PHB description ("nearby items within 30 feet"). Exposed
// for tests and so adapters can override per-spell if a future homebrew
// variant carries a different range. (F-88c)
const DetectMagicRadiusFt = 30

// SetInventoryAdapter wires the inventory-side adapter used by the
// /cast identify and /cast detect-magic short-circuit paths.
func (h *CastHandler) SetInventoryAdapter(a CastInventoryAdapter) {
	h.inventoryAdapter = a
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

// SetMaterialPromptStore wires the reaction-prompt store the handler uses
// to render the gold-fallback "Buy & Cast" / "Cancel" confirmation for
// spells whose costly material component is missing but affordable (E-63).
func (h *CastHandler) SetMaterialPromptStore(p *ReactionPromptStore) {
	h.materialPrompts = p
}

// HasMaterialPromptStore reports whether a non-nil prompt store has been
// wired. Production-wiring tests use this to detect the AOE-CAST follow-up
// regression (nil store → /cast falls back to plain ephemerals).
func (h *CastHandler) HasMaterialPromptStore() bool { return h.materialPrompts != nil }

// SetMetamagicPromptPoster wires the Empowered/Careful/Heightened prompt
// poster. When unset, /cast skips the interactive dice/target picker UI and
// proceeds with the canonical defaults (reroll lowest dice / no allies
// protected / disadvantage on first affected). SR-025.
func (h *CastHandler) SetMetamagicPromptPoster(p *MetamagicPromptPoster) {
	h.metamagicPoster = p
}

// HasMetamagicPromptPoster reports whether SR-025's interactive metamagic
// prompts are wired. Production-wiring tests check this to pin the
// regression where the poster was built but never invoked.
func (h *CastHandler) HasMetamagicPromptPoster() bool { return h.metamagicPoster != nil }

// HandleComponent dispatches button clicks owned by /cast (material-component
// prompt, etc.). Returns true when the click was claimed so the router can
// stop fan-out. Delegates to the prompt store's routing.
func (h *CastHandler) HandleComponent(interaction *discordgo.Interaction) bool {
	if h.materialPrompts == nil {
		return false
	}
	return h.materialPrompts.HandleComponent(interaction)
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

	// Inventory-side spell short-circuit: identify and detect-magic operate on
	// the caster's inventory rather than combatants, so they bypass the entire
	// encounter/turn pipeline. Falls through to the regular path if no
	// inventory adapter is wired.
	if h.inventoryAdapter != nil && (spellID == "identify" || spellID == "detect-magic") {
		h.dispatchInventorySpell(ctx, interaction, spellID, targetStr)
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
// and posts the formatted log line. SR-025: when --empowered is selected and
// a MetamagicPromptPoster is wired, an Empowered Spell die-picker is posted
// to the combat-log channel before Cast is invoked; the prompt is purely
// UX confirmation — the underlying reroll is server-side (see SR-025 note
// on RerollLowestDice).
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
	combatants, listErr := h.encounterProvider.ListCombatantsByEncounterID(ctx, encounterID)
	if listErr != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	var targetID uuid.UUID
	if targetStr != "" {
		target, err := combat.ResolveTarget(targetStr, combatants)
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", targetStr))
			return
		}
		targetID = target.ID
	}

	// SR-025: parse the optional twin-target Discord option so Twinned
	// Spell metamagic resolves a second target instead of silently leaving
	// CastCommand.TwinTargetID at uuid.Nil. Empty value = no second target.
	twinStr := strings.TrimSpace(optionString(interaction, "twin-target"))
	var twinTargetID uuid.UUID
	if twinStr != "" {
		twinTarget, err := combat.ResolveTarget(twinStr, combatants)
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Twin target %q not found.", twinStr))
			return
		}
		twinTargetID = twinTarget.ID
	}

	currentConc, _ := h.combatService.GetCasterConcentrationName(ctx, caster.ID)

	metamagic := collectMetamagic(interaction)
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
		Metamagic:            metamagic,
		TwinTargetID:         twinTargetID,
	}

	// SR-025: empowered metamagic posts an interactive die-picker prompt
	// before the cast resolves. The prompt is UX confirmation; the cast
	// proceeds either way (Cast applies the reroll server-side via
	// RerollLowestDice on the canonical "worst" choice).
	if hasMetamagicFlag(metamagic, "empowered") && h.metamagicPoster != nil {
		channelID := h.resolvePromptChannel(ctx, interaction, encounterID)
		if channelID != "" {
			h.postEmpoweredPromptThenRun(ctx, interaction, encounterID, channelID, spell.Name, cmd)
			return
		}
	}

	h.runSingleTargetCast(ctx, interaction, encounterID, cmd)
}

// runSingleTargetCast is the post-prompt continuation of dispatchSingleTarget:
// invokes Cast and renders the result. Extracted so the SR-025 Empowered
// prompt can resume the flow on click/forfeit without duplicating the
// material-prompt fallback logic.
func (h *CastHandler) runSingleTargetCast(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, cmd combat.CastCommand) {
	result, err := h.combatService.Cast(ctx, cmd, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cast failed: %v", err))
		return
	}

	// E-63: when the service returns NeedsGoldConfirmation (component missing
	// but the caster can afford it), surface a Buy & Cast / Cancel button
	// instead of treating the response as a successful cast. The slot has NOT
	// been deducted yet (Cast returns early before slot deduction in the
	// NeedsGoldConfirmation branch).
	if result.MaterialComponent != nil && result.MaterialComponent.NeedsGoldConfirmation {
		h.promptMaterialFallback(ctx, interaction, encounterID, cmd, result.MaterialComponent)
		return
	}

	logLine := combat.FormatCastLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)
}

// postEmpoweredPromptThenRun posts the Empowered Spell die-picker prompt to
// the combat-log channel and runs the cast once the player clicks (or the
// forfeit timer fires). The dice values shown are a small placeholder
// sequence — the real reroll happens server-side via RerollLowestDice once
// damage is rolled, so the prompt is informational. SR-025.
func (h *CastHandler) postEmpoweredPromptThenRun(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, channelID, spellName string, cmd combat.CastCommand) {
	// Display 4 placeholder dice ("1d6" rolls) so the player has visible
	// buttons; the chosen button index is informational only — the cast
	// still rerolls the lowest dice server-side per SR-025.
	dice := []int{1, 1, 1, 1}
	args := EmpoweredPromptArgs{
		ChannelID:  channelID,
		SpellName:  spellName,
		DiceRolls:  dice,
		MaxRerolls: 1,
	}
	err := h.metamagicPoster.PromptEmpowered(args, func(EmpoweredPromptResult) {
		h.runSingleTargetCast(ctx, interaction, encounterID, cmd)
	})
	if err != nil {
		h.runSingleTargetCast(ctx, interaction, encounterID, cmd)
		return
	}
	respondEphemeral(h.session, interaction, "✨ Empowered Spell: pick a die to reroll in the combat channel.")
}

// promptMaterialFallback renders the gold-fallback Buy & Cast / Cancel
// prompt and routes the click back to the combat service. The original
// ephemeral response is replaced by the prompt content so the caster sees
// the confirmation in their command thread.
func (h *CastHandler) promptMaterialFallback(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	originalCmd combat.CastCommand,
	info *combat.CastMaterialComponentInfo,
) {
	promptMsg := combat.FormatGoldFallbackPrompt(combat.MaterialComponentResult{
		ComponentName: info.ComponentName,
		CostGp:        info.CostGp,
	})

	// Best-effort fallback when no prompt store is wired: surface the message
	// as a plain ephemeral so the caster at least learns the spell didn't go
	// through. Without buttons there's no interactive retry path.
	if h.materialPrompts == nil {
		respondEphemeral(h.session, interaction, promptMsg)
		return
	}

	// We need a channel to post the prompt to. The combat-log mirror channel
	// is reused so the prompt lives in the encounter feed; if no channel is
	// available we fall back to the ephemeral path.
	channelID := h.resolvePromptChannel(ctx, interaction, encounterID)
	if channelID == "" {
		respondEphemeral(h.session, interaction, promptMsg)
		return
	}

	buttons := []ReactionPromptButton{
		{Label: "Buy & Cast", Choice: "buy", Style: discordgo.PrimaryButton},
		{Label: "Cancel", Choice: "cancel", Style: discordgo.SecondaryButton},
	}
	retryCmd := originalCmd
	retryCmd.GoldFallback = true
	_, postErr := h.materialPrompts.Post(ReactionPromptPostArgs{
		ChannelID: channelID,
		Content:   "✨ " + promptMsg,
		Buttons:   buttons,
		OnChoice: func(c context.Context, _ *discordgo.Interaction, choice string) {
			if choice != "buy" {
				_, _ = h.session.ChannelMessageSend(channelID, fmt.Sprintf("❌ Cast of %s cancelled — no gold spent, slot retained.", originalCmd.SpellID))
				return
			}
			retryResult, retryErr := h.combatService.Cast(c, retryCmd, h.roller)
			if retryErr != nil {
				_, _ = h.session.ChannelMessageSend(channelID, fmt.Sprintf("Cast failed: %v", retryErr))
				return
			}
			retryLog := combat.FormatCastLog(retryResult)
			h.postCombatLog(c, encounterID, retryLog)
			_, _ = h.session.ChannelMessageSend(channelID, retryLog)
		},
		OnForfeit: func(c context.Context) {
			_, _ = h.session.ChannelMessageSend(channelID, fmt.Sprintf("⏳ Cast of %s timed out — slot retained, no gold spent.", originalCmd.SpellID))
		},
	})
	if postErr != nil {
		respondEphemeral(h.session, interaction, promptMsg)
		return
	}
	respondEphemeral(h.session, interaction, "Confirm the material-component purchase in the combat channel.")
}

// resolvePromptChannel picks a Discord channel to render the gold-fallback
// prompt in. Priority: combat-log channel (so the encounter sees the
// follow-up), then the interaction's channel as a last resort. Returns ""
// when no channel can be resolved; callers should fall back to ephemeral
// messaging in that case.
func (h *CastHandler) resolvePromptChannel(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID) string {
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

// dispatchAoE runs the AoE /cast path: parses the target coordinate, loads
// walls from the encounter map, calls Service.CastAoE, and posts the log.
// SR-025: when --careful / --heightened / --empowered are selected and a
// MetamagicPromptPoster is wired, the corresponding interactive prompt is
// posted before CastAoE — the cast resumes once the player clicks (or the
// forfeit timer fires). Careful threads the chosen combatant ID into
// AoECastCommand.CarefulTargetIDs; Heightened into HeightenedTargetID;
// Empowered is UX-only (the reroll fires server-side during damage
// resolution; see SR-025 RerollLowestDice).
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

	metamagic := collectMetamagic(interaction)
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
		Metamagic:            metamagic,
	}

	// SR-025: pre-cast metamagic prompts. The cast is held until the first
	// interactive option resolves (click or forfeit); subsequent options
	// chain via the OnChoice callback so each picker resolves independently
	// without losing the others. When the poster is unwired or no AoE
	// metamagic flag is set, we fall straight through to runAoECast.
	if h.metamagicPoster != nil && needsAoEMetamagicPrompt(metamagic) {
		channelID := h.resolvePromptChannel(ctx, interaction, encounterID)
		if channelID != "" {
			h.postAoEMetamagicPrompts(ctx, interaction, encounterID, channelID, spell.Name, cmd, metamagic)
			return
		}
	}

	h.runAoECast(ctx, interaction, encounterID, cmd)
}

// needsAoEMetamagicPrompt reports whether the AoE cast needs one of the
// SR-025 interactive prompts before CastAoE runs.
func needsAoEMetamagicPrompt(metamagic []string) bool {
	return hasMetamagicFlag(metamagic, "careful") ||
		hasMetamagicFlag(metamagic, "heightened") ||
		hasMetamagicFlag(metamagic, "empowered")
}

// postAoEMetamagicPrompts chains the Careful → Heightened → Empowered
// prompts. Each step picks an affected combatant (or die index for
// empowered) and either modifies `cmd` or no-ops, then dispatches the next
// prompt. When the chain runs out of remaining flags, runAoECast fires.
func (h *CastHandler) postAoEMetamagicPrompts(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, channelID, spellName string, cmd combat.AoECastCommand, metamagic []string) {
	// Pull the affected non-caster combatants once so prompt buttons render
	// stable names. SR-025 keeps this simple: the AoE shape calculation is
	// done service-side; the prompts list every non-caster combatant in the
	// encounter so the player can pick from the same pool the service will
	// hit. False-positives (combatant not actually in AoE) just no-op
	// server-side.
	allCombs, _ := h.encounterProvider.ListCombatantsByEncounterID(ctx, encounterID)
	candidates := make([]refdata.Combatant, 0, len(allCombs))
	for _, c := range allCombs {
		if c.ID != cmd.CasterID {
			candidates = append(candidates, c)
		}
	}
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, c.DisplayName)
	}

	// Chain helper: each prompt invokes the next via the closure. The
	// closures capture cmd by value and pass the mutated copy forward.
	var runCareful, runHeightened, runEmpowered, finalize func(combat.AoECastCommand)

	finalize = func(finalCmd combat.AoECastCommand) {
		h.runAoECast(ctx, interaction, encounterID, finalCmd)
	}
	runEmpowered = func(updated combat.AoECastCommand) {
		if !hasMetamagicFlag(metamagic, "empowered") || len(names) == 0 {
			finalize(updated)
			return
		}
		// Empowered prompt is UX-only at cast time (damage isn't rolled
		// until /save resolves). We still post it so the player sees the
		// metamagic fire and can confirm — the actual reroll lands at
		// damage time via RerollLowestDice.
		err := h.metamagicPoster.PromptEmpowered(EmpoweredPromptArgs{
			ChannelID:  channelID,
			SpellName:  spellName,
			DiceRolls:  []int{1, 1, 1, 1},
			MaxRerolls: 1,
		}, func(EmpoweredPromptResult) {
			finalize(updated)
		})
		if err != nil {
			finalize(updated)
		}
	}
	runHeightened = func(updated combat.AoECastCommand) {
		if !hasMetamagicFlag(metamagic, "heightened") || len(names) == 0 {
			runEmpowered(updated)
			return
		}
		err := h.metamagicPoster.PromptHeightened(HeightenedPromptArgs{
			ChannelID:   channelID,
			SpellName:   spellName,
			TargetNames: names,
		}, func(res HeightenedPromptResult) {
			if !res.Forfeited && res.SelectedIndex >= 0 && res.SelectedIndex < len(candidates) {
				updated.HeightenedTargetID = candidates[res.SelectedIndex].ID
			}
			runEmpowered(updated)
		})
		if err != nil {
			runEmpowered(updated)
		}
	}
	runCareful = func(updated combat.AoECastCommand) {
		if !hasMetamagicFlag(metamagic, "careful") || len(names) == 0 {
			runHeightened(updated)
			return
		}
		err := h.metamagicPoster.PromptCareful(CarefulPromptArgs{
			ChannelID:    channelID,
			SpellName:    spellName,
			TargetNames:  names,
			MaxProtected: 1, // SR-025 minimal: single pick. Multi-pick is a follow-up.
		}, func(res CarefulPromptResult) {
			if !res.Forfeited && res.SelectedIndex >= 0 && res.SelectedIndex < len(candidates) {
				updated.CarefulTargetIDs = append(updated.CarefulTargetIDs, candidates[res.SelectedIndex].ID)
			}
			runHeightened(updated)
		})
		if err != nil {
			runHeightened(updated)
		}
	}

	runCareful(cmd)
	respondEphemeral(h.session, interaction, "✨ Metamagic prompt(s) posted to the combat channel.")
}

// runAoECast is the post-prompt continuation of dispatchAoE: invokes CastAoE
// and renders the result + per-player /save pings. Extracted so the SR-025
// metamagic prompt chain can resume the flow on click/forfeit.
func (h *CastHandler) runAoECast(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, cmd combat.AoECastCommand) {
	result, err := h.combatService.CastAoE(ctx, cmd)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cast failed: %v", err))
		return
	}

	logLine := combat.FormatAoECastLog(result)
	h.postCombatLog(ctx, encounterID, logLine)
	respondEphemeral(h.session, interaction, logLine)

	// E-59: ping affected player combatants in the combat-log channel
	// asking them to roll /save <ability>. Each ping names the combatant so
	// the right player knows which save to roll. NPC saves are handled by
	// the DM dashboard; the resolver fires damage once all pending rows
	// (player + DM) are resolved.
	combs, _ := h.encounterProvider.ListCombatantsByEncounterID(ctx, encounterID)
	h.postAoESavePrompts(ctx, encounterID, result, combs)
}

// postAoESavePrompts emits one /save reminder per affected player combatant
// when the AoE spell calls for a save. Best-effort: nothing is posted when
// the result has no save ability or the combat-log channel is not wired.
func (h *CastHandler) postAoESavePrompts(ctx context.Context, encounterID uuid.UUID, result combat.AoECastResult, allCombatants []refdata.Combatant) {
	if result.SaveAbility == "" || len(result.PendingSaves) == 0 {
		return
	}
	combatantByID := make(map[uuid.UUID]refdata.Combatant, len(allCombatants))
	for _, c := range allCombatants {
		combatantByID[c.ID] = c
	}
	for _, ps := range result.PendingSaves {
		if ps.IsNPC {
			continue
		}
		c, ok := combatantByID[ps.CombatantID]
		name := ps.CombatantID.String()
		if ok {
			name = c.DisplayName
		}
		msg := fmt.Sprintf("⚠️ %s — roll `/save %s` (DC %d) vs **%s**.",
			name, ps.SaveAbility, ps.DC, result.SpellName)
		h.postCombatLog(ctx, encounterID, msg)
	}
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

// hasMetamagicFlag reports whether the normalized metamagic slice contains
// the given option. SR-025.
func hasMetamagicFlag(metamagic []string, option string) bool {
	for _, m := range metamagic {
		if m == option {
			return true
		}
	}
	return false
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

// dispatchInventorySpell handles the /cast identify and /cast detect-magic
// short-circuit. Both spells operate on the caster's inventory rather than
// engaging the combat pipeline.
func (h *CastHandler) dispatchInventorySpell(ctx context.Context, interaction *discordgo.Interaction, spellID, targetStr string) {
	userID := discordUserID(interaction)
	char, err := h.inventoryAdapter.GetCharacterByGuildAndDiscord(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read inventory. Please contact the DM.")
		return
	}

	if spellID == "detect-magic" {
		h.dispatchDetectMagic(ctx, interaction, char, items)
		return
	}
	h.dispatchIdentify(ctx, interaction, char, items, targetStr)
}

// dispatchDetectMagic lists the magic items in the caster's inventory plus
// (when a nearby scanner is wired) any items on nearby combatants / dropped
// loot within the spell's radius. Detect Magic is a ritual; we do not
// consume a spell slot here. (F-88c)
func (h *CastHandler) dispatchDetectMagic(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, items []character.InventoryItem) {
	selfMagic := inventory.DetectMagicItems(items)

	nearby := h.collectNearbyMagic(ctx, interaction)

	if len(selfMagic) == 0 && len(nearby) == 0 {
		respondEphemeral(h.session, interaction, "✨ Detect Magic: you sense no magical auras nearby.")
		return
	}

	var b strings.Builder
	b.WriteString("✨ **Detect Magic** — you sense the following magical auras:\n")
	if len(selfMagic) > 0 {
		fmt.Fprintf(&b, "> __On %s (you):__\n", char.Name)
		for _, it := range selfMagic {
			fmt.Fprintf(&b, "> • %s", it.Name)
			if it.Rarity != "" {
				fmt.Fprintf(&b, " [%s]", it.Rarity)
			}
			b.WriteString("\n")
		}
	}
	for _, group := range nearby {
		fmt.Fprintf(&b, "> __Nearby — %s:__\n", group.SourceName)
		for _, it := range group.Items {
			fmt.Fprintf(&b, "> • %s", it.Name)
			if it.Rarity != "" {
				fmt.Fprintf(&b, " [%s]", it.Rarity)
			}
			b.WriteString("\n")
		}
	}
	respondEphemeral(h.session, interaction, b.String())
}

// collectNearbyMagic queries the wired scanner (if any) and returns only
// groups that surface at least one magic item. Best-effort: scanner errors
// degrade silently to "no nearby auras" rather than blocking the cast.
func (h *CastHandler) collectNearbyMagic(ctx context.Context, interaction *discordgo.Interaction) []NearbyInventory {
	if h.nearbyScanner == nil {
		return nil
	}
	groups, err := h.nearbyScanner.ScanNearby(ctx, interaction.GuildID, discordUserID(interaction), DetectMagicRadiusFt)
	if err != nil {
		return nil
	}
	out := make([]NearbyInventory, 0, len(groups))
	for _, g := range groups {
		magic := inventory.DetectMagicItems(g.Items)
		if len(magic) == 0 {
			continue
		}
		out = append(out, NearbyInventory{SourceName: g.SourceName, Items: magic})
	}
	return out
}

// dispatchIdentify casts the Identify spell on a target item. Requires the
// caster to know the spell and have an available 1st-level slot (or invoke
// it as a ritual via `/cast spell:identify target:<item> ritual:true`).
func (h *CastHandler) dispatchIdentify(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, items []character.InventoryItem, targetStr string) {
	if targetStr == "" {
		respondEphemeral(h.session, interaction, "Identify requires a target item (e.g. `/cast spell:identify target:mystery-ring`).")
		return
	}

	knowsIdentify := characterKnowsSpell(char, "identify")
	slots := parseSpellSlotsForCast(char)
	slotLevel := optionInt(interaction, "level")
	if slotLevel == 0 {
		slotLevel = 1
	}
	isRitual := optionBool(interaction, "ritual")

	result, err := inventory.CastIdentify(inventory.CastIdentifyInput{
		Items:      items,
		ItemID:     targetStr,
		KnowsSpell: knowsIdentify,
		SpellSlots: slots,
		SlotLevel:  slotLevel,
		IsRitual:   isRitual,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot cast Identify: %v", err))
		return
	}

	if err := h.persistIdentify(ctx, char, result, slotLevel, isRitual); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save identify result. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// persistIdentify writes the updated inventory and (when not a ritual) the
// reduced spell-slot count.
func (h *CastHandler) persistIdentify(
	ctx context.Context,
	char refdata.Character,
	result inventory.CastIdentifyResult,
	slotLevel int,
	isRitual bool,
) error {
	invJSON, err := character.MarshalInventory(result.UpdatedItems)
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}
	if _, err := h.inventoryAdapter.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		return fmt.Errorf("update inventory: %w", err)
	}
	if isRitual {
		return nil
	}
	updatedSlots := decrementSlot(parseSpellSlotsRaw(char), slotLevel)
	slotJSON, err := json.Marshal(updatedSlots)
	if err != nil {
		return fmt.Errorf("marshal slots: %w", err)
	}
	return h.inventoryAdapter.UpdateCharacterSpellSlots(ctx, char.ID, pqtype.NullRawMessage{RawMessage: slotJSON, Valid: true})
}

// characterKnowsSpell inspects character_data.spells_known and
// character_data.spells_prepared to determine whether the caster knows the
// named spell. Returns false when character_data is missing or unreadable so
// the inventory.CastIdentify call surfaces the canonical error.
func characterKnowsSpell(char refdata.Character, spellID string) bool {
	if !char.CharacterData.Valid {
		return false
	}
	var data map[string]any
	if err := json.Unmarshal(char.CharacterData.RawMessage, &data); err != nil {
		return false
	}
	for _, key := range []string{"spells_known", "spells_prepared", "spells"} {
		if matchesSpellList(data[key], spellID) {
			return true
		}
	}
	return false
}

// matchesSpellList reports whether a JSON-decoded slice of spell ids/objects
// contains the named spell. Accepts a slice of strings or a slice of objects
// each with an "id" or "spell_id" key.
func matchesSpellList(raw any, spellID string) bool {
	list, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, entry := range list {
		switch v := entry.(type) {
		case string:
			if v == spellID {
				return true
			}
		case map[string]any:
			if id, ok := v["id"].(string); ok && id == spellID {
				return true
			}
			if id, ok := v["spell_id"].(string); ok && id == spellID {
				return true
			}
		}
	}
	return false
}

// parseSpellSlotsForCast returns the caster's slot pool keyed by integer level
// (1, 2, 3, ...) so it can be handed to inventory.CastIdentify.
func parseSpellSlotsForCast(char refdata.Character) map[int]int {
	out := map[int]int{}
	for level, slot := range parseSpellSlotsRaw(char) {
		lvl := 0
		if _, err := fmt.Sscanf(level, "%d", &lvl); err != nil || lvl == 0 {
			continue
		}
		out[lvl] = slot.Current
	}
	return out
}

// parseSpellSlotsRaw decodes char.SpellSlots into the canonical map form.
func parseSpellSlotsRaw(char refdata.Character) map[string]character.SlotInfo {
	out := map[string]character.SlotInfo{}
	if !char.SpellSlots.Valid || len(char.SpellSlots.RawMessage) == 0 {
		return out
	}
	_ = json.Unmarshal(char.SpellSlots.RawMessage, &out)
	return out
}

// decrementSlot returns a new copy of slots with one slot consumed at the
// given level. Slot levels in the JSON are stored as decimal strings ("1").
// Floors at 0.
func decrementSlot(slots map[string]character.SlotInfo, level int) map[string]character.SlotInfo {
	out := make(map[string]character.SlotInfo, len(slots))
	keys := make([]string, 0, len(slots))
	for k := range slots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	key := fmt.Sprintf("%d", level)
	for _, k := range keys {
		v := slots[k]
		if k == key && v.Current > 0 {
			v.Current--
		}
		out[k] = v
	}
	return out
}
