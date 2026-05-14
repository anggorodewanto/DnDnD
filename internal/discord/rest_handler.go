package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

const hitDicePrefix = "rest_hitdice"

// RestCharacterUpdater persists character updates after a rest.
type RestCharacterUpdater interface {
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
	characterLookup   CheckCharacterLookup
	encounterProvider CheckEncounterProvider
	charUpdater       RestCharacterUpdater
	rollLogger        dice.RollHistoryLogger
	dmQueueFunc       func(guildID string) string // reserved for future DM approval flow
	notifier          dmqueue.Notifier
	cardUpdater       CardUpdater // SR-007
}

// SetCardUpdater wires the SR-007 character-card refresh callback fired
// after every successful rest write (short / long / partial).
func (h *RestHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// SetNotifier wires the dm-queue Notifier. When set, /rest posts a rest
// request notification to #dm-queue before running the rest flow.
func (h *RestHandler) SetNotifier(n dmqueue.Notifier) { h.notifier = n }

// SetPublisher forwards the encounter publisher + lookup down into the
// underlying rest.Service so /rest publishes a fresh dashboard snapshot
// after HP / hit-dice / spell-slot writes (H-104b). A nil publisher is
// tolerated and disables the fan-out (legacy / headless deploys keep
// working). The same pattern is used by inventory and levelup.
func (h *RestHandler) SetPublisher(p rest.EncounterPublisher, lookup rest.EncounterLookup) {
	if h == nil || h.restService == nil {
		return
	}
	h.restService.SetPublisher(p, lookup)
}

func (h *RestHandler) SetCombatantExhaustionStore(store rest.CombatantExhaustionStore) {
	if h == nil || h.restService == nil {
		return
	}
	h.restService.SetCombatantExhaustionStore(store)
}

// postRestRequestToDMQueue posts a rest request notification via the Notifier.
// No-op when no Notifier is wired.
// SR-002: CampaignID is required by PgStore.Insert; without it the row would
// fail to persist after the Discord message is sent.
func (h *RestHandler) postRestRequestToDMQueue(ctx context.Context, guildID, campaignID, charName, restType string) {
	if h.notifier == nil {
		return
	}
	_, _ = h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindRestRequest,
		PlayerName: charName,
		Summary:    fmt.Sprintf("requests a %s rest", restType),
		GuildID:    guildID,
		CampaignID: campaignID,
	})
}

// HasRollLogger reports whether a non-nil dice.RollHistoryLogger has been
// wired on this handler. Used by production-wiring tests to detect the
// Phase 18 silent-no-op.
func (h *RestHandler) HasRollLogger() bool { return h.rollLogger != nil }

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
		characterLookup:   characterLookup,
		encounterProvider: encounterProvider,
		charUpdater:       charUpdater,
		rollLogger:        rollLogger,
		dmQueueFunc:       dmQueueFunc,
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

	userID := discordUserID(interaction)

	// Check for active combat — Phase 105 routes via combatant membership
	// so a player blocked from resting is specifically one who is still a
	// combatant in an active encounter, not merely in a guild that has one.
	if h.encounterProvider != nil {
		if _, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, userID); err == nil {
			respondEphemeral(h.session, interaction, "You cannot rest during active combat.")
			return
		}
	}
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

	// Notify DM of the rest request via #dm-queue (no-op if no notifier wired).
	h.postRestRequestToDMQueue(ctx, interaction.GuildID, campaign.ID.String(), char.Name, restType)

	// med-34 / Phase 83a: gate behind the auto_approve_rest campaign
	// setting. When the DM has explicitly turned off auto-approval, we
	// only post the request to #dm-queue (above) and tell the player to
	// wait — the rest applies once the DM resolves the queue entry.
	if !restAutoApproved(campaign) {
		respondEphemeral(h.session, interaction, fmt.Sprintf(
			"⏳ %s rest request sent to the DM. Your rest will apply once they approve it.",
			strings.Title(restType),
		))
		return
	}

	switch restType {
	case "short":
		h.handleShortRest(ctx, interaction, char, charData)
	case "long":
		h.handleLongRest(ctx, interaction, char, charData)
	}
}

// restAutoApproved decodes the campaign's auto_approve_rest setting,
// defaulting to true (the historical behaviour) when the column is null
// or the field is absent. (med-34)
func restAutoApproved(c refdata.Campaign) bool {
	if !c.Settings.Valid {
		return true
	}
	var s campaign.Settings
	if err := json.Unmarshal(c.Settings.RawMessage, &s); err != nil {
		return true
	}
	return s.AutoApproveRestEnabled()
}

func (h *RestHandler) handleShortRest(_ context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData) {
	prompt := buildHitDicePrompt(charData.HitDiceRemaining)
	components := BuildHitDiceButtons(char.ID, charData.HitDiceRemaining)

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    prompt,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: components,
		},
	})
}

func buildHitDicePrompt(hitDiceRemaining map[string]int) string {
	var prompt strings.Builder
	prompt.WriteString("**Short Rest** — Select hit dice to spend:\n")
	for _, dieType := range slices.Sorted(maps.Keys(hitDiceRemaining)) {
		remaining := hitDiceRemaining[dieType]
		fmt.Fprintf(&prompt, "> You have **%d** hit dice remaining (%s)\n", remaining, dieType)
	}
	prompt.WriteString("> Each hit die heals 1dX + CON modifier\n")
	return prompt.String()
}

// HandleHitDiceComponent processes a hit dice button click from the short rest prompt.
// For multiclass characters, this supports multi-step spending: each click spends dice
// of one type, then if other die types remain, updated buttons are shown. The "Done"
// button or single-class flow finalizes the rest immediately.
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

	// "Done" button finalizes without spending more dice
	if dieType == "done" {
		h.finalizeShortRest(ctx, interaction, char, charData, map[string]int{})
		return
	}

	spend := map[string]int{}
	if count > 0 {
		spend[dieType] = count
	}

	// For multiclass: if there are other die types with remaining dice, show
	// updated buttons after spending this die type (multi-step flow).
	remainingAfter := maps.Clone(charData.HitDiceRemaining)
	if count > 0 {
		remainingAfter[dieType] -= count
	}

	// Check if other die types still have dice to spend
	otherTypesRemain := false
	for k, v := range remainingAfter {
		if k != dieType && v > 0 {
			otherTypesRemain = true
			break
		}
	}

	if otherTypesRemain && count > 0 {
		// Multi-step: apply this die type's spending, then show buttons for remaining types
		h.finalizeShortRestPartial(ctx, interaction, char, charData, spend, remainingAfter)
		return
	}

	// Single-class or final step: finalize the rest
	h.finalizeShortRest(ctx, interaction, char, charData, spend)
}

func buildShortRestInput(char refdata.Character, charData restCharacterData, spend map[string]int) rest.ShortRestInput {
	return rest.ShortRestInput{
		HPCurrent:        int(char.HpCurrent),
		HPMax:            int(char.HpMax),
		CONModifier:      character.AbilityModifier(charData.Scores.CON),
		HitDiceRemaining: charData.HitDiceRemaining,
		HitDiceSpend:     spend,
		FeatureUses:      charData.FeatureUses,
		PactMagicSlots:   charData.PactMagicSlots,
		Classes:          charData.Classes,
	}
}

// finalizeShortRest applies the full short rest with the given hit dice spend.
func (h *RestHandler) finalizeShortRest(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData, spend map[string]int) {
	result, err := h.restService.ShortRest(buildShortRestInput(char, charData, spend))
	if err != nil {
		h.editInteraction(interaction, fmt.Sprintf("Rest failed: %v", err))
		return
	}

	h.persistRestChanges(ctx, char, charData, int32(result.HPAfter), result.HitDiceRemaining, nil)

	// H-104b: fan out a dashboard snapshot if the character is also in
	// a sibling active encounter. Silent no-op when not in combat or
	// no publisher is wired.
	h.restService.PublishForCharacter(ctx, char.ID)

	// SR-007: refresh #character-cards after the rest write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	msg := rest.FormatShortRestResult(char.Name, result)
	h.editInteraction(interaction, msg)

	h.logRestToHistory(char.Name, "Short Rest", msg)
}

// finalizeShortRestPartial applies one die type's spending and shows buttons for remaining types.
func (h *RestHandler) finalizeShortRestPartial(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData, spend map[string]int, remainingAfter map[string]int) {
	result, err := h.restService.ShortRest(buildShortRestInput(char, charData, spend))
	if err != nil {
		h.editInteraction(interaction, fmt.Sprintf("Rest failed: %v", err))
		return
	}

	// Persist the partial changes (HP + hit dice so far)
	h.persistRestChanges(ctx, char, charData, int32(result.HPAfter), result.HitDiceRemaining, nil)

	// H-104b: fan out a dashboard snapshot (mid-step partial). The
	// per-step publish is harmless and keeps the dashboard in sync as
	// the multi-die-type flow walks through its remaining steps.
	h.restService.PublishForCharacter(ctx, char.ID)

	// SR-007: refresh #character-cards after the partial rest write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	// Build updated prompt showing remaining dice for other types + Done button
	otherDice := make(map[string]int)
	for k, v := range remainingAfter {
		if v > 0 {
			otherDice[k] = v
		}
	}

	var prompt strings.Builder
	prompt.WriteString("**Short Rest** — Hit dice spent so far:\n")
	for _, roll := range result.HitDieRolls {
		fmt.Fprintf(&prompt, "> • %s: rolled %d + %d CON = %d HP\n", roll.DieType, roll.Rolled, roll.CONMod, roll.Healed)
	}
	fmt.Fprintf(&prompt, "> HP: %d/%d\n", result.HPAfter, result.HPMax)
	prompt.WriteString("> Spend more hit dice or click Done:\n")

	components := BuildHitDiceButtons(char.ID, otherDice)
	// Add Done button
	components = append(components, &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Done",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("%s:%s:done:0", hitDicePrefix, char.ID.String()),
			},
		},
	})

	content := prompt.String()
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &components,
	})
}

func (h *RestHandler) editInteraction(interaction *discordgo.Interaction, content string) {
	empty := []discordgo.MessageComponent{}
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &empty,
	})
}

// persistRestChanges persists HP, hit dice, feature uses, pact slots, and optionally spell slots.
// spellSlots is nil for short rests (unchanged) and non-nil for long rests (restored).
func (h *RestHandler) persistRestChanges(ctx context.Context, char refdata.Character, charData restCharacterData, hpCurrent int32, hitDiceRemaining map[string]int, spellSlots map[string]character.SlotInfo) {
	if h.charUpdater == nil {
		return
	}

	hitDiceData, err := json.Marshal(hitDiceRemaining)
	if err != nil {
		return
	}
	featureData, err := json.Marshal(charData.FeatureUses)
	if err != nil {
		return
	}

	params := baseUpdateParams(char)
	params.HpCurrent = hpCurrent
	params.HitDiceRemaining = hitDiceData
	params.FeatureUses = pqtype.NullRawMessage{RawMessage: featureData, Valid: true}

	// Spell slots: use new values if provided, else preserve existing
	if spellSlots != nil {
		slotData, err := json.Marshal(spellSlots)
		if err == nil {
			params.SpellSlots = pqtype.NullRawMessage{RawMessage: slotData, Valid: true}
		}
	}

	// Pact magic slots (mutated by service)
	if charData.PactMagicSlots != nil {
		pactData, err := json.Marshal(charData.PactMagicSlots)
		if err == nil {
			params.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactData, Valid: true}
		}
	}

	_, _ = h.charUpdater.UpdateCharacter(ctx, params)
}

func (h *RestHandler) persistLongRestChanges(ctx context.Context, char refdata.Character, charData restCharacterData, result rest.LongRestResult) {
	if h.charUpdater == nil {
		return
	}

	hitDiceData, err := json.Marshal(result.HitDiceRemaining)
	if err != nil {
		return
	}
	featureData, err := json.Marshal(charData.FeatureUses)
	if err != nil {
		return
	}

	params := baseUpdateParams(char)
	params.HpCurrent = int32(result.HPAfter)
	params.TempHp = 0 // SR-053: long rest clears temp HP
	params.HitDiceRemaining = hitDiceData
	params.FeatureUses = pqtype.NullRawMessage{RawMessage: featureData, Valid: true}
	if slotData, err := json.Marshal(result.SpellSlots); err == nil {
		params.SpellSlots = pqtype.NullRawMessage{RawMessage: slotData, Valid: true}
	}
	if charData.PactMagicSlots != nil {
		if pactData, err := json.Marshal(charData.PactMagicSlots); err == nil {
			params.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactData, Valid: true}
		}
	}
	params.CharacterData = pqtype.NullRawMessage{
		RawMessage: rest.CharacterDataWithExhaustion(char.CharacterData.RawMessage, result.ExhaustionLevelAfter),
		Valid:      true,
	}

	_, _ = h.charUpdater.UpdateCharacter(ctx, params)
}

// baseUpdateParams creates UpdateCharacterParams with all fields copied from the character.
// Callers override the fields they want to change.
func baseUpdateParams(char refdata.Character) refdata.UpdateCharacterParams {
	return refdata.UpdateCharacterParams{
		ID:               char.ID,
		Name:             char.Name,
		Race:             char.Race,
		Classes:          char.Classes,
		Level:            char.Level,
		AbilityScores:    char.AbilityScores,
		HpMax:            char.HpMax,
		HpCurrent:        char.HpCurrent,
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
		HitDiceRemaining: char.HitDiceRemaining,
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
	}
}

// BuildHitDiceButtons creates Discord button components for hit dice selection.
// Single-class: one row with buttons [0] [1] ... [N].
// Multiclass: one row per die type with buttons [0] [1] ... [N].
func BuildHitDiceButtons(charID uuid.UUID, hitDiceRemaining map[string]int) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent

	dieTypes := slices.Sorted(maps.Keys(hitDiceRemaining))
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
				CustomID: fmt.Sprintf("%s:%s:%s:%d", hitDicePrefix, charID.String(), dieType, i),
			})
		}

		components = append(components, &discordgo.ActionsRow{Components: buttons})
	}

	return components
}

// ParseHitDiceCustomID parses a custom ID like "rest_hitdice:<charID>:<dieType>:<count>".
func ParseHitDiceCustomID(customID string) (uuid.UUID, string, int, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 4 || parts[0] != hitDicePrefix {
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

func (h *RestHandler) handleLongRest(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, charData restCharacterData) {
	exhaustionLevel := charData.ExhaustionLevel
	if !charData.ExhaustionLevelSet {
		exhaustionLevel = h.restService.ExhaustionLevelForCharacter(ctx, char.ID)
	}
	input := rest.LongRestInput{
		HPCurrent:        int(char.HpCurrent),
		HPMax:            int(char.HpMax),
		TempHP:           int(char.TempHp),
		HitDiceRemaining: charData.HitDiceRemaining,
		Classes:          charData.Classes,
		FeatureUses:      charData.FeatureUses,
		SpellSlots:       charData.SpellSlots,
		PactMagicSlots:   charData.PactMagicSlots,
		ExhaustionLevel:  exhaustionLevel,
	}

	result := h.restService.LongRest(input)

	// Persist all changes
	h.persistLongRest(ctx, char, charData, result)
	h.restService.PersistLongRestExhaustion(ctx, char.ID, result)

	// H-104b: refresh the dashboard snapshot for a sibling encounter the
	// character may still be a combatant in. PublishForCharacter is a
	// silent no-op when not in combat or when no publisher is wired.
	h.restService.PublishForCharacter(ctx, char.ID)

	// SR-007: refresh #character-cards after the long-rest write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	msg := rest.FormatLongRestResult(char.Name, result)

	// E-65 / Phase 65: invoke the canonical combat.LongRestPrepareReminder
	// helper so prepared casters (cleric / druid / paladin) get the
	// "/prepare" hint. The format string already embeds the same text
	// inline when PreparedCasterReminder is true; this explicit invocation
	// ensures the wiring goes through the canonical helper and stays in
	// sync if the helper's copy ever changes.
	if reminder := combat.LongRestPrepareReminder(longRestPrepareClasses(charData.Classes)); reminder != "" && !strings.Contains(msg, reminder) {
		msg = msg + "\n" + reminder
	}

	respondEphemeral(h.session, interaction, msg)

	h.logRestToHistory(char.Name, "Long Rest", msg)
}

// longRestPrepareClasses adapts the rest handler's character.ClassEntry
// slice into the combat package's CharacterClass shape so
// combat.LongRestPrepareReminder can be invoked.
func longRestPrepareClasses(classes []character.ClassEntry) []combat.CharacterClass {
	out := make([]combat.CharacterClass, 0, len(classes))
	for _, c := range classes {
		out = append(out, combat.CharacterClass{Class: c.Class, Level: c.Level})
	}
	return out
}

func (h *RestHandler) persistLongRest(ctx context.Context, char refdata.Character, charData restCharacterData, result rest.LongRestResult) {
	h.persistLongRestChanges(ctx, char, charData, result)
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
	Scores             character.AbilityScores
	Classes            []character.ClassEntry
	HitDiceRemaining   map[string]int
	FeatureUses        map[string]character.FeatureUse
	SpellSlots         map[string]character.SlotInfo
	PactMagicSlots     *character.PactMagicSlots
	ExhaustionLevel    int
	ExhaustionLevelSet bool
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
	if char.CharacterData.Valid {
		if exhaustion, ok := rest.ExhaustionLevelFromCharacterData(char.CharacterData.RawMessage); ok {
			data.ExhaustionLevel = exhaustion
			data.ExhaustionLevelSet = true
		}
	}

	return data, nil
}
