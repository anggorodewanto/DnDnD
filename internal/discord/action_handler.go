package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// ActionEncounterResolver resolves a Discord user to their active encounter.
type ActionEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// ActionCombatService is the combat-side surface the /action handler needs.
// *combat.Service structurally satisfies this interface.
type ActionCombatService interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	FreeformAction(ctx context.Context, cmd combat.FreeformActionCommand) (combat.FreeformActionResult, error)
	CancelFreeformAction(ctx context.Context, cmd combat.CancelFreeformActionCommand) (combat.CancelFreeformActionResult, error)
	CancelExplorationFreeformAction(ctx context.Context, combatantID uuid.UUID) (combat.CancelFreeformActionResult, error)
	ReadyAction(ctx context.Context, cmd combat.ReadyActionCommand) (combat.ReadyActionResult, error)

	// D-53 / Phase 53: Action Surge dispatch.
	ActionSurge(ctx context.Context, cmd combat.ActionSurgeCommand) (combat.ActionSurgeResult, error)

	// D-54 / Phase 54: standard-action dispatch (Hide is also wired here per D-57).
	Dash(ctx context.Context, cmd combat.DashCommand) (combat.DashResult, error)
	Disengage(ctx context.Context, cmd combat.DisengageCommand) (combat.DisengageResult, error)
	Dodge(ctx context.Context, cmd combat.DodgeCommand) (combat.DodgeResult, error)
	Help(ctx context.Context, cmd combat.HelpCommand) (combat.HelpResult, error)
	Hide(ctx context.Context, cmd combat.HideCommand, roller *dice.Roller) (combat.HideResult, error)
	Stand(ctx context.Context, cmd combat.StandCommand) (combat.StandResult, error)
	DropProne(ctx context.Context, cmd combat.DropProneCommand) (combat.DropProneResult, error)
	Escape(ctx context.Context, cmd combat.EscapeCommand, roller *dice.Roller) (combat.EscapeResult, error)
	Grapple(ctx context.Context, cmd combat.GrappleCommand, roller *dice.Roller) (combat.GrappleResult, error)

	// D-50 / Phase 50: Channel Divinity dispatch. DM-queued options route via
	// ChannelDivinityDMQueue for DM resolution; the four resolved-in-engine
	// options route to TurnUndead / PreserveLife / SacredWeapon / VowOfEnmity.
	TurnUndead(ctx context.Context, cmd combat.TurnUndeadCommand, roller *dice.Roller) (combat.TurnUndeadResult, error)
	PreserveLife(ctx context.Context, cmd combat.PreserveLifeCommand) (combat.PreserveLifeResult, error)
	SacredWeapon(ctx context.Context, cmd combat.SacredWeaponCommand) (combat.SacredWeaponResult, error)
	VowOfEnmity(ctx context.Context, cmd combat.VowOfEnmityCommand) (combat.VowOfEnmityResult, error)
	ChannelDivinityDMQueue(ctx context.Context, cmd combat.ChannelDivinityDMQueueCommand) (combat.DMQueueResult, error)

	// D-52 / Phase 52: Lay on Hands moves to /action (it costs an action).
	LayOnHands(ctx context.Context, cmd combat.LayOnHandsCommand) (combat.LayOnHandsResult, error)
}

// ActionTurnProvider loads turn rows for the combat path.
type ActionTurnProvider interface {
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// ActionCampaignProvider maps a guild to its campaign so the handler can
// resolve the invoker's character via characterLookup.
type ActionCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// ActionCharacterLookup resolves a Discord user to their character in a
// campaign. Used to validate turn ownership in combat and to pick the
// invoker's PC combatant in exploration.
type ActionCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// ActionPendingStore persists a pending_actions row for the exploration
// freeform-action path (combat path does it via combat.Service).
type ActionPendingStore interface {
	CreatePendingAction(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error)
}

// ActionStabilizeStore persists the dying target's updated death-save
// tallies after a successful Medicine check. C-43 / Phase 43: a DC 10
// Medicine check stabilizes the target, which we represent as DeathSaves{
// Successes: 3 } (the StabilizeTarget helper builds the exact value).
type ActionStabilizeStore interface {
	UpdateCombatantDeathSaves(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error)
}

// ActionZoneLookup lists encounter zones so /action hide can gate on
// obscurement (you can hide only when at least lightly obscured per 5e).
// Nil-safe: when unset the handler keeps the historical "always allow hide"
// behaviour. E-69 / Phase 69.
type ActionZoneLookup interface {
	ListZonesForEncounter(ctx context.Context, encounterID uuid.UUID) ([]combat.ZoneInfo, error)
}

// ActionSpeedLookup resolves a combatant's actual walk speed in feet.
// /action stand uses this to compute the half-movement stand cost so
// halflings (25ft) and tabaxi (35ft) pay the spec-correct 12-13ft instead
// of the hardcoded 15ft default. Nil-safe: when unset, stand keeps the
// historical 30ft default. (D-54-followup)
type ActionSpeedLookup interface {
	LookupWalkSpeed(ctx context.Context, combatant refdata.Combatant) (int, error)
}

// ActionMedicineLookup resolves the actor's Medicine-check modifier
// (WIS modifier + proficiency bonus when proficient in Medicine, plus
// double-prof for expertise). /action stabilize uses this so a high-WIS
// proficient cleric rolls d20 + (e.g.) +6 vs DC 10 rather than the
// historical flat d20. Nil-safe: when unset, stabilize falls back to a
// flat d20 + 0 to preserve the pre-followup behaviour. (C-43-stabilize-
// followup)
type ActionMedicineLookup interface {
	LookupMedicineModifier(ctx context.Context, combatant refdata.Combatant) (int, error)
}

// ActionHandler wires /action [freeform text] and /action cancel into the
// freeform-action service (combat) or directly onto the dm-queue notifier
// (exploration, where there is no turn / action economy).
type ActionHandler struct {
	session           Session
	resolver          ActionEncounterResolver
	combatService     ActionCombatService
	turnProvider      ActionTurnProvider
	campaignProvider  ActionCampaignProvider
	characterLookup   ActionCharacterLookup
	pendingStore      ActionPendingStore
	notifier          dmqueue.Notifier
	roller            *dice.Roller
	channelIDProvider CampaignSettingsProvider
	// C-43-stabilize: optional store for persisting the dying target's
	// updated death saves when /action stabilize succeeds. nil-safe — when
	// unset, the stabilize subcommand reports the roll result but warns
	// that persistence is disabled.
	stabilizeStore ActionStabilizeStore
	// E-69 / Phase 69: optional zone lookup so /action hide gates on
	// obscurement (you can hide only when at least lightly obscured).
	// nil-safe — when unset, Hide keeps its historical "always allow"
	// behaviour to preserve existing tests.
	zoneLookup ActionZoneLookup
	// D-54-followup: optional walk-speed lookup so /action stand resolves
	// the actor's actual max walk speed for the half-movement cost rule.
	// nil-safe — when unset, stand falls back to the historical 30ft.
	speedLookup ActionSpeedLookup
	// C-43-stabilize-followup: optional Medicine modifier lookup so
	// /action stabilize rolls d20 + WIS_mod + Medicine proficiency instead
	// of a flat d20. nil-safe — when unset, the +0 modifier is used.
	medicineLookup ActionMedicineLookup
}

// NewActionHandler constructs an ActionHandler. The notifier is wired later
// via SetNotifier; headless tests may leave it nil and the exploration path
// will no-op the dm-queue post (pending_action row still persists).
func NewActionHandler(
	session Session,
	resolver ActionEncounterResolver,
	combatService ActionCombatService,
	turnProvider ActionTurnProvider,
	campaignProvider ActionCampaignProvider,
	characterLookup ActionCharacterLookup,
	pendingStore ActionPendingStore,
) *ActionHandler {
	return &ActionHandler{
		session:          session,
		resolver:         resolver,
		combatService:    combatService,
		turnProvider:     turnProvider,
		campaignProvider: campaignProvider,
		characterLookup:  characterLookup,
		pendingStore:     pendingStore,
	}
}

// SetNotifier wires the dm-queue Notifier used by the exploration freeform
// path. A nil notifier causes exploration posts to be silent no-ops; the
// pending_actions row still lands in the DB.
func (h *ActionHandler) SetNotifier(n dmqueue.Notifier) { h.notifier = n }

// SetRoller wires the dice roller used by subcommand dispatch (Hide / Escape
// / TurnUndead). When unset, those dispatches reject with a usage hint so
// the freeform fallback path doesn't silently swallow them.
func (h *ActionHandler) SetRoller(r *dice.Roller) { h.roller = r }

// SetChannelIDProvider wires the campaign settings provider for
// combat-log mirroring on subcommand dispatch results.
func (h *ActionHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// SetStabilizeStore wires the death-save persistence for /action stabilize.
// nil-safe — leaving this unset surfaces a "stabilize unavailable" message
// rather than silently dropping the roll.
func (h *ActionHandler) SetStabilizeStore(s ActionStabilizeStore) {
	h.stabilizeStore = s
}

// HasStabilizeStore reports whether a non-nil ActionStabilizeStore has been
// wired. Production-wiring tests use this to detect the C-43-followup
// regression (nil store → /action stabilize reports "not available").
func (h *ActionHandler) HasStabilizeStore() bool { return h.stabilizeStore != nil }

// SetZoneLookup wires the encounter-zone lookup so /action hide checks
// obscurement (per 5e: hide requires at least light obscurement, total cover,
// or heavy distraction). Nil disables the gate. (E-69 / Phase 69)
func (h *ActionHandler) SetZoneLookup(z ActionZoneLookup) { h.zoneLookup = z }

// HasZoneLookup reports whether a non-nil ActionZoneLookup has been wired.
// Production-wiring tests use this to detect the COMBAT-MISC-followup
// regression (nil lookup => /action hide degrades to "always allow" instead
// of enforcing obscurement gating).
func (h *ActionHandler) HasZoneLookup() bool { return h.zoneLookup != nil }

// SetSpeedLookup wires the walk-speed resolver used by /action stand so
// the half-movement cost reflects the actor's real walk speed (Halfling
// 25ft / Tabaxi 35ft / etc.). nil-safe — when unset, stand falls back to
// 30ft. (D-54-followup)
func (h *ActionHandler) SetSpeedLookup(s ActionSpeedLookup) { h.speedLookup = s }

// HasSpeedLookup reports whether a non-nil ActionSpeedLookup has been
// wired. Production-wiring tests use this to detect the D-54-followup
// regression (nil lookup => /action stand falls back to 30ft).
func (h *ActionHandler) HasSpeedLookup() bool { return h.speedLookup != nil }

// SetMedicineLookup wires the Medicine-modifier resolver used by /action
// stabilize so the roll is d20 + WIS_mod + Medicine proficiency. nil-safe
// — when unset, the roll keeps the historical flat d20. (C-43-stabilize-
// followup)
func (h *ActionHandler) SetMedicineLookup(m ActionMedicineLookup) { h.medicineLookup = m }

// HasMedicineLookup reports whether a non-nil ActionMedicineLookup has
// been wired. Production-wiring tests use this to detect the C-43-stabilize-
// followup regression (nil lookup => /action stabilize is a flat d20).
func (h *ActionHandler) HasMedicineLookup() bool { return h.medicineLookup != nil }

// Handle dispatches the /action slash command.
func (h *ActionHandler) Handle(interaction *discordgo.Interaction) {
	rawAction := strings.TrimSpace(optionString(interaction, "action"))
	rawArgs := strings.TrimSpace(optionString(interaction, "args"))

	if rawAction == "" {
		respondEphemeral(h.session, interaction, "Please provide an action (e.g. `/action flip the table`).")
		return
	}

	sub := normalizeActionSubcommand(rawAction)
	isCancel := sub == "cancel" && rawArgs == ""
	isReady := sub == "ready"

	actionText := rawAction
	if rawArgs != "" {
		actionText = rawAction + " " + rawArgs
	}

	ctx := context.Background()
	userID := discordUserID(interaction)

	encounterID, err := h.resolver.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "You are not in an active encounter.")
		return
	}

	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load encounter.")
		return
	}

	if encounter.Mode == "exploration" {
		h.handleExploration(ctx, interaction, encounter, userID, actionText, isCancel)
		return
	}

	// Combat path: a small number of subcommand names route to dedicated
	// combat services (D-47..D-57). Everything else falls through to the
	// freeform / cancel / ready path so the DM-queue surface still works.
	if h.isDispatchSubcommand(sub) {
		h.handleCombatSubcommand(ctx, interaction, encounter, encounterID, userID, sub, rawArgs)
		return
	}

	h.handleCombat(ctx, interaction, encounter, userID, actionText, isCancel, isReady, rawArgs)
}

// normalizeActionSubcommand lower-cases and de-dashes a raw subcommand name so
// /action Channel-Divinity, /action channeldivinity, and /action channel-divinity
// all map to the same canonical key.
func normalizeActionSubcommand(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// isDispatchSubcommand reports whether the subcommand name routes to a
// dedicated combat service rather than the freeform fallback.
func (h *ActionHandler) isDispatchSubcommand(sub string) bool {
	switch sub {
	case "surge", "action-surge",
		"dash", "disengage", "dodge", "help", "hide",
		"stand", "drop-prone", "dropprone",
		"escape", "grapple",
		"channel-divinity", "channeldivinity",
		"lay-on-hands", "layonhands",
		"stabilize":
		return true
	}
	return false
}

// handleCombat handles the combat-mode freeform post or cancel. Requires an
// active turn and verifies turn ownership against the invoker's character.
func (h *ActionHandler) handleCombat(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	userID, actionText string,
	isCancel, isReady bool,
	rawArgs string,
) {
	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	turn, err := h.turnProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load turn.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return
	}

	if !h.combatantBelongsToUser(ctx, interaction.GuildID, userID, combatant) {
		respondEphemeral(h.session, interaction, "It's not your turn.")
		return
	}

	// C-43-block-commands: a dying or incapacitated combatant cannot
	// post freeform actions (cancel falls through so they can withdraw
	// a queued action posted before they dropped).
	if !isCancel {
		if msg, blocked := incapacitatedRejection(combatant); blocked {
			respondEphemeral(h.session, interaction, msg)
			return
		}
	}

	if isCancel {
		h.performCombatCancel(ctx, interaction, combatant, turn)
		return
	}

	if isReady {
		spellName := strings.TrimSpace(optionString(interaction, "spell"))
		spellSlot := optionInt(interaction, "slot")
		h.performReadyAction(ctx, interaction, combatant, turn, rawArgs, spellName, spellSlot)
		return
	}

	result, err := h.combatService.FreeformAction(ctx, combat.FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: actionText,
		GuildID:    interaction.GuildID,
		CampaignID: encounter.CampaignID.String(),
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to post action: %v", err))
		return
	}
	respondEphemeral(h.session, interaction, result.CombatLog)
}

// performReadyAction dispatches /action ready into combat.Service.ReadyAction
// and posts the resulting combat log line. The trigger description is taken
// from rawArgs verbatim ("ready" itself was stripped before dispatch).
//
// E-71-followup: when the optional spellName/spellSlot are non-empty, they
// are threaded into ReadyActionCommand so the service-side ReadyAction
// expends the slot at ready-time, sets concentration, and surfaces the
// spell name + slot level in FormatReadiedActionsStatus.
func (h *ActionHandler) performReadyAction(
	ctx context.Context,
	interaction *discordgo.Interaction,
	combatant refdata.Combatant,
	turn refdata.Turn,
	description, spellName string,
	spellSlot int,
) {
	if strings.TrimSpace(description) == "" {
		respondEphemeral(h.session, interaction, "Please describe the readied action (e.g. `/action ready args:shoot anyone who opens the door`).")
		return
	}
	result, err := h.combatService.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:      combatant,
		Turn:           turn,
		Description:    description,
		SpellName:      spellName,
		SpellSlotLevel: spellSlot,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Ready action failed: %v", err))
		return
	}
	respondEphemeral(h.session, interaction, result.CombatLog)
}

// performCombatCancel invokes CancelFreeformAction and translates the
// freeform-action sentinel errors into the spec-mandated user messages.
func (h *ActionHandler) performCombatCancel(
	ctx context.Context,
	interaction *discordgo.Interaction,
	combatant refdata.Combatant,
	turn refdata.Turn,
) {
	result, err := h.combatService.CancelFreeformAction(ctx, combat.CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      turn,
	})
	h.respondCancelResult(interaction, result, err)
}

// combatantBelongsToUser returns true when the invoking Discord user's
// character maps to the turn's combatant. NPCs (CharacterID NULL) always
// return false so a player cannot cancel an enemy's pending action. Any
// lookup failure also returns false (handled as "not your turn").
func (h *ActionHandler) combatantBelongsToUser(ctx context.Context, guildID, userID string, combatant refdata.Combatant) bool {
	if !combatant.CharacterID.Valid {
		return false
	}
	if h.campaignProvider == nil || h.characterLookup == nil {
		return false
	}
	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return false
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		return false
	}
	return combatant.CharacterID.UUID == char.ID
}

// handleExploration handles the exploration-mode freeform post or cancel.
// There is no Turn (and no action resource), so the notifier is driven
// directly and a pending_actions row is persisted without turn updates.
func (h *ActionHandler) handleExploration(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	userID, actionText string,
	isCancel bool,
) {
	combatant, ok := h.resolveExplorationCombatant(ctx, interaction.GuildID, userID, encounter.ID)
	if !ok {
		respondEphemeral(h.session, interaction, "Could not find your character in this encounter.")
		return
	}

	if isCancel {
		h.performExplorationCancel(ctx, interaction, combatant)
		return
	}

	itemID := h.postExplorationDMQueue(ctx, combatant, actionText, interaction.GuildID, encounter.CampaignID.String())

	if _, err := h.pendingStore.CreatePendingAction(ctx, refdata.CreatePendingActionParams{
		EncounterID:   encounter.ID,
		CombatantID:   combatant.ID,
		ActionText:    actionText,
		DmQueueItemID: nullableUUIDFromStr(itemID),
	}); err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to record action: %v", err))
		return
	}

	respondEphemeral(h.session, interaction, fmt.Sprintf("🎭 Sent to DM queue: \"%s\"", actionText))
}

// performExplorationCancel invokes CancelExplorationFreeformAction and
// mirrors the combat cancel error translation.
func (h *ActionHandler) performExplorationCancel(
	ctx context.Context,
	interaction *discordgo.Interaction,
	combatant refdata.Combatant,
) {
	result, err := h.combatService.CancelExplorationFreeformAction(ctx, combatant.ID)
	h.respondCancelResult(interaction, result, err)
}

// respondCancelResult translates a CancelFreeformAction* outcome into the
// spec-mandated ephemeral reply. Shared between the combat and exploration
// cancel paths since they use identical error sentinels and success wording.
func (h *ActionHandler) respondCancelResult(
	interaction *discordgo.Interaction,
	result combat.CancelFreeformActionResult,
	err error,
) {
	switch {
	case errors.Is(err, combat.ErrNoPendingAction):
		respondEphemeral(h.session, interaction, "❌ No pending action to cancel.")
	case errors.Is(err, combat.ErrActionAlreadyResolved):
		respondEphemeral(h.session, interaction, "❌ That action has already been resolved — use `/undo` to request a correction instead.")
	case err != nil:
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to cancel action: %v", err))
	default:
		respondEphemeral(h.session, interaction, fmt.Sprintf("✅ Pending action cancelled: *%s*", result.PendingAction.ActionText))
	}
}

// postExplorationDMQueue posts the freeform-action event through the wired
// notifier and returns the resulting itemID. Returns "" when no notifier is
// wired or the notifier itself returned an empty id (silent no-op).
func (h *ActionHandler) postExplorationDMQueue(
	ctx context.Context,
	combatant refdata.Combatant,
	actionText, guildID, campaignID string,
) string {
	if h.notifier == nil {
		return ""
	}
	itemID, err := h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindFreeformAction,
		PlayerName: combatant.DisplayName,
		Summary:    fmt.Sprintf("\"%s\"", actionText),
		GuildID:    guildID,
		CampaignID: campaignID,
	})
	if err != nil {
		return ""
	}
	return itemID
}

// resolveExplorationCombatant picks the alive PC combatant whose character
// belongs to the invoker. Delegates to resolveExplorationPC so the logic is
// shared with move_handler.resolveExplorationMover.
func (h *ActionHandler) resolveExplorationCombatant(ctx context.Context, guildID, userID string, encounterID uuid.UUID) (refdata.Combatant, bool) {
	all, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return refdata.Combatant{}, false
	}
	var getCampaign func(ctx context.Context, guildID string) (refdata.Campaign, error)
	if h.campaignProvider != nil {
		getCampaign = h.campaignProvider.GetCampaignByGuildID
	}
	var getCharacter func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
	if h.characterLookup != nil {
		getCharacter = h.characterLookup.GetCharacterByCampaignAndDiscord
	}
	return resolveExplorationPC(ctx, all, guildID, userID, getCampaign, getCharacter)
}

// nullableUUIDFromStr parses an ID string into uuid.NullUUID. An empty or
// unparseable string yields a NULL value so the pending_actions row still
// persists even when the notifier wasn't wired or returned nothing.
func nullableUUIDFromStr(s string) uuid.NullUUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}

// --- Subcommand dispatch (D-47, D-50, D-52, D-53, D-54, D-57) ---

// handleCombatSubcommand resolves the turn / acting combatant / combatant
// roster and routes to the per-subcommand dispatch. Shares the same
// turn-ownership check as the freeform path.
func (h *ActionHandler) handleCombatSubcommand(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	encounterID uuid.UUID,
	userID, sub, args string,
) {
	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	turn, err := h.turnProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load turn.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load combatant.")
		return
	}

	if !h.combatantBelongsToUser(ctx, interaction.GuildID, userID, combatant) {
		respondEphemeral(h.session, interaction, "It's not your turn.")
		return
	}

	// C-43-block-commands: dying / incapacitated combatants cannot take
	// actions. Stabilize is invoked BY a healthy actor TARGETING a dying
	// target — the guard checks the invoker, not the target, so it is
	// safe to fall through.
	if msg, blocked := incapacitatedRejection(combatant); blocked {
		respondEphemeral(h.session, interaction, msg)
		return
	}

	combatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	switch sub {
	case "surge", "action-surge":
		h.dispatchActionSurge(ctx, interaction, encounterID, combatant, turn)
	case "dash":
		h.dispatchDash(ctx, interaction, encounterID, encounter, combatant, turn)
	case "disengage":
		h.dispatchDisengage(ctx, interaction, encounterID, combatant, turn)
	case "dodge":
		h.dispatchDodge(ctx, interaction, encounterID, encounter, combatant, turn)
	case "help":
		h.dispatchHelp(ctx, interaction, encounterID, encounter, combatant, turn, combatants, args)
	case "hide":
		h.dispatchHide(ctx, interaction, encounterID, encounter, combatant, turn, combatants)
	case "stand":
		h.dispatchStand(ctx, interaction, encounterID, combatant, turn)
	case "drop-prone", "dropprone":
		h.dispatchDropProne(ctx, interaction, encounterID, encounter, combatant, turn)
	case "escape":
		h.dispatchEscape(ctx, interaction, encounterID, encounter, combatant, turn, combatants)
	case "grapple":
		h.dispatchGrapple(ctx, interaction, encounterID, encounter, combatant, turn, combatants, args)
	case "channel-divinity", "channeldivinity":
		h.dispatchChannelDivinity(ctx, interaction, encounterID, encounter, combatant, turn, combatants, args)
	case "lay-on-hands", "layonhands":
		h.dispatchLayOnHands(ctx, interaction, encounterID, combatant, turn, combatants, args)
	case "stabilize":
		h.dispatchStabilize(ctx, interaction, encounterID, combatant, combatants, args)
	}
}

// respondAndLog mirrors the bonus-handler pattern: post the combat-log line
// to #combat-log when wired, then send the ephemeral confirmation to the
// invoker. Centralized so every subcommand uses the same shape.
func (h *ActionHandler) respondAndLog(interaction *discordgo.Interaction, encounterID uuid.UUID, log string) {
	if log == "" {
		log = "Action resolved."
	}
	postCombatLogChannel(context.Background(), h.session, h.channelIDProvider, encounterID, log)
	respondEphemeral(h.session, interaction, log)
}

// dispatchActionSurge wires /action surge (D-53).
func (h *ActionHandler) dispatchActionSurge(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, actor refdata.Combatant, turn refdata.Turn) {
	result, err := h.combatService.ActionSurge(ctx, combat.ActionSurgeCommand{
		Fighter: actor,
		Turn:    turn,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Action Surge failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchDash wires /action dash (D-54).
func (h *ActionHandler) dispatchDash(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn) {
	result, err := h.combatService.Dash(ctx, combat.DashCommand{
		Combatant: actor,
		Turn:      turn,
		Encounter: encounter,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Dash failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchDisengage wires /action disengage (D-54).
func (h *ActionHandler) dispatchDisengage(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, actor refdata.Combatant, turn refdata.Turn) {
	result, err := h.combatService.Disengage(ctx, combat.DisengageCommand{
		Combatant: actor,
		Turn:      turn,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Disengage failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchDodge wires /action dodge (D-54).
func (h *ActionHandler) dispatchDodge(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn) {
	result, err := h.combatService.Dodge(ctx, combat.DodgeCommand{
		Combatant:    actor,
		Turn:         turn,
		Encounter:    encounter,
		CurrentRound: int(encounter.RoundNumber),
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Dodge failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchHelp wires /action help <ally> [target] (D-54). When only one
// arg is given it is treated as both ally and target — i.e. the helper grants
// the ally advantage on attacking that same target adjacent to the helper.
// When two args are given, the first is the ally and the second is the
// target. Adjacency is enforced inside the combat service.
func (h *ActionHandler) dispatchHelp(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant, args string) {
	tokens := strings.Fields(args)
	if len(tokens) == 0 {
		respondEphemeral(h.session, interaction, "Help requires `<ally> [target]` (e.g. `/action help args:AR OS`).")
		return
	}

	ally, err := combat.ResolveTarget(tokens[0], combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Ally %q not found.", tokens[0]))
		return
	}

	target := ally
	if len(tokens) >= 2 {
		target, err = combat.ResolveTarget(tokens[1], combatants)
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", tokens[1]))
			return
		}
	}

	result, err := h.combatService.Help(ctx, combat.HelpCommand{
		Helper:    actor,
		Ally:      *ally,
		Target:    *target,
		Turn:      turn,
		Encounter: encounter,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Help failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchHide wires /action hide (D-57).
func (h *ActionHandler) dispatchHide(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant) {
	if h.roller == nil {
		respondEphemeral(h.session, interaction, "Hide is not available right now (no dice roller wired).")
		return
	}

	// E-69 / Phase 69: when a zone lookup is wired AND zones exist in the
	// encounter (lighting-aware combat), gate Hide on the actor's
	// effective obscurement. ObscurementAllowsHide rejects NotObscured.
	// Surface the obscurement reason on success so players see why Hide
	// was allowed.
	obscurementReason := ""
	if h.zoneLookup != nil {
		zones, zerr := h.zoneLookup.ListZonesForEncounter(ctx, encounterID)
		if zerr == nil && len(zones) > 0 {
			col := combat.ColToIndex(actor.PositionCol)
			row := int(actor.PositionRow) - 1
			level := combat.CombatantObscurement(col, row, zones, combat.VisionCapabilities{})
			if !combat.ObscurementAllowsHide(level) {
				respondEphemeral(h.session, interaction, fmt.Sprintf("%s cannot Hide here — you are in plain sight (not obscured and lacking cover).", actor.DisplayName))
				return
			}
			obscurementReason = fmt.Sprintf("%s (Hide allowed)", level.String())
		}
	}

	hostiles := filterHostiles(combatants, actor)
	result, err := h.combatService.Hide(ctx, combat.HideCommand{
		Combatant: actor,
		Turn:      turn,
		Encounter: encounter,
		Hostiles:  hostiles,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Hide failed: %v", err))
		return
	}
	logMsg := result.CombatLog
	if obscurementReason != "" {
		logMsg = fmt.Sprintf("%s\n> %s", logMsg, obscurementReason)
	}
	h.respondAndLog(interaction, encounterID, logMsg)
}

// dispatchStand wires /action stand (D-54). The half-movement-to-stand
// cost depends on the actor's actual walk speed, so we consult the wired
// ActionSpeedLookup. Halflings (25ft) pay 13ft to stand, Tabaxi (35ft) pay
// 18ft, default 30ft pays 15ft. nil lookup or a lookup error falls back to
// 30ft to preserve pre-D-54-followup behaviour. (D-54-followup)
func (h *ActionHandler) dispatchStand(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, actor refdata.Combatant, turn refdata.Turn) {
	maxSpeed := h.resolveStandSpeed(ctx, actor)
	result, err := h.combatService.Stand(ctx, combat.StandCommand{
		Combatant: actor,
		Turn:      turn,
		MaxSpeed:  maxSpeed,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Stand failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// resolveStandSpeed returns the actor's max walk speed in feet, falling
// back to 30 when no lookup is wired or the lookup returns an error /
// non-positive value. (D-54-followup)
func (h *ActionHandler) resolveStandSpeed(ctx context.Context, actor refdata.Combatant) int {
	const defaultSpeed = 30
	if h.speedLookup == nil {
		return defaultSpeed
	}
	speed, err := h.speedLookup.LookupWalkSpeed(ctx, actor)
	if err != nil || speed <= 0 {
		return defaultSpeed
	}
	return speed
}

// dispatchDropProne wires /action drop-prone (D-54).
func (h *ActionHandler) dispatchDropProne(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn) {
	result, err := h.combatService.DropProne(ctx, combat.DropProneCommand{
		Combatant:    actor,
		Turn:         turn,
		Encounter:    encounter,
		CurrentRound: int(encounter.RoundNumber),
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Drop prone failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchEscape wires /action escape [acrobatics] (D-54). The grappler is
// resolved by reading the "grappled" condition's source on the escapee.
func (h *ActionHandler) dispatchEscape(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant) {
	if h.roller == nil {
		respondEphemeral(h.session, interaction, "Escape is not available right now (no dice roller wired).")
		return
	}
	cond, ok := combat.GetCondition(actor.Conditions, "grappled")
	if !ok {
		respondEphemeral(h.session, interaction, "You are not grappled.")
		return
	}
	var grappler *refdata.Combatant
	for i := range combatants {
		if combatants[i].ID.String() == cond.SourceCombatantID {
			grappler = &combatants[i]
			break
		}
	}
	if grappler == nil {
		respondEphemeral(h.session, interaction, "Could not find the creature grappling you.")
		return
	}
	argsLower := strings.ToLower(strings.TrimSpace(optionString(interaction, "args")))
	useAcrobatics := strings.Contains(argsLower, "acrobatics")
	useAthletics := strings.Contains(argsLower, "athletics")
	result, err := h.combatService.Escape(ctx, combat.EscapeCommand{
		Escapee:       actor,
		Grappler:      *grappler,
		Turn:          turn,
		Encounter:     encounter,
		UseAcrobatics: useAcrobatics,
		UseAthletics:  useAthletics,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Escape failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchGrapple wires /action grapple <target> (SR-048 / Phase 56).
// Resolves the target from args and delegates to combat.Service.Grapple.
func (h *ActionHandler) dispatchGrapple(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant, args string) {
	if h.roller == nil {
		respondEphemeral(h.session, interaction, "Grapple is not available right now (no dice roller wired).")
		return
	}
	targetStr := strings.TrimSpace(args)
	if targetStr == "" {
		respondEphemeral(h.session, interaction, "Grapple requires a target (e.g. `/action grapple args:G1`).")
		return
	}
	target, err := combat.ResolveTarget(strings.Fields(targetStr)[0], combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", targetStr))
		return
	}
	result, err := h.combatService.Grapple(ctx, combat.GrappleCommand{
		Grappler:  actor,
		Target:    *target,
		Turn:      turn,
		Encounter: encounter,
	}, h.roller)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Grapple failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchChannelDivinity wires /action channel-divinity <option> [args] (D-50).
// Five options are recognized: turn-undead, preserve-life, sacred-weapon,
// vow-of-enmity, and a generic DM-routed bucket for everything else.
func (h *ActionHandler) dispatchChannelDivinity(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, encounter refdata.Encounter, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant, args string) {
	tokens := strings.Fields(args)
	if len(tokens) == 0 {
		respondEphemeral(h.session, interaction, "Channel Divinity requires `<option>` (e.g. `/action channel-divinity args:turn-undead`).")
		return
	}
	option := strings.ToLower(tokens[0])
	rest := strings.TrimSpace(strings.TrimPrefix(args, tokens[0]))

	switch option {
	case "turn-undead", "turnundead":
		if h.roller == nil {
			respondEphemeral(h.session, interaction, "Turn Undead is not available right now (no dice roller wired).")
			return
		}
		result, err := h.combatService.TurnUndead(ctx, combat.TurnUndeadCommand{
			Cleric:       actor,
			Turn:         turn,
			CurrentRound: int(encounter.RoundNumber),
		}, h.roller)
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Turn Undead failed: %v", err))
			return
		}
		h.respondAndLog(interaction, encounterID, result.CombatLog)
	case "preserve-life", "preservelife":
		healing, parseErr := parsePreserveLifeArgs(rest, combatants)
		if parseErr != nil {
			respondEphemeral(h.session, interaction, parseErr.Error())
			return
		}
		result, err := h.combatService.PreserveLife(ctx, combat.PreserveLifeCommand{
			Cleric:        actor,
			Turn:          turn,
			TargetHealing: healing,
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Preserve Life failed: %v", err))
			return
		}
		h.respondAndLog(interaction, encounterID, result.CombatLog)
	case "sacred-weapon", "sacredweapon":
		result, err := h.combatService.SacredWeapon(ctx, combat.SacredWeaponCommand{
			Paladin:      actor,
			Turn:         turn,
			CurrentRound: int(encounter.RoundNumber),
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Sacred Weapon failed: %v", err))
			return
		}
		h.respondAndLog(interaction, encounterID, result.CombatLog)
	case "vow-of-enmity", "vowofenmity":
		targetTokens := strings.Fields(rest)
		if len(targetTokens) == 0 {
			respondEphemeral(h.session, interaction, "Vow of Enmity requires `<target>` (e.g. `/action channel-divinity args:vow-of-enmity OS`).")
			return
		}
		target, err := combat.ResolveTarget(targetTokens[0], combatants)
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", targetTokens[0]))
			return
		}
		result, err := h.combatService.VowOfEnmity(ctx, combat.VowOfEnmityCommand{
			Paladin:      actor,
			Target:       *target,
			Turn:         turn,
			CurrentRound: int(encounter.RoundNumber),
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Vow of Enmity failed: %v", err))
			return
		}
		h.respondAndLog(interaction, encounterID, result.CombatLog)
	default:
		// Generic DM-routed option: forward to ChannelDivinityDMQueue with the
		// option name and the caller's class hint inferred from "cleric" /
		// "paladin" suffix tokens. When no class is provided the service will
		// reject (validates Cleric/Paladin only).
		className := "Cleric"
		if strings.Contains(strings.ToLower(rest), "paladin") {
			className = "Paladin"
		}
		result, err := h.combatService.ChannelDivinityDMQueue(ctx, combat.ChannelDivinityDMQueueCommand{
			Caster:     actor,
			Turn:       turn,
			OptionName: option,
			ClassName:  className,
			GuildID:    interaction.GuildID,
			CampaignID: encounter.CampaignID.String(),
		})
		if err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Channel Divinity failed: %v", err))
			return
		}
		h.respondAndLog(interaction, encounterID, result.CombatLog)
	}
}

// parsePreserveLifeArgs parses `<target>:<hp> <target>:<hp>...` into a map
// keyed by combatant ID. Tokens without a colon are rejected so the
// dispatcher can route to a clear usage hint.
func parsePreserveLifeArgs(args string, combatants []refdata.Combatant) (map[string]int32, error) {
	tokens := strings.Fields(args)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("Preserve Life requires `<target>:<hp>` pairs (e.g. `/action channel-divinity args:preserve-life AR:5 OS:3`).")
	}
	out := make(map[string]int32, len(tokens))
	for _, t := range tokens {
		parts := strings.SplitN(t, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid Preserve Life token %q (expected target:hp).", t)
		}
		target, err := combat.ResolveTarget(parts[0], combatants)
		if err != nil {
			return nil, fmt.Errorf("Target %q not found.", parts[0])
		}
		hp, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid HP %q for %s.", parts[1], parts[0])
		}
		out[target.ID.String()] = int32(hp)
	}
	return out, nil
}

// dispatchLayOnHands wires /action lay-on-hands <target> <hp> [poison] [disease] (D-52).
// Identical surface to the legacy /bonus lay-on-hands; the bonus alias remains
// deprecated for one cycle so playtest macros don't break overnight.
func (h *ActionHandler) dispatchLayOnHands(ctx context.Context, interaction *discordgo.Interaction, encounterID uuid.UUID, actor refdata.Combatant, turn refdata.Turn, combatants []refdata.Combatant, args string) {
	tokens := strings.Fields(args)
	if len(tokens) < 2 {
		respondEphemeral(h.session, interaction, "Lay on Hands requires `<target> <hp> [poison] [disease]` (e.g. `/action lay-on-hands args:AR 10`).")
		return
	}
	target, err := combat.ResolveTarget(tokens[0], combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", tokens[0]))
		return
	}
	hp, err := strconv.Atoi(tokens[1])
	if err != nil || hp < 0 {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid HP value %q.", tokens[1]))
		return
	}
	curePoison, cureDisease := parseFlagTokens(tokens[2:])
	result, err := h.combatService.LayOnHands(ctx, combat.LayOnHandsCommand{
		Paladin:     actor,
		Target:      *target,
		Turn:        turn,
		HP:          hp,
		CurePoison:  curePoison,
		CureDisease: cureDisease,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Lay on Hands failed: %v", err))
		return
	}
	h.respondAndLog(interaction, encounterID, result.CombatLog)
}

// dispatchStabilize wires /action stabilize <target> (C-43 / Phase 43).
// Requires the target to be a dying combatant within 5ft of the actor.
// Rolls Medicine (1d20 + Wisdom modifier per the helper's contract; here
// we use the actor's display name and a flat d20 since the WIS modifier
// is not exposed through ActionCombatService). On a result ≥ DC 10 the
// target is stabilized via combat.StabilizeTarget and the resulting death
// saves are persisted via the wired StabilizeStore. On failure no state
// changes; the target keeps rolling death saves.
func (h *ActionHandler) dispatchStabilize(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounterID uuid.UUID,
	actor refdata.Combatant,
	combatants []refdata.Combatant,
	args string,
) {
	if h.roller == nil {
		respondEphemeral(h.session, interaction, "Stabilize is not available right now (no dice roller wired).")
		return
	}
	if h.stabilizeStore == nil {
		respondEphemeral(h.session, interaction, "Stabilize is not available right now (no persistence wired).")
		return
	}
	tokens := strings.Fields(args)
	if len(tokens) == 0 {
		respondEphemeral(h.session, interaction, "Stabilize requires `<target>` (e.g. `/action stabilize args:AR`).")
		return
	}
	target, err := combat.ResolveTarget(tokens[0], combatants)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Target %q not found.", tokens[0]))
		return
	}

	// Reach check (5ft Chebyshev between grid tiles).
	if dist := stabilizeReachFt(actor, *target); dist > 5 {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%s is %dft away — stabilize requires reach (5ft).", target.DisplayName, dist))
		return
	}

	// Target must be dying — alive at 0 HP, not yet stabilized.
	ds, dsErr := combat.ParseDeathSaves(target.DeathSaves.RawMessage)
	if dsErr != nil {
		respondEphemeral(h.session, interaction, "Failed to read target death-save state.")
		return
	}
	if !combat.IsDying(target.IsAlive, int(target.HpCurrent), ds) {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%s is not dying — stabilize requires a target at 0 HP making death saves.", target.DisplayName))
		return
	}

	rollResult, rollErr := h.roller.Roll("1d20")
	if rollErr != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to roll Medicine check: %v", rollErr))
		return
	}
	// C-43-stabilize-followup: PHB Medicine is d20 + WIS_mod + (prof if
	// trained). When a lookup is wired, resolve the actor's modifier;
	// fall back to +0 (the legacy flat-d20 path) when not.
	medicineMod := h.resolveMedicineModifier(ctx, actor)
	total := rollResult.Total + medicineMod
	const stabilizeDC = 10
	if total < stabilizeDC {
		msg := fmt.Sprintf("🩹 %s attempts to stabilize %s — rolled %d (d20=%d %s) vs DC %d, fails.",
			actor.DisplayName, target.DisplayName, total, rollResult.Total, formatSignedMod(medicineMod), stabilizeDC)
		h.respondAndLog(interaction, encounterID, msg)
		return
	}

	outcome := combat.StabilizeTarget(target.DisplayName, ds, fmt.Sprintf("%s's Medicine check (%d = d20:%d %s vs DC %d)", actor.DisplayName, total, rollResult.Total, formatSignedMod(medicineMod), stabilizeDC))
	if _, err := h.stabilizeStore.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         target.ID,
		DeathSaves: combat.MarshalDeathSaves(outcome.DeathSaves),
	}); err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to persist stabilize: %v", err))
		return
	}

	logLine := joinMessages(outcome.Messages)
	h.respondAndLog(interaction, encounterID, logLine)
}

// resolveMedicineModifier returns the actor's Medicine-check modifier
// (WIS mod + proficiency when proficient + expertise when applicable).
// Falls back to 0 when no lookup is wired or the lookup errors out so
// /action stabilize always produces a usable roll. (C-43-stabilize-followup)
func (h *ActionHandler) resolveMedicineModifier(ctx context.Context, actor refdata.Combatant) int {
	if h.medicineLookup == nil {
		return 0
	}
	mod, err := h.medicineLookup.LookupMedicineModifier(ctx, actor)
	if err != nil {
		return 0
	}
	return mod
}

// formatSignedMod renders a modifier as " +N" / " -N" / " +0" for inline
// inclusion in roll breakdown strings.
func formatSignedMod(mod int) string {
	if mod >= 0 {
		return fmt.Sprintf("+%d", mod)
	}
	return fmt.Sprintf("%d", mod)
}

// stabilizeReachFt returns the Chebyshev (5-ft grid) distance between two
// combatants. Errors decoding either position degrade to a far-away value
// (the caller treats that as out-of-reach).
func stabilizeReachFt(a, b refdata.Combatant) int {
	aCol, aRow, aErr := renderer.ParseCoordinate(a.PositionCol + strconv.Itoa(int(a.PositionRow)))
	bCol, bRow, bErr := renderer.ParseCoordinate(b.PositionCol + strconv.Itoa(int(b.PositionRow)))
	if aErr != nil || bErr != nil {
		return 999
	}
	dc := aCol - bCol
	if dc < 0 {
		dc = -dc
	}
	dr := aRow - bRow
	if dr < 0 {
		dr = -dr
	}
	if dc > dr {
		return dc * 5
	}
	return dr * 5
}

// filterHostiles returns the alive combatants on the opposite faction from
// the actor (used by Hide for the highest-passive-Perception scan).
func filterHostiles(all []refdata.Combatant, actor refdata.Combatant) []refdata.Combatant {
	var out []refdata.Combatant
	for _, c := range all {
		if c.ID == actor.ID || !c.IsAlive {
			continue
		}
		if c.IsNpc == actor.IsNpc {
			continue
		}
		out = append(out, c)
	}
	return out
}

