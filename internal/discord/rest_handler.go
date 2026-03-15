package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// RestCharacterUpdater persists character updates after a rest.
type RestCharacterUpdater interface {
	UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error)
	UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error)
	UpdateCharacterPactMagicSlots(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error)
	UpdateCharacter(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error)
}

// RestHandler handles the /rest slash command.
type RestHandler struct {
	session           Session
	restService       *rest.Service
	campaignProvider  CheckCampaignProvider
	characterLookup  CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	charUpdater      RestCharacterUpdater
	rollLogger       dice.RollHistoryLogger
	dmQueueFunc      func(guildID string) string
}

// NewRestHandler creates a new RestHandler.
func NewRestHandler(
	session Session,
	roller *dice.Roller,
	campaignProvider CheckCampaignProvider,
	characterLookup CheckCharacterLookup,
	encounterProvider CheckEncounterProvider,
	charUpdater RestCharacterUpdater,
	rollLogger dice.RollHistoryLogger,
	dmQueueFunc func(guildID string) string,
) *RestHandler {
	return &RestHandler{
		session:           session,
		restService:       rest.NewService(roller),
		campaignProvider:  campaignProvider,
		characterLookup:  characterLookup,
		encounterProvider: encounterProvider,
		charUpdater:      charUpdater,
		rollLogger:       rollLogger,
		dmQueueFunc:      dmQueueFunc,
	}
}

// Handle processes the /rest command interaction.
func (h *RestHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	restType := h.parseRestType(data.Options)

	if restType != "short" && restType != "long" {
		respondEphemeral(h.session, interaction, "Invalid rest type. Use `/rest short` or `/rest long`.")
		return
	}

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	// Check for active combat
	if h.encounterProvider != nil {
		if _, err := h.encounterProvider.GetActiveEncounterID(ctx, interaction.GuildID); err == nil {
			respondEphemeral(h.session, interaction, "You cannot rest during active combat.")
			return
		}
	}

	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	charData, err := parseRestCharacterData(char)
	if err != nil {
		respondEphemeral(h.session, interaction, "Error reading character data.")
		return
	}

	switch restType {
	case "short":
		h.handleShortRest(ctx, interaction, char, charData)
	case "long":
		h.handleLongRest(ctx, interaction, char, charData)
	}
}

func (h *RestHandler) handleShortRest(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData) {
	// For Phase 83a: auto-approve, spend 0 hit dice (hit dice spending will be interactive in future)
	// The service still recharges features and pact slots
	input := rest.ShortRestInput{
		HPCurrent:        int(char.HpCurrent),
		HPMax:            int(char.HpMax),
		CONModifier:      character.AbilityModifier(charData.Scores.CON),
		HitDiceRemaining: charData.HitDiceRemaining,
		HitDiceSpend:     map[string]int{}, // no dice spent in auto-mode
		FeatureUses:      charData.FeatureUses,
		PactMagicSlots:   charData.PactMagicSlots,
		Classes:          charData.Classes,
	}

	result, err := h.restService.ShortRest(input)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Rest failed: %v", err))
		return
	}

	// Persist feature uses
	if len(result.FeaturesRecharged) > 0 {
		h.persistFeatureUses(ctx, char.ID, charData.FeatureUses)
	}

	// Persist pact magic slots
	if result.PactSlotsRestored && charData.PactMagicSlots != nil {
		h.persistPactMagicSlots(ctx, char.ID, charData.PactMagicSlots)
	}

	msg := rest.FormatShortRestResult(char.Name, result)
	respondEphemeral(h.session, interaction, msg)

	h.logRestToHistory(char.Name, "Short Rest", msg)
}

func (h *RestHandler) handleLongRest(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData) {
	input := rest.LongRestInput{
		HPCurrent:        int(char.HpCurrent),
		HPMax:            int(char.HpMax),
		HitDiceRemaining: charData.HitDiceRemaining,
		Classes:          charData.Classes,
		FeatureUses:      charData.FeatureUses,
		SpellSlots:       charData.SpellSlots,
		PactMagicSlots:   charData.PactMagicSlots,
	}

	result := h.restService.LongRest(input)

	// Persist all changes
	h.persistLongRest(ctx, char, charData, result)

	msg := rest.FormatLongRestResult(char.Name, result)
	respondEphemeral(h.session, interaction, msg)

	h.logRestToHistory(char.Name, "Long Rest", msg)
}

func (h *RestHandler) persistFeatureUses(ctx context.Context, charID uuid.UUID, featureUses map[string]character.FeatureUse) {
	if h.charUpdater == nil {
		return
	}
	data, err := json.Marshal(featureUses)
	if err != nil {
		return
	}
	_, _ = h.charUpdater.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          charID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: data, Valid: true},
	})
}

func (h *RestHandler) persistPactMagicSlots(ctx context.Context, charID uuid.UUID, pact *character.PactMagicSlots) {
	if h.charUpdater == nil {
		return
	}
	data, err := json.Marshal(pact)
	if err != nil {
		return
	}
	_, _ = h.charUpdater.UpdateCharacterPactMagicSlots(ctx, refdata.UpdateCharacterPactMagicSlotsParams{
		ID:             charID,
		PactMagicSlots: pqtype.NullRawMessage{RawMessage: data, Valid: true},
	})
}

func (h *RestHandler) persistLongRest(ctx context.Context, char refdata.Character, charData restCharacterData, result rest.LongRestResult) {
	if h.charUpdater == nil {
		return
	}

	// Update feature uses
	h.persistFeatureUses(ctx, char.ID, charData.FeatureUses)

	// Update spell slots
	if len(result.SpellSlots) > 0 {
		data, err := json.Marshal(result.SpellSlots)
		if err == nil {
			_, _ = h.charUpdater.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
				ID:         char.ID,
				SpellSlots: pqtype.NullRawMessage{RawMessage: data, Valid: true},
			})
		}
	}

	// Update pact magic slots
	if result.PactSlotsRestored && charData.PactMagicSlots != nil {
		h.persistPactMagicSlots(ctx, char.ID, charData.PactMagicSlots)
	}

	// Update HP and hit dice remaining
	hitDiceData, _ := json.Marshal(result.HitDiceRemaining)
	_, _ = h.charUpdater.UpdateCharacter(ctx, refdata.UpdateCharacterParams{
		ID:               char.ID,
		Name:             char.Name,
		Race:             char.Race,
		Classes:          char.Classes,
		Level:            char.Level,
		AbilityScores:    char.AbilityScores,
		HpMax:            char.HpMax,
		HpCurrent:        int32(result.HPAfter),
		TempHp:           char.TempHp,
		Ac:               char.Ac,
		AcFormula:        char.AcFormula,
		SpeedFt:          char.SpeedFt,
		ProficiencyBonus: char.ProficiencyBonus,
		EquippedMainHand: char.EquippedMainHand,
		EquippedOffHand:  char.EquippedOffHand,
		EquippedArmor:    char.EquippedArmor,
		SpellSlots:       char.SpellSlots,
		PactMagicSlots:   char.PactMagicSlots,
		HitDiceRemaining: hitDiceData,
		FeatureUses:      char.FeatureUses,
		Features:         char.Features,
		Proficiencies:    char.Proficiencies,
		Gold:             char.Gold,
		AttunementSlots:  char.AttunementSlots,
		Languages:        char.Languages,
		Inventory:        char.Inventory,
		CharacterData:    char.CharacterData,
		DdbUrl:           char.DdbUrl,
		Homebrew:         char.Homebrew,
	})
}

func (h *RestHandler) logRestToHistory(charName, restType, msg string) {
	if h.rollLogger == nil {
		return
	}
	_ = h.rollLogger.LogRoll(dice.RollLogEntry{
		Roller:  charName,
		Purpose: restType,
	})
}

// parseRestType extracts the rest type from command options.
func (h *RestHandler) parseRestType(opts []*discordgo.ApplicationCommandInteractionDataOption) string {
	for _, opt := range opts {
		if opt.Name == "type" {
			return strings.ToLower(opt.StringValue())
		}
	}
	return ""
}

// restCharacterData holds parsed character data needed for rests.
type restCharacterData struct {
	Scores           character.AbilityScores
	Classes          []character.ClassEntry
	HitDiceRemaining map[string]int
	FeatureUses      map[string]character.FeatureUse
	SpellSlots       map[string]character.SlotInfo
	PactMagicSlots   *character.PactMagicSlots
}

// parseRestCharacterData extracts all fields needed for rest processing.
func parseRestCharacterData(char refdata.Character) (restCharacterData, error) {
	var data restCharacterData

	if err := json.Unmarshal(char.AbilityScores, &data.Scores); err != nil {
		return data, fmt.Errorf("parsing ability scores: %w", err)
	}

	if err := json.Unmarshal(char.Classes, &data.Classes); err != nil {
		return data, fmt.Errorf("parsing classes: %w", err)
	}

	data.HitDiceRemaining = make(map[string]int)
	if len(char.HitDiceRemaining) > 0 {
		if err := json.Unmarshal(char.HitDiceRemaining, &data.HitDiceRemaining); err != nil {
			return data, fmt.Errorf("parsing hit dice: %w", err)
		}
	}

	data.FeatureUses = make(map[string]character.FeatureUse)
	if char.FeatureUses.Valid {
		if err := json.Unmarshal(char.FeatureUses.RawMessage, &data.FeatureUses); err != nil {
			return data, fmt.Errorf("parsing feature uses: %w", err)
		}
	}

	data.SpellSlots = make(map[string]character.SlotInfo)
	if char.SpellSlots.Valid {
		if err := json.Unmarshal(char.SpellSlots.RawMessage, &data.SpellSlots); err != nil {
			return data, fmt.Errorf("parsing spell slots: %w", err)
		}
	}

	if char.PactMagicSlots.Valid {
		var pact character.PactMagicSlots
		if err := json.Unmarshal(char.PactMagicSlots.RawMessage, &pact); err != nil {
			return data, fmt.Errorf("parsing pact magic slots: %w", err)
		}
		if pact.Max > 0 {
			data.PactMagicSlots = &pact
		}
	}

	return data, nil
}
