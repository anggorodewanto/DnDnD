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
// TODO: Wire DM approval flow when the DM queue approval callback system is built.
// The dmQueueFunc field is reserved for posting rest requests to #dm-queue and
// waiting for DM approval before applying rest benefits.
type RestHandler struct {
	session           Session
	restService       *rest.Service
	campaignProvider  CheckCampaignProvider
	characterLookup  CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	charUpdater      RestCharacterUpdater
	rollLogger       dice.RollHistoryLogger
	dmQueueFunc      func(guildID string) string // reserved for future DM approval flow
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

func (h *RestHandler) handleShortRest(_ context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData) {
	// Build hit dice prompt with buttons
	var prompt strings.Builder
	prompt.WriteString("**Short Rest** — Select hit dice to spend:\n")
	for dieType, remaining := range charData.HitDiceRemaining {
		prompt.WriteString(fmt.Sprintf("> You have **%d** hit dice remaining (%s)\n", remaining, dieType))
	}
	prompt.WriteString("> Each hit die heals 1dX + CON modifier\n")

	components := BuildHitDiceButtons(char.ID, charData.HitDiceRemaining)

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    prompt.String(),
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: components,
		},
	})
}

// HandleHitDiceComponent processes a hit dice button click from the short rest prompt.
func (h *RestHandler) HandleHitDiceComponent(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, dieType, count, err := ParseHitDiceCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid hit dice data: %v", err))
		return
	}

	// Acknowledge the component interaction immediately
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		h.editInteraction(interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		h.editInteraction(interaction, "Could not find your character.")
		return
	}

	if char.ID != charID {
		h.editInteraction(interaction, "This hit dice prompt is not for your character.")
		return
	}

	charData, err := parseRestCharacterData(char)
	if err != nil {
		h.editInteraction(interaction, "Error reading character data.")
		return
	}

	spend := map[string]int{}
	if count > 0 {
		spend[dieType] = count
	}

	input := rest.ShortRestInput{
		HPCurrent:        int(char.HpCurrent),
		HPMax:            int(char.HpMax),
		CONModifier:      character.AbilityModifier(charData.Scores.CON),
		HitDiceRemaining: charData.HitDiceRemaining,
		HitDiceSpend:     spend,
		FeatureUses:      charData.FeatureUses,
		PactMagicSlots:   charData.PactMagicSlots,
		Classes:          charData.Classes,
	}

	result, err := h.restService.ShortRest(input)
	if err != nil {
		h.editInteraction(interaction, fmt.Sprintf("Rest failed: %v", err))
		return
	}

	// Persist all short rest changes in a single UpdateCharacter call
	h.persistShortRest(ctx, char, charData, result)

	msg := rest.FormatShortRestResult(char.Name, result)
	h.editInteraction(interaction, msg)

	h.logRestToHistory(char.Name, "Short Rest", msg)
}

func (h *RestHandler) editInteraction(interaction *discordgo.Interaction, content string) {
	empty := []discordgo.MessageComponent{}
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &empty,
	})
}

func (h *RestHandler) persistShortRest(ctx context.Context, char refdata.Character, charData restCharacterData, result rest.ShortRestResult) {
	if h.charUpdater == nil {
		return
	}

	hitDiceData, _ := json.Marshal(result.HitDiceRemaining)
	featureData, _ := json.Marshal(charData.FeatureUses)

	params := refdata.UpdateCharacterParams{
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
		HitDiceRemaining: hitDiceData,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureData, Valid: true},
		Features:         char.Features,
		Proficiencies:    char.Proficiencies,
		Gold:             char.Gold,
		AttunementSlots:  char.AttunementSlots,
		Languages:        char.Languages,
		Inventory:        char.Inventory,
		CharacterData:    char.CharacterData,
		DdbUrl:           char.DdbUrl,
		Homebrew:         char.Homebrew,
	}

	// Pact magic slots (mutated by service)
	if charData.PactMagicSlots != nil {
		pactData, err := json.Marshal(charData.PactMagicSlots)
		if err == nil {
			params.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactData, Valid: true}
		}
	} else {
		params.PactMagicSlots = char.PactMagicSlots
	}

	_, _ = h.charUpdater.UpdateCharacter(ctx, params)
}

// BuildHitDiceButtons creates Discord button components for hit dice selection.
// Single-class: one row with buttons [0] [1] ... [N].
// Multiclass: one row per die type with buttons [0] [1] ... [N].
func BuildHitDiceButtons(charID uuid.UUID, hitDiceRemaining map[string]int) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent

	dieTypes := sortedDieTypes(hitDiceRemaining)
	for _, dieType := range dieTypes {
		remaining := hitDiceRemaining[dieType]
		var buttons []discordgo.MessageComponent

		// Cap at 5 buttons per row (Discord limit)
		maxButtons := remaining
		if maxButtons > 4 {
			maxButtons = 4
		}

		for i := 0; i <= maxButtons; i++ {
			label := fmt.Sprintf("%d", i)
			if i == 0 {
				label = "Skip"
			}
			buttons = append(buttons, discordgo.Button{
				Label:    fmt.Sprintf("%s %s", dieType, label),
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("rest_hitdice:%s:%s:%d", charID.String(), dieType, i),
			})
		}

		components = append(components, &discordgo.ActionsRow{Components: buttons})
	}

	return components
}

// ParseHitDiceCustomID parses a custom ID like "rest_hitdice:<charID>:<dieType>:<count>".
func ParseHitDiceCustomID(customID string) (uuid.UUID, string, int, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 4 || parts[0] != "rest_hitdice" {
		return uuid.Nil, "", 0, fmt.Errorf("invalid hit dice custom ID: %s", customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", 0, fmt.Errorf("invalid character ID: %w", err)
	}
	dieType := parts[2]
	var count int
	if _, err := fmt.Sscanf(parts[3], "%d", &count); err != nil {
		return uuid.Nil, "", 0, fmt.Errorf("invalid count: %w", err)
	}
	return charID, dieType, count, nil
}

func sortedDieTypes(m map[string]int) []string {
	types := make([]string, 0, len(m))
	for k := range m {
		types = append(types, k)
	}
	sort.Strings(types)
	return types
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

func (h *RestHandler) persistLongRest(ctx context.Context, char refdata.Character, charData restCharacterData, result rest.LongRestResult) {
	if h.charUpdater == nil {
		return
	}

	// Marshal all updated values into a single UpdateCharacter call
	hitDiceData, _ := json.Marshal(result.HitDiceRemaining)
	featureData, _ := json.Marshal(charData.FeatureUses)
	spellSlotsData, _ := json.Marshal(result.SpellSlots)

	params := refdata.UpdateCharacterParams{
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
		SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlotsData, Valid: true},
		HitDiceRemaining: hitDiceData,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureData, Valid: true},
		Features:         char.Features,
		Proficiencies:    char.Proficiencies,
		Gold:             char.Gold,
		AttunementSlots:  char.AttunementSlots,
		Languages:        char.Languages,
		Inventory:        char.Inventory,
		CharacterData:    char.CharacterData,
		DdbUrl:           char.DdbUrl,
		Homebrew:         char.Homebrew,
	}

	// Marshal pact magic slots (mutated by service)
	if charData.PactMagicSlots != nil {
		pactData, err := json.Marshal(charData.PactMagicSlots)
		if err == nil {
			params.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactData, Valid: true}
		}
	} else {
		params.PactMagicSlots = char.PactMagicSlots
	}

	_, _ = h.charUpdater.UpdateCharacter(ctx, params)
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
