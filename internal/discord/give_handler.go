package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// GiveCharacterStore persists inventory updates for both giver and receiver atomically.
type GiveCharacterStore interface {
	UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error
}

// GiveTargetResolver resolves a name or ID to a character in the campaign.
type GiveTargetResolver interface {
	ResolveTarget(ctx context.Context, campaignID uuid.UUID, nameOrID string) (refdata.Character, error)
}

// GiveCombatProvider provides combat state for adjacency/resource checks.
type GiveCombatProvider interface {
	GetCombatantsForGuild(ctx context.Context, guildID string) ([]refdata.Combatant, bool, error)
}

// GivePlayerLookup resolves a character to its player-character row so the
// receiver's Discord user ID can be used to DM them after a transfer (T25).
type GivePlayerLookup interface {
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
}

// GiveTurnProvider resolves the active turn for the invoking character and
// persists the per-turn resource flags after a successful /give. med-35:
// /give in combat costs the per-turn free object interaction. When no turn
// is active (out of combat), no cost is taken.
type GiveTurnProvider interface {
	GetActiveTurnForCharacter(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

// GiveHandler handles the /give slash command.
type GiveHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	targetResolver  GiveTargetResolver
	store           GiveCharacterStore
	combatProv      GiveCombatProvider
	turnProv        GiveTurnProvider
	cardUpdater     CardUpdater      // SR-007
	playerLookup    GivePlayerLookup // T25: receiver DM
}

// NewGiveHandler creates a new GiveHandler.
func NewGiveHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	targetResolver GiveTargetResolver,
	store GiveCharacterStore,
	combatProv GiveCombatProvider,
) *GiveHandler {
	return &GiveHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		targetResolver:  targetResolver,
		store:           store,
		combatProv:      combatProv,
	}
}

// SetTurnProvider wires the optional turn provider for combat-time
// /give cost validation (med-35: free object interaction). When unset,
// /give never deducts a turn resource.
func (h *GiveHandler) SetTurnProvider(p GiveTurnProvider) {
	h.turnProv = p
}

// SetCardUpdater wires the SR-007 character-card refresh callback.
// /give fires it for BOTH giver and receiver after the atomic inventory
// write because both characters' inventory state may surface on the card.
func (h *GiveHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// SetPlayerLookup wires the optional player-character lookup used by T25 to
// resolve the receiver's Discord user ID for the post-transfer DM. When unset,
// the receiver DM is skipped (nil-safe, crit-01c).
func (h *GiveHandler) SetPlayerLookup(p GivePlayerLookup) {
	h.playerLookup = p
}

// Handle processes the /give command interaction.
func (h *GiveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	var itemID, targetStr string
	for _, opt := range data.Options {
		switch opt.Name {
		case "item":
			itemID = opt.StringValue()
		case "target":
			targetStr = opt.StringValue()
		}
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	giver, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	giverItems, err := character.ParseInventoryItems(giver.Inventory.RawMessage, giver.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read inventory. Please contact the DM.")
		return
	}

	// med-35: when in combat, deduct the free object interaction. Reject
	// up front when the resource is already spent. Out-of-combat /give
	// carries no cost.
	turn, inCombat, costErr := h.lookupActiveTurn(ctx, interaction.GuildID, giver.ID)
	if costErr != nil {
		respondEphemeral(h.session, interaction, "Failed to check turn state. Please try again.")
		return
	}
	if inCombat {
		if err := combat.ValidateResource(turn, combat.ResourceFreeInteract); err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot give item: %v", err))
			return
		}
	}

	// Resolve target
	receiver, err := h.targetResolver.ResolveTarget(ctx, campaign.ID, targetStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Could not find target %q.", targetStr))
		return
	}

	receiverItems, err := character.ParseInventoryItems(receiver.Inventory.RawMessage, receiver.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read target inventory. Please contact the DM.")
		return
	}

	// T25: enforce adjacency when both parties are positioned combatants. Out
	// of combat (no positions) the range cannot be enforced, so the give is
	// allowed. Returns an ephemeral error and does NOT transfer when too far.
	if dist, tooFar := h.rangeCheck(ctx, interaction.GuildID, giver.ID, receiver.ID); tooFar {
		respondEphemeral(h.session, interaction, fmt.Sprintf(
			"❌ %s is too far away — %dft away (you must be adjacent to hand over an item).",
			receiver.Name, dist))
		return
	}

	result, err := inventory.GiveItem(inventory.GiveInput{
		GiverItems:    giverItems,
		ReceiverItems: receiverItems,
		ItemID:        itemID,
		GiverName:     giver.Name,
		ReceiverName:  receiver.Name,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot give item: %v", err))
		return
	}

	// Persist both inventories atomically
	giverInvJSON, err := character.MarshalInventory(result.UpdatedGiverItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}
	receiverInvJSON, err := character.MarshalInventory(result.UpdatedReceiverItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	if err := h.store.UpdateTwoCharacterInventories(ctx, giver.ID,
		pqtype.NullRawMessage{RawMessage: giverInvJSON, Valid: true},
		receiver.ID,
		pqtype.NullRawMessage{RawMessage: receiverInvJSON, Valid: true},
	); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	// SR-007: refresh #character-cards for both parties after the transfer.
	notifyCardUpdate(ctx, h.cardUpdater, giver.ID)
	notifyCardUpdate(ctx, h.cardUpdater, receiver.ID)

	// T25: announce the transfer publicly and DM the receiver. Both are
	// best-effort — a failure here must never undo the committed transfer.
	itemName := giveItemName(giverItems, itemID)
	h.postTransferToStory(interaction.GuildID, giver.Name, receiver.Name, itemName)
	h.dmReceiver(ctx, campaign.ID, receiver.ID, giver.Name, itemName)

	// med-35: persist the free-interaction deduction. Best-effort: a
	// save failure does not undo the committed inventory transfer.
	if inCombat {
		h.spendFreeInteract(ctx, turn)
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// lookupActiveTurn resolves the active turn for the invoking character when a
// turn provider is wired. Returns inCombat=false when no provider is set or
// no active turn exists (out-of-combat /give).
func (h *GiveHandler) lookupActiveTurn(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error) {
	if h.turnProv == nil {
		return refdata.Turn{}, false, nil
	}
	turn, ok, err := h.turnProv.GetActiveTurnForCharacter(ctx, guildID, charID)
	if err != nil {
		return refdata.Turn{}, false, err
	}
	return turn, ok, nil
}

// spendFreeInteract marks the free-interaction resource as used and persists
// the change. Best-effort: failures are swallowed.
func (h *GiveHandler) spendFreeInteract(ctx context.Context, turn refdata.Turn) {
	updated, err := combat.UseResource(turn, combat.ResourceFreeInteract)
	if err != nil {
		return
	}
	_, _ = h.turnProv.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updated))
}

// rangeCheck returns the distance in feet and whether the receiver is out of
// reach (>5ft). T25: the check is only enforced when BOTH parties are
// positioned combatants in the same encounter. When either has no combatant or
// no position (pure out-of-combat trade), tooFar is false so the give proceeds.
func (h *GiveHandler) rangeCheck(ctx context.Context, guildID string, giverID, receiverID uuid.UUID) (int, bool) {
	if h.combatProv == nil {
		return 0, false
	}
	combatants, _, err := h.combatProv.GetCombatantsForGuild(ctx, guildID)
	if err != nil {
		return 0, false
	}
	giverC, okG := findCombatant(combatants, giverID)
	receiverC, okR := findCombatant(combatants, receiverID)
	if !okG || !okR {
		return 0, false
	}
	if giverC.PositionCol == "" || receiverC.PositionCol == "" {
		return 0, false
	}
	dist := combat.GridDistanceFt(
		giverC.PositionCol, int(giverC.PositionRow),
		receiverC.PositionCol, int(receiverC.PositionRow),
	)
	return dist, dist > 5
}

// findCombatant returns the combatant for the given character ID.
func findCombatant(combatants []refdata.Combatant, charID uuid.UUID) (refdata.Combatant, bool) {
	for _, c := range combatants {
		if c.CharacterID.Valid && c.CharacterID.UUID == charID {
			return c, true
		}
	}
	return refdata.Combatant{}, false
}

// postTransferToStory announces a transfer in #the-story (T25). Best-effort:
// an unresolvable channel or a send failure is silently ignored so the give
// still succeeds.
func (h *GiveHandler) postTransferToStory(guildID, giverName, receiverName, itemName string) {
	if guildID == "" {
		return
	}
	chID, err := resolveStoryChannel(h.session, guildID)
	if err != nil {
		return
	}
	msg := fmt.Sprintf("\U0001f381 **%s** gave **%s** to **%s**.", giverName, itemName, receiverName)
	_, _ = h.session.ChannelMessageSend(chID, msg)
}

// dmReceiver DMs the receiver that they got an item (T25). Best-effort: a
// missing player lookup, no Discord ID, or a send failure (DMs closed) is
// silently ignored so the give still succeeds.
func (h *GiveHandler) dmReceiver(ctx context.Context, campaignID, receiverID uuid.UUID, giverName, itemName string) {
	if h.playerLookup == nil {
		return
	}
	pc, err := h.playerLookup.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  campaignID,
		CharacterID: receiverID,
	})
	if err != nil || pc.DiscordUserID == "" {
		return
	}
	ch, err := h.session.UserChannelCreate(pc.DiscordUserID)
	if err != nil {
		return
	}
	msg := fmt.Sprintf("\U0001f381 You received **%s** from **%s**.", itemName, giverName)
	_, _ = h.session.ChannelMessageSend(ch.ID, msg)
}

// giveItemName resolves the display name of the given item from the giver's
// inventory, matching by ID or display name (case-insensitive). Falls back to
// the raw query when no match is found so messages always have a name.
func giveItemName(items []character.InventoryItem, query string) string {
	for _, item := range items {
		if item.ItemID == query || strings.EqualFold(item.Name, query) {
			return item.Name
		}
	}
	return query
}
