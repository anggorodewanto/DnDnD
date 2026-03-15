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
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/save"
)

// SaveHandler handles the /save slash command.
type SaveHandler struct {
	session           Session
	saveService       *save.Service
	campaignProvider  CheckCampaignProvider
	characterLookup   CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	combatantLookup   CheckCombatantLookup
	rollLogger        dice.RollHistoryLogger
}

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

	rollMode := dice.Normal
	if adv && disadv {
		rollMode = dice.AdvantageAndDisadvantage
	} else if adv {
		rollMode = dice.Advantage
	} else if disadv {
		rollMode = dice.Disadvantage
	}

	input := save.SaveInput{
		Scores:          charData.Scores,
		Ability:         strings.ToLower(ability),
		ProficientSaves: charData.Saves,
		ProfBonus:       int(char.ProficiencyBonus),
		RollMode:        rollMode,
	}

	// Apply condition effects if in combat
	if condInfo, ok := h.lookupCombatConditions(ctx, interaction.GuildID, char.ID); ok {
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

// lookupCombatConditions checks if the character is in active combat and returns their conditions.
func (h *SaveHandler) lookupCombatConditions(ctx context.Context, guildID string, charID uuid.UUID) (check.ConditionInfo, bool) {
	if h.encounterProvider == nil || h.combatantLookup == nil {
		return check.ConditionInfo{}, false
	}

	encounterID, err := h.encounterProvider.GetActiveEncounterID(ctx, guildID)
	if err != nil {
		return check.ConditionInfo{}, false
	}

	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return check.ConditionInfo{}, false
	}

	for _, c := range combatants {
		if !c.CharacterID.Valid || c.CharacterID.UUID != charID {
			continue
		}
		return check.ConditionInfo{
			Conditions:      c.Conditions,
			ExhaustionLevel: int(c.ExhaustionLevel),
		}, true
	}

	return check.ConditionInfo{}, false
}
