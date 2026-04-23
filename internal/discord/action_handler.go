package discord

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dmqueue"
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

// ActionHandler wires /action [freeform text] and /action cancel into the
// freeform-action service (combat) or directly onto the dm-queue notifier
// (exploration, where there is no turn / action economy).
type ActionHandler struct {
	session          Session
	resolver         ActionEncounterResolver
	combatService    ActionCombatService
	turnProvider     ActionTurnProvider
	campaignProvider ActionCampaignProvider
	characterLookup  ActionCharacterLookup
	pendingStore     ActionPendingStore
	notifier         dmqueue.Notifier
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

// Handle dispatches the /action slash command.
func (h *ActionHandler) Handle(interaction *discordgo.Interaction) {
	rawAction := strings.TrimSpace(optionString(interaction, "action"))
	rawArgs := strings.TrimSpace(optionString(interaction, "args"))

	if rawAction == "" {
		respondEphemeral(h.session, interaction, "Please provide an action (e.g. `/action flip the table`).")
		return
	}

	isCancel := strings.EqualFold(rawAction, "cancel") && rawArgs == ""

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

	h.handleCombat(ctx, interaction, encounter, userID, actionText, isCancel)
}

// handleCombat handles the combat-mode freeform post or cancel. Requires an
// active turn and verifies turn ownership against the invoker's character.
func (h *ActionHandler) handleCombat(
	ctx context.Context,
	interaction *discordgo.Interaction,
	encounter refdata.Encounter,
	userID, actionText string,
	isCancel bool,
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

	if isCancel {
		h.performCombatCancel(ctx, interaction, combatant, turn)
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
