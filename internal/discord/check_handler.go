package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

var errNoEncounter = errors.New("no active encounter")

// CheckCharacterLookup resolves a Discord user to their character.
type CheckCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// CheckCampaignProvider provides the campaign for a guild.
type CheckCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// CheckEncounterProvider provides the active encounter for a guild.
type CheckEncounterProvider interface {
	GetActiveEncounterID(ctx context.Context, guildID string) (uuid.UUID, error)
}

// CheckCombatantLookup provides combatant data for an encounter.
type CheckCombatantLookup interface {
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
}

// CheckHandler handles the /check slash command.
type CheckHandler struct {
	session           Session
	checkService      *check.Service
	campaignProvider  CheckCampaignProvider
	characterLookup  CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	combatantLookup  CheckCombatantLookup
	rollLogger       dice.RollHistoryLogger
}

// NewCheckHandler creates a new CheckHandler.
func NewCheckHandler(
	session Session,
	roller *dice.Roller,
	campaignProvider CheckCampaignProvider,
	characterLookup CheckCharacterLookup,
	encounterProvider CheckEncounterProvider,
	combatantLookup CheckCombatantLookup,
	rollLogger dice.RollHistoryLogger,
) *CheckHandler {
	return &CheckHandler{
		session:           session,
		checkService:      check.NewService(roller),
		campaignProvider:  campaignProvider,
		characterLookup:  characterLookup,
		encounterProvider: encounterProvider,
		combatantLookup:  combatantLookup,
		rollLogger:       rollLogger,
	}
}

// Handle processes the /check command interaction.
func (h *CheckHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)

	// Parse options
	skill, adv, disadv := h.parseOptions(data.Options)
	if skill == "" {
		respondEphemeral(h.session, interaction, "Please specify a skill or ability (e.g. `/check perception`).")
		return
	}

	// Resolve campaign and character
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

	// Parse character data
	scores, profs, err := parseCharacterData(char)
	if err != nil {
		respondEphemeral(h.session, interaction, "Error reading character data.")
		return
	}

	// Determine roll mode from flags
	rollMode := dice.Normal
	if adv && disadv {
		rollMode = dice.AdvantageAndDisadvantage
	} else if adv {
		rollMode = dice.Advantage
	} else if disadv {
		rollMode = dice.Disadvantage
	}

	// Look up conditions from combat if active
	condInfo := h.lookupCombatConditions(ctx, interaction.GuildID, char.ID)

	// Build input
	input := check.SingleCheckInput{
		Scores:           scores,
		Skill:            strings.ToLower(skill),
		ProficientSkills: profs.Skills,
		ExpertiseSkills:  h.getExpertiseSkills(profs),
		JackOfAllTrades:  h.hasJackOfAllTrades(char),
		ProfBonus:        int(char.ProficiencyBonus),
		RollMode:         rollMode,
	}

	// Apply condition effects if in combat
	if len(condInfo) > 0 {
		conds, _ := check.ParseConditions(condInfo[0].Conditions)
		input.Conditions = conds
		input.ExhaustionLevel = condInfo[0].ExhaustionLevel
	}

	result, err := h.checkService.SingleCheck(input)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Check failed: %v", err))
		return
	}

	// Format and respond
	msg := check.FormatSingleCheckResult(char.Name, result)
	respondEphemeral(h.session, interaction, msg)

	// Log to roll history
	if h.rollLogger != nil && !result.AutoFail {
		_ = h.rollLogger.LogRoll(dice.RollLogEntry{
			DiceRolls:  []dice.GroupResult{{Die: 20, Count: 1, Results: result.D20Result.Rolls, Total: result.D20Result.Chosen}},
			Total:      result.Total,
			Expression: fmt.Sprintf("d20+%d", result.Modifier),
			Roller:     char.Name,
			Purpose:    fmt.Sprintf("%s check", result.Skill),
			Breakdown:  result.D20Result.Breakdown,
			Timestamp:  result.D20Result.Timestamp,
		})
	}
}

// parseOptions extracts skill, adv, disadv, and target from command options.
func (h *CheckHandler) parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) (skill string, adv, disadv bool) {
	for _, opt := range opts {
		switch opt.Name {
		case "skill":
			skill = opt.StringValue()
		case "adv":
			adv = opt.BoolValue()
		case "disadv":
			disadv = opt.BoolValue()
		}
	}
	return
}

// parseCharacterData extracts ability scores and proficiencies from a character.
func parseCharacterData(char refdata.Character) (character.AbilityScores, character.Proficiencies, error) {
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return scores, character.Proficiencies{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	var profs character.Proficiencies
	if char.Proficiencies.Valid {
		if err := json.Unmarshal(char.Proficiencies.RawMessage, &profs); err != nil {
			return scores, character.Proficiencies{}, fmt.Errorf("parsing proficiencies: %w", err)
		}
	}

	return scores, profs, nil
}

// lookupCombatConditions checks if the character is in active combat and returns their conditions.
func (h *CheckHandler) lookupCombatConditions(ctx context.Context, guildID string, charID uuid.UUID) []check.ConditionInfo {
	if h.encounterProvider == nil || h.combatantLookup == nil {
		return nil
	}

	encounterID, err := h.encounterProvider.GetActiveEncounterID(ctx, guildID)
	if err != nil {
		return nil
	}

	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil
	}

	for _, c := range combatants {
		if !c.CharacterID.Valid || c.CharacterID.UUID != charID {
			continue
		}
		return []check.ConditionInfo{{
			Conditions:      c.Conditions,
			ExhaustionLevel: int(c.ExhaustionLevel),
		}}
	}

	return nil
}

// getExpertiseSkills returns expertise skills. Currently reads from proficiencies.
// In 5e, expertise is typically stored separately; for now we check the same field.
func (h *CheckHandler) getExpertiseSkills(_ character.Proficiencies) []string {
	// TODO: wire expertise from character data when stored
	return nil
}

// hasJackOfAllTrades checks if the character has Jack of All Trades (Bard feature).
func (h *CheckHandler) hasJackOfAllTrades(_ refdata.Character) bool {
	// TODO: wire from character features when stored
	return false
}
