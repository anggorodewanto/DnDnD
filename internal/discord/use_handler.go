package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// UseCharacterStore persists inventory/HP updates after using an item.
type UseCharacterStore interface {
	UpdateCharacterInventoryAndHP(ctx context.Context, arg refdata.UpdateCharacterInventoryAndHPParams) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

// UseCombatProvider provides combat state for action-cost validation.
// Implementations resolve the active turn for the invoking character (when in
// combat) and persist the per-turn resource flags after a successful /use.
// med-35: /use of a potion deducts a bonus action; magic-item active abilities
// deduct an action. When no turn is active (out of combat), no cost is taken.
//
// GetActiveCombatantForCharacter / UpdateCombatantHP exist because a character
// has two HP stores: `characters.hp_current` (the between-sessions sheet) and
// `combatants.hp_current` (the live combat token). Healing consumed mid-combat
// must land on the token — the sheet is not refreshed during a fight, so
// healing it both misses the real HP pool and silently clamps to zero against
// a full-looking sheet.
type UseCombatProvider interface {
	GetActiveTurnForCharacter(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
	GetActiveCombatantForCharacter(ctx context.Context, charID uuid.UUID) (refdata.Combatant, bool, error)
	UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
}

// UseMagicItemLookup resolves magic-item reference data so the use handler
// can read active_abilities and detect spell-casting items (Finding 8).
type UseMagicItemLookup interface {
	GetMagicItem(ctx context.Context, id string) (refdata.MagicItem, error)
}

// UseMagicItemSpellCaster routes a magic-item spell through the combat spell
// resolution path (Finding 8). Implementations delegate to combat.Service.Cast
// or combat.Service.CastAoE depending on the spell's area_of_effect.
type UseMagicItemSpellCaster interface {
	CastFromItem(ctx context.Context, input MagicItemCastInput) (MagicItemCastResult, error)
}

// MagicItemCastInput holds the parameters for casting a spell from a magic item.
type MagicItemCastInput struct {
	SpellID     string
	GuildID     string
	CharacterID uuid.UUID
	Charges     int // number of charges to spend (determines upcast level)
}

// MagicItemCastResult holds the result of casting a spell from a magic item.
type MagicItemCastResult struct {
	Message string
	Routed  bool // true if the spell was routed through combat resolution
}

// UseHandler handles the /use slash command.
type UseHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           UseCharacterStore
	invService      *inventory.Service
	combatProv      UseCombatProvider
	magicItemLookup UseMagicItemLookup      // Finding 8: active ability lookup
	spellCaster     UseMagicItemSpellCaster // Finding 8: spell resolution routing
	dmQueueFunc     func(guildID string) string
	notifier        dmqueue.Notifier
	cardUpdater     CardUpdater // SR-007
}

// NewUseHandler creates a new UseHandler.
func NewUseHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store UseCharacterStore,
	randFn dice.RandSource,
	combatProv UseCombatProvider,
) *UseHandler {
	return &UseHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
		invService:      inventory.NewService(randFn),
		combatProv:      combatProv,
	}
}

// SetDMQueueFunc sets the function that resolves a guild ID to a #dm-queue channel ID.
func (h *UseHandler) SetDMQueueFunc(fn func(guildID string) string) {
	h.dmQueueFunc = fn
}

// SetNotifier wires the dm-queue Notifier. When set, consumable-without-effect
// posts route through the unified dmqueue framework instead of the legacy
// dmQueueFunc path.
func (h *UseHandler) SetNotifier(n dmqueue.Notifier) {
	h.notifier = n
}

// SetCardUpdater wires the SR-007 character-card refresh callback. Fires
// after a successful /use write (consumable or magic-item charge).
func (h *UseHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// SetMagicItemLookup wires the magic-item reference lookup for reading
// active_abilities (Finding 8).
func (h *UseHandler) SetMagicItemLookup(l UseMagicItemLookup) {
	h.magicItemLookup = l
}

// SetSpellCaster wires the spell resolution adapter so magic items with
// spell_id route through combat spell resolution (Finding 8).
func (h *UseHandler) SetSpellCaster(c UseMagicItemSpellCaster) {
	h.spellCaster = c
}

// Handle processes the /use command interaction.
func (h *UseHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	itemID := ""
	for _, opt := range data.Options {
		if opt.Name == "item" {
			itemID = opt.StringValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to use.")
		return
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
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

	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read inventory. Please contact the DM.")
		return
	}

	// Both /use paths (consumable and magic-item charge) spend turn resources,
	// so the combat gate is resolved once here and shared. Keeping it above the
	// magic-item branch is what stops the two paths drifting apart.
	turn, inCombat, combatant, hasCombatant, ok := h.resolveCombatGate(ctx, interaction, char.ID)
	if !ok {
		return
	}

	// Magic items with charges short-circuit to the active-ability path.
	// Consumables fall through to UseConsumable below.
	if itemHasActiveCharges(items, itemID) {
		h.handleMagicItemCharge(ctx, interaction, char, items, itemID, turn, inCombat)
		return
	}

	if inCombat {
		if err := combat.ValidateResource(turn, useResourceCost(itemID)); err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
			return
		}
	}

	// Heal against whichever HP store is authoritative right now.
	hpCurrent, hpMax := int(char.HpCurrent), int(char.HpMax)
	if hasCombatant {
		hpCurrent, hpMax = int(combatant.HpCurrent), int(combatant.HpMax)
	}

	result, err := h.invService.UseConsumable(inventory.UseInput{
		Items:     items,
		ItemID:    itemID,
		HPCurrent: hpCurrent,
		HPMax:     hpMax,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
		return
	}

	// Persist changes
	invJSON, err := character.MarshalInventory(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	if err := h.persistUse(ctx, char, combatant, hasCombatant, result, invJSON); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	// SR-007: refresh #character-cards after a successful /use write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	// med-35: persist the turn-resource deduction. Best-effort: a save
	// failure is logged but does not undo the committed inventory change.
	if inCombat {
		h.spendTurnResource(ctx, turn, useResourceCost(itemID))
	}

	// Post to #dm-queue if item requires DM adjudication.
	if result.DMQueueRequired {
		usedItemName := itemID
		for _, it := range items {
			if it.ItemID == itemID {
				usedItemName = it.Name
				break
			}
		}
		h.postConsumableToDMQueue(ctx, interaction.GuildID, campaign.ID.String(), char.Name, usedItemName)
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// itemHasActiveCharges reports whether the inventory item with itemID is a
// magic item with non-zero MaxCharges (i.e. an active-ability target).
func itemHasActiveCharges(items []character.InventoryItem, itemID string) bool {
	for _, it := range items {
		if it.ItemID != itemID {
			continue
		}
		return it.IsMagic && it.MaxCharges > 0
	}
	return false
}

// handleMagicItemCharge consumes one charge from the named magic item and
// persists the updated inventory. It enforces attunement and charge-balance
// rules via inventory.UseCharges. The default amount is 1 charge. med-35:
// when in combat, an action is deducted from the active turn before the
// charge is spent.
//
// Finding 8: when the magic item has an active ability with a spell_id, the
// handler routes through the spell resolution path (UseMagicItemSpellCaster)
// instead of just deducting a charge silently.
func (h *UseHandler) handleMagicItemCharge(
	ctx context.Context,
	interaction *discordgo.Interaction,
	char refdata.Character,
	items []character.InventoryItem,
	itemID string,
	turn refdata.Turn,
	inCombat bool,
) {
	attunement, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	if inCombat {
		if err := combat.ValidateResource(turn, combat.ResourceAction); err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
			return
		}
	}

	// Finding 8: detect spell-casting active abilities.
	spellID, chargesCost := h.resolveSpellAbility(ctx, itemID)
	amount := max(chargesCost, 1)

	result, err := h.invService.UseCharges(inventory.UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     itemID,
		Amount:     amount,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
		return
	}

	invJSON, err := character.MarshalInventory(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	// SR-007: refresh #character-cards after a magic-item charge write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	if inCombat {
		h.spendTurnResource(ctx, turn, combat.ResourceAction)
	}

	// Finding 8: if the item casts a spell, route through spell resolution.
	if spellID != "" && h.spellCaster != nil && inCombat {
		castResult, err := h.spellCaster.CastFromItem(ctx, MagicItemCastInput{
			SpellID:     spellID,
			GuildID:     interaction.GuildID,
			CharacterID: char.ID,
			Charges:     amount,
		})
		if err == nil && castResult.Routed {
			respondEphemeral(h.session, interaction, fmt.Sprintf("%s\n%s", result.Message, castResult.Message))
			return
		}
	}

	// Fallback: report charge usage (and spell name if applicable).
	if spellID != "" {
		msg := fmt.Sprintf("%s\n🔮 Casts **%s** from the item.", result.Message, spellID)
		respondEphemeral(h.session, interaction, msg)
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// resolveSpellAbility reads the magic item's active_abilities from the ref
// table and returns the spell_id and charges_cost if a spell-casting ability
// is found. Returns ("", 0) when no lookup is wired or no spell ability exists.
func (h *UseHandler) resolveSpellAbility(ctx context.Context, itemID string) (string, int) {
	if h.magicItemLookup == nil {
		return "", 0
	}
	mi, err := h.magicItemLookup.GetMagicItem(ctx, itemID)
	if err != nil || !mi.ActiveAbilities.Valid {
		return "", 0
	}
	var abilities []struct {
		SpellID     string `json:"spell_id"`
		ChargesCost int    `json:"charges_cost"`
	}
	if err := json.Unmarshal(mi.ActiveAbilities.RawMessage, &abilities); err != nil {
		return "", 0
	}
	for _, a := range abilities {
		if a.SpellID != "" {
			cost := max(a.ChargesCost, 1)
			return a.SpellID, cost
		}
	}
	return "", 0
}

// lookupActiveTurn returns the active turn for the invoking character when a
// combat provider is wired. inCombat is false when no provider is configured
// or when the character has no active turn (out-of-combat /use). Errors from
// the provider are surfaced so callers can short-circuit.
func (h *UseHandler) lookupActiveTurn(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error) {
	if h.combatProv == nil {
		return refdata.Turn{}, false, nil
	}
	turn, ok, err := h.combatProv.GetActiveTurnForCharacter(ctx, guildID, charID)
	if err != nil {
		return refdata.Turn{}, false, err
	}
	return turn, ok, nil
}

// useResourceCost returns the turn resource a /use of itemID costs. Potions are
// a bonus action; everything else (magic items, DM-adjudicated consumables)
// costs an action.
func useResourceCost(itemID string) combat.ResourceType {
	if inventory.IsPotion(itemID) {
		return combat.ResourceBonusAction
	}
	return combat.ResourceAction
}

// lookupCombatant resolves the character's live combat token, if any. A
// missing provider or a character who is not in an active encounter both
// report (zero, false, nil) so callers fall back to the character sheet.
func (h *UseHandler) lookupCombatant(ctx context.Context, charID uuid.UUID) (refdata.Combatant, bool, error) {
	if h.combatProv == nil {
		return refdata.Combatant{}, false, nil
	}
	return h.combatProv.GetActiveCombatantForCharacter(ctx, charID)
}

// resolveCombatGate resolves the character's turn and combat token and applies
// the checks that must hold before any /use spends a resource. It responds to
// the interaction itself and reports ok=false when the use must not proceed.
//
// Out of combat every check is a no-op, so this reduces to a pair of lookups.
func (h *UseHandler) resolveCombatGate(
	ctx context.Context,
	interaction *discordgo.Interaction,
	charID uuid.UUID,
) (turn refdata.Turn, inCombat bool, combatant refdata.Combatant, hasCombatant bool, ok bool) {
	turn, inCombat, err := h.lookupActiveTurn(ctx, interaction.GuildID, charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to check turn state. Please try again.")
		return turn, false, combatant, false, false
	}

	// A failed lookup must not fall through to the character sheet: mid-combat
	// the sheet reads full HP, which silently swallows the healing entirely.
	combatant, hasCombatant, err = h.lookupCombatant(ctx, charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to check combat state. Please try again.")
		return turn, false, combatant, false, false
	}

	if !hasCombatant {
		return turn, inCombat, combatant, false, true
	}

	// The turn lookup returns whichever turn the encounter currently has, not
	// necessarily this character's. Using an item costs an action or bonus
	// action, so it is only legal on your own turn — without this guard the
	// cost is validated and spent against another combatant's turn, which for
	// an enemy NPC silently eats the DM's Action.
	if inCombat && turn.CombatantID != combatant.ID {
		respondEphemeral(h.session, interaction, "Cannot use item: it is not your turn.")
		return turn, inCombat, combatant, hasCombatant, false
	}

	// is_alive=false means actually dead (three failed death saves, or
	// exhaustion 6) — not merely dying. A corpse does not drink potions.
	if !combatant.IsAlive {
		respondEphemeral(h.session, interaction, "Cannot use item: you are dead.")
		return turn, inCombat, combatant, hasCombatant, false
	}

	// Incapacitated characters (Unconscious, Stunned, Paralyzed, ...) take no
	// actions, so they cannot use an item on themselves. This also stops a
	// dying character self-administering a potion, which would raise the token
	// above 0 HP while leaving them Unconscious with their death saves intact
	// — this path cannot reach combat.Service.MaybeResetDeathSavesOnHeal, so
	// it must not be allowed to revive anyone. Someone else has to pour it.
	if combat.IsIncapacitatedRaw(combatant.Conditions) {
		respondEphemeral(h.session, interaction, "Cannot use item: you cannot act right now.")
		return turn, inCombat, combatant, hasCombatant, false
	}

	return turn, inCombat, combatant, hasCombatant, true
}

// persistUse writes the post-use inventory and any healing. Inventory always
// belongs to the character sheet; healing goes to the live combat token when
// one exists and to the sheet otherwise, so a potion drunk mid-fight moves the
// HP the fight actually reads.
//
// The sheet path is a single atomic statement, but the combat path spans two
// tables and cannot be. The HP write therefore goes FIRST: if the second write
// fails the player keeps an un-consumed potion alongside a heal they already
// got, which is bounded by HpMax and visible to the DM. The reverse ordering
// reproduces the very bug this fixes — potion gone, HP unmoved — and a retry
// would cost a second potion.
//
// Known limit: UpdateCombatantHP is a full-column overwrite computed from a
// snapshot read earlier in the request, so a temp-HP grant or damage landing
// in that window is clobbered. Every heal call site in internal/combat shares
// this shape; narrowing it belongs with that query, not here.
func (h *UseHandler) persistUse(
	ctx context.Context,
	char refdata.Character,
	combatant refdata.Combatant,
	hasCombatant bool,
	result inventory.UseResult,
	invJSON []byte,
) error {
	invMsg := pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}

	if result.HealingDone > 0 && !hasCombatant {
		_, err := h.store.UpdateCharacterInventoryAndHP(ctx, refdata.UpdateCharacterInventoryAndHPParams{
			ID:        char.ID,
			Inventory: invMsg,
			HpCurrent: int32(result.HPAfter),
		})
		return err
	}

	if result.HealingDone > 0 && hasCombatant {
		// Healing never restores temp HP (it is a separate pool), so carry the
		// combatant's existing value through untouched. IsAlive is
		// unconditionally true: resolveCombatGate has already rejected dead and
		// incapacitated characters, so anyone reaching here was alive already.
		if _, err := h.combatProv.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        combatant.ID,
			HpCurrent: int32(result.HPAfter),
			TempHp:    combatant.TempHp,
			IsAlive:   true,
		}); err != nil {
			return err
		}
	}

	_, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: invMsg,
	})
	return err
}

// spendTurnResource marks resource as used on turn and persists the change.
// Best-effort: persistence failures are swallowed so an uncommitted turn
// flag never undoes a committed inventory mutation.
func (h *UseHandler) spendTurnResource(ctx context.Context, turn refdata.Turn, resource combat.ResourceType) {
	updated, err := combat.UseResource(turn, resource)
	if err != nil {
		return
	}
	_, _ = h.combatProv.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updated))
}

// postConsumableToDMQueue dispatches a consumable-without-effect notification
// either through the dmqueue Notifier (preferred) or the legacy dmQueueFunc
// fallback. Both paths produce content containing the player and item names.
// SR-002: CampaignID is required by PgStore.Insert.
func (h *UseHandler) postConsumableToDMQueue(ctx context.Context, guildID, campaignID, charName, itemName string) {
	if h.notifier != nil {
		_, _ = h.notifier.Post(ctx, dmqueue.Event{
			Kind:       dmqueue.KindConsumable,
			PlayerName: charName,
			Summary:    fmt.Sprintf("uses %s", itemName),
			GuildID:    guildID,
			CampaignID: campaignID,
		})
		return
	}
	if h.dmQueueFunc == nil {
		return
	}
	channelID := h.dmQueueFunc(guildID)
	if channelID == "" {
		return
	}
	event := dmqueue.Event{
		Kind:        dmqueue.KindConsumable,
		PlayerName:  charName,
		Summary:     fmt.Sprintf("uses %s", itemName),
		ResolvePath: "#", // legacy path has no dashboard item ID
	}
	_, _ = h.session.ChannelMessageSend(channelID, dmqueue.FormatEvent(event))
}
