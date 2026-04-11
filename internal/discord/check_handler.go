package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
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

// CheckEncounterProvider resolves the active encounter that a given Discord
// user is currently a combatant in. Phase 105: this replaces the previous
// guild-scoped lookup so /check and /save inside simultaneous encounters
// pick up conditions from the correct encounter.
type CheckEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
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
	characterLookup   CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	combatantLookup   CheckCombatantLookup
	rollLogger        dice.RollHistoryLogger
	notifier          dmqueue.Notifier
}

// SetNotifier wires the dm-queue Notifier so non-trivial /check rolls are
// gated through #dm-queue for DM narration. When nil (or unset), every
// /check responds immediately with the numeric result and no queue post.
func (h *CheckHandler) SetNotifier(n dmqueue.Notifier) { h.notifier = n }

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
		characterLookup:   characterLookup,
		encounterProvider: encounterProvider,
		combatantLookup:   combatantLookup,
		rollLogger:        rollLogger,
	}
}

// Handle processes the /check command interaction.
func (h *CheckHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)

	// Parse options
	skill, adv, disadv, dc, hasDC := h.parseOptions(data.Options)
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
	charData, err := parseCharacterData(char)
	if err != nil {
		respondEphemeral(h.session, interaction, "Error reading character data.")
		return
	}

	rollMode := rollModeFromFlags(adv, disadv)

	// Build input
	input := check.SingleCheckInput{
		Scores:           charData.Scores,
		Skill:            strings.ToLower(skill),
		ProficientSkills: charData.Skills,
		ExpertiseSkills:  charData.Expertise,
		JackOfAllTrades:  charData.JackOfAllTrades,
		ProfBonus:        int(char.ProficiencyBonus),
		RollMode:         rollMode,
	}

	// Apply condition effects if in combat
	if condInfo, ok := lookupCombatConditions(ctx, h.encounterProvider, h.combatantLookup, interaction.GuildID, userID, char.ID); ok {
		conds, _ := check.ParseConditions(condInfo.Conditions)
		input.Conditions = conds
		input.ExhaustionLevel = condInfo.ExhaustionLevel
	}

	result, err := h.checkService.SingleCheck(input)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Check failed: %v", err))
		return
	}

	// Phase 106d: gate non-trivial outcomes through #dm-queue. AutoFail
	// short-circuits to immediate ephemeral; otherwise apply the trivial
	// outcome rule and post to the queue when gated.
	if !result.AutoFail && h.shouldGate(result, dc, hasDC) {
		if h.postSkillCheckNarration(ctx, interaction, char, result) {
			respondEphemeral(h.session, interaction, "🎲 Check rolled — result sent to the DM for narration.")
			h.logRollIfWanted(char, result)
			return
		}
		// Fall through to immediate response if the queue post failed.
	}

	// Format and respond
	msg := check.FormatSingleCheckResult(char.Name, result)
	respondEphemeral(h.session, interaction, msg)

	h.logRollIfWanted(char, result)
}

// shouldGate decides whether to route the result through #dm-queue. The
// rule (Phase 106d): always gate EXCEPT when the natural d20 roll is 20 and
// the total meets/exceeds an explicit DC (trivial success), or when the
// natural d20 roll is 1 with an explicit DC (trivial failure).
func (h *CheckHandler) shouldGate(result check.SingleCheckResult, dc int, hasDC bool) bool {
	if h.notifier == nil {
		return false
	}
	if !hasDC {
		return true
	}
	natural := naturalD20(result.D20Result)
	if natural == 20 && result.Total >= dc {
		return false
	}
	if natural == 1 {
		return false
	}
	return true
}

// naturalD20 returns the chosen natural d20 face for the result, or 0 if
// the result has no d20 rolls (defensive).
func naturalD20(d20 dice.D20Result) int {
	if len(d20.Rolls) == 0 {
		return 0
	}
	return d20.Chosen
}

// postSkillCheckNarration posts a KindSkillCheckNarration event to the
// dm-queue carrying the channel, player, skill label, and total in
// ExtraMetadata so ResolveSkillCheckNarration can deliver a follow-up.
// Returns true on a successful post.
func (h *CheckHandler) postSkillCheckNarration(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, result check.SingleCheckResult) bool {
	skillLabel := titleSkill(result.Skill)
	summary := fmt.Sprintf("%s check (rolled %d)", skillLabel, result.Total)
	itemID, err := h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindSkillCheckNarration,
		PlayerName: char.Name,
		Summary:    summary,
		GuildID:    interaction.GuildID,
		ExtraMetadata: map[string]string{
			dmqueue.SkillCheckChannelIDKey:       interaction.ChannelID,
			dmqueue.SkillCheckPlayerDiscordIDKey: discordUserID(interaction),
			dmqueue.SkillCheckSkillLabelKey:      skillLabel,
			dmqueue.SkillCheckTotalKey:           strconv.Itoa(result.Total),
			dmqueue.SkillCheckCharNameKey:        char.Name,
		},
	})
	if err != nil {
		return false
	}
	// itemID may be "" if no #dm-queue is configured for this guild — treat
	// as "not gated" so the player still gets their result.
	return itemID != ""
}

// titleSkill returns the display label for a skill key (e.g. "perception"
// → "Perception"). Mirrors check.FormatSingleCheckResult's Title casing.
func titleSkill(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// logRollIfWanted persists the d20 roll to the player's roll history when
// a logger is wired and the result is not an auto-fail.
func (h *CheckHandler) logRollIfWanted(char refdata.Character, result check.SingleCheckResult) {
	if h.rollLogger == nil || result.AutoFail {
		return
	}
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

// parseOptions extracts skill, adv, disadv, dc, and hasDC from command options.
func (h *CheckHandler) parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) (skill string, adv, disadv bool, dc int, hasDC bool) {
	for _, opt := range opts {
		switch opt.Name {
		case "skill":
			skill = opt.StringValue()
		case "adv":
			adv = opt.BoolValue()
		case "disadv":
			disadv = opt.BoolValue()
		case "dc":
			dc = int(opt.IntValue())
			hasDC = true
		}
	}
	return
}

// characterData holds parsed character data needed for checks.
type characterData struct {
	Scores          character.AbilityScores
	Skills          []string
	Expertise       []string
	JackOfAllTrades bool
}

// parseCharacterData extracts ability scores and proficiency data from a character.
func parseCharacterData(char refdata.Character) (characterData, error) {
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return characterData{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	var profData struct {
		Skills          []string `json:"skills"`
		Expertise       []string `json:"expertise"`
		JackOfAllTrades bool     `json:"jack_of_all_trades"`
	}
	if char.Proficiencies.Valid {
		if err := json.Unmarshal(char.Proficiencies.RawMessage, &profData); err != nil {
			return characterData{}, fmt.Errorf("parsing proficiencies: %w", err)
		}
	}

	return characterData{
		Scores:          scores,
		Skills:          profData.Skills,
		Expertise:       profData.Expertise,
		JackOfAllTrades: profData.JackOfAllTrades,
	}, nil
}

// rollModeFromFlags converts advantage/disadvantage boolean flags to a dice.RollMode.
func rollModeFromFlags(adv, disadv bool) dice.RollMode {
	if adv && disadv {
		return dice.AdvantageAndDisadvantage
	}
	if adv {
		return dice.Advantage
	}
	if disadv {
		return dice.Disadvantage
	}
	return dice.Normal
}

// lookupCombatConditions checks if the character is in active combat and returns their conditions.
// Phase 105: routes via the invoker's combatant entry so conditions reflect the
// encounter the player actually belongs to, not some arbitrary active encounter.
func lookupCombatConditions(ctx context.Context, encounterProvider CheckEncounterProvider, combatantLookup CheckCombatantLookup, guildID, discordUserID string, charID uuid.UUID) (check.ConditionInfo, bool) {
	if encounterProvider == nil || combatantLookup == nil {
		return check.ConditionInfo{}, false
	}

	encounterID, err := encounterProvider.ActiveEncounterForUser(ctx, guildID, discordUserID)
	if err != nil {
		return check.ConditionInfo{}, false
	}

	combatants, err := combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
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
