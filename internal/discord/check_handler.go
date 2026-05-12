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
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/combat"
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

// CheckOpponentResolver looks up the contested-check opposing roll modifier
// for a target combatant identified by short ID. Returns (displayName,
// modifier, true) on success or ("", 0, false) when the opponent or their
// stats can't be resolved (the handler then falls back to a regular single
// check). Implementations typically join through Character / Creature
// ability scores for the same skill the initiator rolled. (med-32 / Phase 81)
type CheckOpponentResolver interface {
	ResolveContestedOpponent(ctx context.Context, encounterID uuid.UUID, targetShortID, skill string) (name string, modifier int, ok bool)
}

// CheckArmorLookup resolves an armor row by ID. Used by /check stealth to
// honor the armor's stealth_disadv flag (med-31 / Phase 75b). Wrap
// refdata.Queries.GetArmor in production. Nil disables the lookup so the
// handler keeps working in unit tests built before this wiring landed.
type CheckArmorLookup interface {
	GetArmor(ctx context.Context, id string) (refdata.Armor, error)
}

// CheckTargetResolver locates the caster's combatant (the one rolling the
// /check) and the target combatant by short ID in the active encounter so
// the handler can validate adjacency and (in combat) deduct an action.
// Returns ok=false when either combatant can't be resolved or the caster
// isn't in the encounter. F-81 targeted-check wiring.
type CheckTargetResolver interface {
	ResolveTargetCombatant(ctx context.Context, encounterID uuid.UUID, casterDiscordID, targetShortID string) (caster, target refdata.Combatant, ok bool)
}

// CheckTurnProvider resolves the active turn for the invoking character so
// /check medicine on an adjacent ally deducts the per-turn Action when in
// combat (F-81). When unset, /check still works but never consumes an
// action — matches the historical out-of-combat behavior.
type CheckTurnProvider interface {
	GetActiveTurnForCharacter(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

// CheckZoneLookup lists active encounter zones so /check perception can apply
// obscurement-driven disadvantage and surface the lighting reason in the
// response. Nil disables the wiring (handler keeps the historical "no
// obscurement effect" behavior for unit tests built before this wiring landed).
// E-69 / Phase 69.
type CheckZoneLookup interface {
	ListZonesForEncounter(ctx context.Context, encounterID uuid.UUID) ([]combat.ZoneInfo, error)
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
	// med-32 / Phase 81: opponent resolver wiring for contested checks
	// triggered by the slash command's `target` option. Nil disables the
	// contested path entirely (the handler then runs a regular single
	// check, matching the historical behaviour).
	opponentResolver CheckOpponentResolver
	// med-31 / Phase 75b: armor lookup so /check stealth applies armor
	// stealth_disadv automatically. Nil disables the lookup (preserves
	// pre-wiring behaviour for unit tests).
	armorLookup CheckArmorLookup
	// F-81: targeted-check positional / action-cost wiring. Both are
	// optional; when unset the targeted /check path keeps its existing
	// behaviour (contested route via opponentResolver, otherwise plain
	// single check with no adjacency or action enforcement).
	targetResolver CheckTargetResolver
	turnProvider   CheckTurnProvider
	// E-69 / Phase 69: zone lookup so /check perception in an obscured
	// area auto-applies disadvantage and includes the lighting reason in
	// the response. Nil disables the effect (preserves pre-wiring
	// behaviour for tests built before this wiring landed).
	zoneLookup CheckZoneLookup
}

// SetArmorLookup wires the equipped-armor lookup so /check stealth honors the
// armor's stealth_disadv flag. Nil disables the lookup (med-31 / Phase 75b).
func (h *CheckHandler) SetArmorLookup(l CheckArmorLookup) { h.armorLookup = l }

// SetOpponentResolver wires the contested-check opponent lookup. Pass nil
// to keep the historical "ignore target" behaviour (med-32).
func (h *CheckHandler) SetOpponentResolver(r CheckOpponentResolver) { h.opponentResolver = r }

// SetTargetResolver wires the positional target lookup used by the targeted
// (non-contested) /check path. F-81: when set, `/check medicine target:AR`
// validates adjacency and deducts an action when in combat. Nil keeps the
// historical "no enforcement" behaviour.
func (h *CheckHandler) SetTargetResolver(r CheckTargetResolver) { h.targetResolver = r }

// SetTurnProvider wires the active-turn lookup so an in-combat targeted
// /check (e.g. medicine on a downed ally) costs the caster's Action.
// Nil keeps the historical "no cost" behaviour. (F-81)
func (h *CheckHandler) SetTurnProvider(p CheckTurnProvider) { h.turnProvider = p }

// SetNotifier wires the dm-queue Notifier so non-trivial /check rolls are
// gated through #dm-queue for DM narration. When nil (or unset), every
// /check responds immediately with the numeric result and no queue post.
func (h *CheckHandler) SetNotifier(n dmqueue.Notifier) { h.notifier = n }

// SetZoneLookup wires the active-zone lookup so /check perception inside
// obscurement applies disadvantage and surfaces the lighting reason. Nil
// disables the effect. (E-69 / Phase 69)
func (h *CheckHandler) SetZoneLookup(z CheckZoneLookup) { h.zoneLookup = z }

// HasZoneLookup reports whether a non-nil CheckZoneLookup has been wired.
// Production-wiring tests use this to detect the COMBAT-MISC-followup
// regression (nil lookup => obscurement-driven disadvantage on Perception
// silently no-ops).
func (h *CheckHandler) HasZoneLookup() bool { return h.zoneLookup != nil }

// HasRollLogger reports whether a non-nil dice.RollHistoryLogger has been
// wired on this handler. Used by production-wiring tests to detect the
// Phase 18 silent-no-op (nil rollLogger means no #roll-history posts).
func (h *CheckHandler) HasRollLogger() bool { return h.rollLogger != nil }

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
	skill, adv, disadv, dc, hasDC, target := h.parseOptions(data.Options)
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

	skillKey := strings.ToLower(skill)

	// med-31 / Phase 75b: apply armor stealth disadvantage when the player
	// rolls /check stealth wearing armor flagged stealth_disadv. The
	// resolved disadv flag combines with any explicit --disadv via the
	// usual rollModeFromFlags cancellation rules.
	armorImposesDisadv := h.armorImposesStealthDisadv(ctx, char, skillKey)
	rollMode := rollModeFromFlags(adv, disadv || armorImposesDisadv)

	// Build input
	input := check.SingleCheckInput{
		Scores:           charData.Scores,
		Skill:            skillKey,
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

	// E-69 / Phase 69: apply obscurement-driven roll mode for sight-based
	// checks (Perception). When the caster is in an active encounter and
	// stands on an obscured tile, ObscurementCheckEffect imposes
	// disadvantage. The reason is appended to the formatted response so
	// the player sees why the check was modified.
	obscurementReason := h.applyObscurementToInput(ctx, interaction, char.ID, &input)

	// med-32 / Phase 81: targeted contested check. When the player supplies
	// a target short ID (e.g. /check athletics target:G1), look up the
	// opposing combatant in the active encounter and run a contested check
	// using check.Service.ContestedCheck. Falls back to a single check when
	// the target cannot be resolved (no encounter / unknown short id).
	if target != "" {
		if h.handleContestedCheck(ctx, interaction, char, input, target) {
			return
		}
		// F-81: non-contested targeted check (e.g. `/check medicine
		// target:AR` on a downed ally). Validate tile adjacency and, in
		// combat, deduct an action up-front. Out-of-combat: skip the
		// cost. When resolution fails (no resolver wired / target not in
		// encounter), fall through to the plain single check so the
		// player still gets a result.
		if rejectMsg, blocked := h.enforceTargetedCheckPreconditions(ctx, interaction, char, target); blocked {
			respondEphemeral(h.session, interaction, rejectMsg)
			return
		}
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
	if obscurementReason != "" {
		msg += fmt.Sprintf("\n> %s", obscurementReason)
	}
	respondEphemeral(h.session, interaction, msg)

	h.logRollIfWanted(char, result)
}

// handleContestedCheck attempts to resolve the contested-check path when
// the player supplied a target short ID. Returns true when the contested
// path was taken (initiator + opponent both rolled, response posted);
// false when the path could not be taken and the caller should fall
// through to the regular single check. (med-32 / Phase 81)
func (h *CheckHandler) handleContestedCheck(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, input check.SingleCheckInput, target string) bool {
	if h.opponentResolver == nil || h.encounterProvider == nil {
		return false
	}
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, discordUserID(interaction))
	if err != nil {
		return false
	}
	oppName, oppMod, ok := h.opponentResolver.ResolveContestedOpponent(ctx, encounterID, target, input.Skill)
	if !ok {
		return false
	}

	// Initiator modifier mirrors what SingleCheck would compute internally.
	initiatorMod := character.SkillModifier(input.Scores, input.Skill, input.ProficientSkills, input.ExpertiseSkills, input.JackOfAllTrades, input.ProfBonus)

	contested := h.checkService.ContestedCheck(check.ContestedCheckInput{
		Initiator: check.ContestedParticipant{Name: char.Name, Modifier: initiatorMod, RollMode: input.RollMode},
		Opponent:  check.ContestedParticipant{Name: oppName, Modifier: oppMod, RollMode: dice.Normal},
	})

	msg := formatContestedCheckResult(input.Skill, contested)
	respondEphemeral(h.session, interaction, msg)

	// Log the initiator's roll to roll history; opponent's roll is the DM's
	// to log if they care. Mirrors logRollIfWanted semantics.
	if h.rollLogger != nil {
		_ = h.rollLogger.LogRoll(dice.RollLogEntry{
			DiceRolls:  []dice.GroupResult{{Die: 20, Count: 1, Results: contested.InitiatorD20.Rolls, Total: contested.InitiatorD20.Chosen}},
			Total:      contested.InitiatorTotal,
			Expression: fmt.Sprintf("d20+%d", initiatorMod),
			Roller:     char.Name,
			Purpose:    fmt.Sprintf("contested %s vs %s", input.Skill, oppName),
			Breakdown:  contested.InitiatorD20.Breakdown,
			Timestamp:  contested.InitiatorD20.Timestamp,
		})
	}
	return true
}

// enforceTargetedCheckPreconditions handles the non-contested targeted
// check path (F-81). When a target resolver is wired and the active
// encounter resolves, it validates tile adjacency (5ft Chebyshev) and, if
// a turn provider is wired and the caster has an active turn (in combat),
// it deducts an Action up-front. Returns blocked=true with a player-
// facing message when adjacency fails or the Action is already spent.
// Returns blocked=false when the path succeeds or when no resolver is
// wired (legacy callers fall through to plain SingleCheck unchanged).
func (h *CheckHandler) enforceTargetedCheckPreconditions(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, target string) (string, bool) {
	if h.targetResolver == nil || h.encounterProvider == nil {
		return "", false
	}
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, discordUserID(interaction))
	if err != nil {
		// Not in combat — skip enforcement, plain single check.
		return "", false
	}
	caster, targetComb, ok := h.targetResolver.ResolveTargetCombatant(ctx, encounterID, discordUserID(interaction), target)
	if !ok {
		// Target not in this encounter — skip enforcement.
		return "", false
	}
	if !isAdjacent(caster, targetComb) {
		return fmt.Sprintf("❌ %s is not adjacent — targeted checks require 5ft.", targetComb.DisplayName), true
	}
	if h.turnProvider == nil {
		return "", false
	}
	turn, hasTurn, err := h.turnProvider.GetActiveTurnForCharacter(ctx, interaction.GuildID, char.ID)
	if err != nil || !hasTurn {
		return "", false
	}
	if turn.ActionUsed {
		return "❌ You've already used your action this turn.", true
	}
	updated, err := combat.UseResource(turn, combat.ResourceAction)
	if err != nil {
		return fmt.Sprintf("❌ Cannot perform targeted check: %v", err), true
	}
	if _, err := h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updated)); err != nil {
		// Persistence failure: don't block the roll — combat-state
		// corrections can be made by the DM.
		return "", false
	}
	return "", false
}

// isAdjacent reports whether two combatants share a 5ft tile boundary
// (Chebyshev distance <= 1 with both positions parseable).
func isAdjacent(a, b refdata.Combatant) bool {
	aCol, aRow, okA := parseCombatantCoord(a)
	bCol, bRow, okB := parseCombatantCoord(b)
	if !okA || !okB {
		return false
	}
	d := absInt(aCol-bCol)
	if dr := absInt(aRow - bRow); dr > d {
		d = dr
	}
	return d <= 1
}

func parseCombatantCoord(c refdata.Combatant) (int, int, bool) {
	if c.PositionCol == "" {
		return 0, 0, false
	}
	col := int(c.PositionCol[0]-'A')
	if col < 0 || col > 25 {
		return 0, 0, false
	}
	return col, int(c.PositionRow), true
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// formatContestedCheckResult renders a contested-check outcome as a single
// Discord message. Tie outcomes call out the spec's tie-handling note.
func formatContestedCheckResult(skill string, r check.ContestedCheckResult) string {
	skillLabel := titleSkill(skill)
	if r.Tie {
		return fmt.Sprintf(
			"🎲 **Contested %s** — Tie!\n• %s rolled %d\n• %s rolled %d\n_Tie: status quo holds._",
			skillLabel,
			r.InitiatorD20.Breakdown, r.InitiatorTotal,
			r.OpponentD20.Breakdown, r.OpponentTotal,
		)
	}
	return fmt.Sprintf(
		"🎲 **Contested %s** — **%s wins**\n• Initiator rolled %d (%s)\n• Opponent rolled %d (%s)",
		skillLabel,
		r.Winner,
		r.InitiatorTotal, r.InitiatorD20.Breakdown,
		r.OpponentTotal, r.OpponentD20.Breakdown,
	)
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

// parseOptions extracts skill, adv, disadv, dc, hasDC, and target from
// command options. (med-32: target was previously parsed but discarded.)
func (h *CheckHandler) parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) (skill string, adv, disadv bool, dc int, hasDC bool, target string) {
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
		case "target":
			target = opt.StringValue()
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

// armorImposesStealthDisadv reports whether the character's equipped armor
// forces disadvantage on a stealth check (armor.stealth_disadv = true). It
// returns false for non-stealth checks, when no armor lookup is wired, when
// the character has no equipped armor, or when the armor row can't be
// fetched. (med-31 / Phase 75b)
func (h *CheckHandler) armorImposesStealthDisadv(ctx context.Context, char refdata.Character, skill string) bool {
	if skill != "stealth" {
		return false
	}
	if h.armorLookup == nil {
		return false
	}
	if !char.EquippedArmor.Valid || char.EquippedArmor.String == "" {
		return false
	}
	armor, err := h.armorLookup.GetArmor(ctx, char.EquippedArmor.String)
	if err != nil {
		return false
	}
	if !armor.StealthDisadv.Valid || !armor.StealthDisadv.Bool {
		return false
	}
	// Honor Medium Armor Master: negates stealth disadvantage for medium
	// armor (mirrors combat.standard_actions.go Hide).
	if armor.ArmorType == "medium" && hasFeatureEffectKey(char.Features, "no_stealth_disadvantage_medium_armor") {
		return false
	}
	return true
}

// hasFeatureEffectKey reports whether a JSON-encoded features blob contains
// any feature whose mechanical_effect string matches key. Defensive: returns
// false on parse errors (parity with combat.hasFeatureEffect).
func hasFeatureEffectKey(features pqtype.NullRawMessage, key string) bool {
	if !features.Valid || len(features.RawMessage) == 0 {
		return false
	}
	var feats []struct {
		MechanicalEffect string `json:"mechanical_effect"`
	}
	if err := json.Unmarshal(features.RawMessage, &feats); err != nil {
		return false
	}
	for _, f := range feats {
		if f.MechanicalEffect == key {
			return true
		}
	}
	return false
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

// applyObscurementToInput updates the SingleCheckInput's RollMode for
// sight-based checks (Perception today) when the caster stands inside an
// obscured zone and returns a player-facing reason string. All wiring is
// nil-safe: when the zone lookup, combatant lookup, encounter provider, or
// caster combatant can't be resolved the function is a no-op. The reason
// (when present) is suitable for appending to the formatted check response.
// E-69 / Phase 69.
func (h *CheckHandler) applyObscurementToInput(ctx context.Context, interaction *discordgo.Interaction, charID uuid.UUID, input *check.SingleCheckInput) string {
	if h.zoneLookup == nil || h.encounterProvider == nil || h.combatantLookup == nil {
		return ""
	}
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, discordUserID(interaction))
	if err != nil {
		return ""
	}
	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return ""
	}
	var caster refdata.Combatant
	found := false
	for _, c := range combatants {
		if !c.CharacterID.Valid || c.CharacterID.UUID != charID {
			continue
		}
		caster = c
		found = true
		break
	}
	if !found {
		return ""
	}
	zones, err := h.zoneLookup.ListZonesForEncounter(ctx, encounterID)
	if err != nil || len(zones) == 0 {
		return ""
	}
	col := combat.ColToIndex(caster.PositionCol)
	row := int(caster.PositionRow) - 1
	// Vision capabilities are zero-valued for now; matches the production
	// attack path which also doesn't yet derive Darkvision / Devil's Sight
	// flags from character / creature rows. Wiring those in is tracked
	// separately under E-68.
	vision := combat.VisionCapabilities{}
	level := combat.CombatantObscurement(col, row, zones, vision)
	if level == combat.NotObscured {
		return ""
	}
	mode, reason := combat.ObscurementCheckEffect(level, input.Skill)
	if reason == "" {
		return ""
	}
	input.RollMode = dice.CombineRollModes(input.RollMode, mode)
	return reason
}
