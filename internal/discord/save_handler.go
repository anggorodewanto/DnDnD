package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/save"
)

// AoESaveResolver is the slice of combat.Service the /save handler needs to
// resolve player-rolled AoE saves and (when all saves on a spell are in)
// dispatch damage application. Wired by main; tests inject a mock. (E-59)
//
// RecordAoEPendingSaveRoll: marks one pending AoE save row as resolved.
// Returns the row's spell ID (for the next call) and whether anything was
// resolved.
//
// ResolveAoEPendingSavesForSpell: when called after a successful record, it
// triggers ResolveAoESaves on the spell if every row on this spell is now
// rolled / forfeited.
type AoESaveResolver interface {
	RecordAoEPendingSaveRoll(ctx context.Context, combatantID uuid.UUID, ability string, total int, autoFail bool) (string, bool, error)
	ResolveAoEPendingSavesForSpell(ctx context.Context, encounterID uuid.UUID, spellID string) error
}

// AoESaveServiceAdapter wraps a CastCombatService-grade combat service +
// roller so it satisfies AoESaveResolver. Production wiring builds this in
// cmd/dndnd; tests bypass it via the mockAoESaveResolver double.
type AoESaveServiceAdapter struct {
	svc interface {
		RecordAoEPendingSaveRoll(ctx context.Context, combatantID uuid.UUID, ability string, total int, autoFail bool) (string, bool, error)
		ResolveAoEPendingSaves(ctx context.Context, encounterID uuid.UUID, spellID string, roller *dice.Roller) (*combat.AoEDamageResult, error)
	}
	roller *dice.Roller
}

// NewAoESaveServiceAdapter constructs the adapter used by cmd/dndnd to wire
// /save into the combat service's AoE pending-save resolution path.
func NewAoESaveServiceAdapter(svc interface {
	RecordAoEPendingSaveRoll(ctx context.Context, combatantID uuid.UUID, ability string, total int, autoFail bool) (string, bool, error)
	ResolveAoEPendingSaves(ctx context.Context, encounterID uuid.UUID, spellID string, roller *dice.Roller) (*combat.AoEDamageResult, error)
}, roller *dice.Roller) *AoESaveServiceAdapter {
	return &AoESaveServiceAdapter{svc: svc, roller: roller}
}

// RecordAoEPendingSaveRoll passes through to the wrapped service.
func (a *AoESaveServiceAdapter) RecordAoEPendingSaveRoll(ctx context.Context, combatantID uuid.UUID, ability string, total int, autoFail bool) (string, bool, error) {
	return a.svc.RecordAoEPendingSaveRoll(ctx, combatantID, ability, total, autoFail)
}

// ResolveAoEPendingSavesForSpell injects the adapter's roller and forwards.
func (a *AoESaveServiceAdapter) ResolveAoEPendingSavesForSpell(ctx context.Context, encounterID uuid.UUID, spellID string) error {
	_, err := a.svc.ResolveAoEPendingSaves(ctx, encounterID, spellID, a.roller)
	return err
}

// SaveHandler handles the /save slash command.
type SaveHandler struct {
	session           Session
	saveService       *save.Service
	campaignProvider  CheckCampaignProvider
	characterLookup   CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	combatantLookup   CheckCombatantLookup
	rollLogger        dice.RollHistoryLogger
	aoeSaveResolver   AoESaveResolver // E-59: optional; nil disables AoE pending-save resolution
}

// SetAoESaveResolver wires the combat-side resolver for AoE pending saves.
// When unset, /save behaves exactly as before (rolls and logs, but does not
// touch pending_saves rows). (E-59)
func (h *SaveHandler) SetAoESaveResolver(r AoESaveResolver) {
	h.aoeSaveResolver = r
}

// HasAoESaveResolver reports whether a non-nil AoESaveResolver has been
// wired. Production-wiring tests use this to detect the AOE-CAST follow-up
// regression (nil resolver → AoE saves are recorded but never resolved).
func (h *SaveHandler) HasAoESaveResolver() bool { return h.aoeSaveResolver != nil }

// HasRollLogger reports whether a non-nil dice.RollHistoryLogger has been
// wired on this handler. Used by production-wiring tests to detect the
// Phase 18 silent-no-op.
func (h *SaveHandler) HasRollLogger() bool { return h.rollLogger != nil }

// NewSaveHandler creates a new SaveHandler.
func NewSaveHandler(
	session Session,
	roller *dice.Roller,
	campaignProvider CheckCampaignProvider,
	characterLookup CheckCharacterLookup,
	encounterProvider CheckEncounterProvider,
	combatantLookup CheckCombatantLookup,
	rollLogger dice.RollHistoryLogger,
) *SaveHandler {
	return &SaveHandler{
		session:           session,
		saveService:       save.NewService(roller),
		campaignProvider:  campaignProvider,
		characterLookup:   characterLookup,
		encounterProvider: encounterProvider,
		combatantLookup:   combatantLookup,
		rollLogger:        rollLogger,
	}
}

// Handle processes the /save command interaction.
func (h *SaveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)

	ability, adv, disadv := h.parseOptions(data.Options)
	if ability == "" {
		respondEphemeral(h.session, interaction, "Please specify an ability (e.g. `/save dex`).")
		return
	}

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	charData, err := parseSaveCharacterData(char)
	if err != nil {
		respondEphemeral(h.session, interaction, "Error reading character data.")
		return
	}

	rollMode := rollModeFromFlags(adv, disadv)

	input := save.SaveInput{
		Scores:          charData.Scores,
		Ability:         strings.ToLower(ability),
		ProficientSaves: charData.Saves,
		ProfBonus:       int(char.ProficiencyBonus),
		RollMode:        rollMode,
	}

	// med-33 / Phase 82: populate FeatureEffects + EffectCtx so Aura of
	// Protection, Bless, magic-item save bonuses, etc. are layered onto
	// the result instead of silently dropped (mirrors the FES population
	// pattern in attack.go populateAttackFES). char.Classes / char.Features
	// drive BuildFeatureDefinitions; an unmarshal error degrades to no
	// feature effects rather than failing the whole roll.
	input.FeatureEffects = buildSaveFeatureEffects(char)
	input.EffectCtx = combat.EffectContext{
		AbilityUsed: strings.ToLower(ability),
		WearingArmor: char.EquippedArmor.Valid && char.EquippedArmor.String != "",
	}

	// Apply condition effects if in combat
	if condInfo, ok := lookupCombatConditions(ctx, h.encounterProvider, h.combatantLookup, interaction.GuildID, userID, char.ID); ok {
		conds, _ := check.ParseConditions(condInfo.Conditions)
		input.Conditions = conds
		input.ExhaustionLevel = condInfo.ExhaustionLevel
	}

	result, err := h.saveService.Save(input)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Save failed: %v", err))
		return
	}

	msg := save.FormatSaveResult(char.Name, result)
	respondEphemeral(h.session, interaction, msg)

	// Log to roll history
	if h.rollLogger != nil && !result.AutoFail {
		_ = h.rollLogger.LogRoll(dice.RollLogEntry{
			DiceRolls:  []dice.GroupResult{{Die: 20, Count: 1, Results: result.D20Result.Rolls, Total: result.D20Result.Chosen}},
			Total:      result.Total,
			Expression: fmt.Sprintf("d20+%d", result.Modifier+result.FeatureBonus),
			Roller:     char.Name,
			Purpose:    fmt.Sprintf("%s save", strings.ToUpper(result.Ability)),
			Breakdown:  result.D20Result.Breakdown,
			Timestamp:  result.D20Result.Timestamp,
		})
	}

	// E-59: resolve any AoE pending_saves row matching this player's
	// combatant + ability. When this was the last outstanding save for the
	// spell, the resolver fires damage application via ResolveAoESaves.
	h.maybeResolveAoESave(ctx, interaction, char, ability, result)
}

// maybeResolveAoESave looks up the rolling combatant's encounter and asks
// the AoE save resolver to consume one pending row. When the resolver
// reports that the row was the spell's last outstanding save it then drives
// the damage-application hook for that spell. Best-effort: any wiring gap
// (no resolver, no encounter, no matching combatant) is a no-op so the
// surrounding /save behaviour is unaffected. (E-59)
func (h *SaveHandler) maybeResolveAoESave(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, ability string, result save.SaveResult) {
	if h.aoeSaveResolver == nil || h.encounterProvider == nil || h.combatantLookup == nil {
		return
	}
	userID := discordUserID(interaction)
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		return
	}
	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return
	}
	var combatantID uuid.UUID
	for _, c := range combatants {
		if c.CharacterID.Valid && c.CharacterID.UUID == char.ID {
			combatantID = c.ID
			break
		}
	}
	if combatantID == uuid.Nil {
		return
	}
	// Pass the rolled total + autoFail flag; the resolver does the
	// canonical (total >= row.Dc) comparison using the stored DC so we
	// don't need to plumb DC into /save.
	spellID, resolved, err := h.aoeSaveResolver.RecordAoEPendingSaveRoll(ctx, combatantID, strings.ToLower(ability), result.Total, result.AutoFail)
	if err != nil || !resolved {
		return
	}
	_ = h.aoeSaveResolver.ResolveAoEPendingSavesForSpell(ctx, encounterID, spellID)
}

// parseOptions extracts ability, adv, disadv from command options.
func (h *SaveHandler) parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) (ability string, adv, disadv bool) {
	for _, opt := range opts {
		switch opt.Name {
		case "ability":
			ability = opt.StringValue()
		case "adv":
			adv = opt.BoolValue()
		case "disadv":
			disadv = opt.BoolValue()
		}
	}
	return
}

// saveCharacterData holds parsed character data needed for saves.
type saveCharacterData struct {
	Scores character.AbilityScores
	Saves  []string
}

// buildSaveFeatureEffects collects FES feature definitions from the
// character's classes + features columns (the same pair that drives attack
// feature effects). Unmarshal errors degrade to a nil slice — better to
// drop a feature bonus than to fail the whole save roll. (med-33)
func buildSaveFeatureEffects(char refdata.Character) []combat.FeatureDefinition {
	var classes []combat.CharacterClass
	if len(char.Classes) > 0 {
		_ = json.Unmarshal(char.Classes, &classes)
	}
	var feats []combat.CharacterFeature
	if char.Features.Valid && len(char.Features.RawMessage) > 0 {
		_ = json.Unmarshal(char.Features.RawMessage, &feats)
	}
	if len(classes) == 0 && len(feats) == 0 {
		return nil
	}
	return combat.BuildFeatureDefinitions(classes, feats)
}

// parseSaveCharacterData extracts ability scores and save proficiencies from a character.
func parseSaveCharacterData(char refdata.Character) (saveCharacterData, error) {
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return saveCharacterData{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	var profData struct {
		Saves []string `json:"saves"`
	}
	if char.Proficiencies.Valid {
		if err := json.Unmarshal(char.Proficiencies.RawMessage, &profData); err != nil {
			return saveCharacterData{}, fmt.Errorf("parsing proficiencies: %w", err)
		}
	}

	return saveCharacterData{
		Scores: scores,
		Saves:  profData.Saves,
	}, nil
}
