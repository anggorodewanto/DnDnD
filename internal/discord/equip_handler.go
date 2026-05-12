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
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// EquipCharacterStore persists inventory updates for the equip command.
type EquipCharacterStore interface {
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

// EquipCombatService runs the full /equip flow: 2H/shield validation,
// in-combat armor block, AC recalc, and column writes for
// equipped_main_hand/equipped_off_hand/equipped_armor that downstream
// grapple/somatic/stealth/attack code reads. *combat.Service implements it.
type EquipCombatService interface {
	Equip(ctx context.Context, cmd combat.EquipCommand) (combat.EquipResult, error)
}

// EquipEncounterProvider resolves the invoker's active encounter (if any) so
// the handler can pass the current Turn to combat.Equip for resource-cost
// enforcement (free interact, action, armor-block).
type EquipEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// EquipHandler handles the /equip slash command.
type EquipHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           EquipCharacterStore
	// SR-004: optional collaborators. When combatSvc is wired the handler
	// routes through combat.Equip so the spec'd column writes + AC recalc
	// + 2H/shield/armor validation actually run. When nil, the handler
	// falls back to the inventory-JSON-only path (used by test deploys
	// without a combat service).
	combatSvc         EquipCombatService
	encounterProvider EquipEncounterProvider
}

// NewEquipHandler creates a new EquipHandler.
func NewEquipHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store EquipCharacterStore,
) *EquipHandler {
	return &EquipHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
	}
}

// SetCombatService wires the combat.Equip service so /equip applies the
// spec's full validation + AC recalc + column writes (SR-004).
func (h *EquipHandler) SetCombatService(svc EquipCombatService) {
	h.combatSvc = svc
}

// SetEncounterProvider wires the active-encounter + turn lookup so the
// handler can pass the current Turn to combat.Equip for in-combat gating.
func (h *EquipHandler) SetEncounterProvider(p EquipEncounterProvider) {
	h.encounterProvider = p
}

// Handle processes the /equip command interaction.
func (h *EquipHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	var itemID string
	var offhand, armor bool
	for _, opt := range data.Options {
		switch opt.Name {
		case "item":
			itemID = opt.StringValue()
		case "offhand":
			offhand = opt.BoolValue()
		case "armor":
			armor = opt.BoolValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to equip.")
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

	slots, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	// SR-004: when the combat service is wired, route through combat.Equip
	// so the spec'd validations and column writes happen. Otherwise fall
	// back to the inventory-JSON-only path (legacy test deploys).
	if h.combatSvc != nil {
		h.handleViaCombat(ctx, interaction, char, items, slots, itemID, offhand, armor)
		return
	}

	h.handleViaInventoryOnly(ctx, interaction, char, items, slots, itemID, offhand, armor)
}

// handleViaCombat is the SR-004 path: run combat.Equip (validates 2H/shield,
// blocks armor-in-combat, recalculates AC, writes columns) then ALSO update
// the inventory JSON flag so magic-item bonuses + portal display continue to
// read the right state.
func (h *EquipHandler) handleViaCombat(
	ctx context.Context,
	interaction *discordgo.Interaction,
	char refdata.Character,
	items []character.InventoryItem,
	slots []character.AttunementSlot,
	itemID string,
	offhand, armor bool,
) {
	turn := h.lookupActiveTurn(ctx, interaction.GuildID, discordUserID(interaction))

	cmd := combat.EquipCommand{
		Character: char,
		Turn:      turn,
		ItemName:  itemID,
		Offhand:   offhand,
		Armor:     armor,
	}
	result, err := h.combatSvc.Equip(ctx, cmd)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%v", err))
		return
	}

	// Mirror the equip into the inventory JSON so /inventory display +
	// magic-item bonuses (internal/magicitem/effects.go) + portal sheet
	// stay in sync. combat.Equip only writes the columns.
	if err := h.mirrorInventoryJSON(ctx, char.ID, items, itemID, offhand, armor); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save equipment changes. Please try again.")
		return
	}

	msg := result.CombatLog
	if msg == "" {
		msg = fmt.Sprintf("Equipped %s.", itemID)
	}
	// Surface attunement warning the same way the legacy path did so the
	// "use /attune" hint isn't lost when routing through combat.Equip.
	if warning := attunementWarning(items, slots, itemID); warning != "" {
		msg += "\n" + warning
	}
	respondEphemeral(h.session, interaction, msg)
}

// mirrorInventoryJSON persists the inventory JSON `Equipped`/`EquipSlot`
// flip that mirrors the column write performed by combat.Equip. When the
// equipped/unequipped item isn't tracked in inventory JSON (or json.Marshal
// fails) we skip the write rather than failing the operation — the column
// write already happened in combat.Equip and is the authoritative state.
// Only an UpdateCharacterInventory DB error propagates.
func (h *EquipHandler) mirrorInventoryJSON(
	ctx context.Context,
	charID uuid.UUID,
	items []character.InventoryItem,
	itemID string,
	offhand, armor bool,
) error {
	updatedItems, ok := mirrorInventoryFlag(items, itemID, offhand, armor)
	if !ok {
		return nil
	}
	invJSON, err := json.Marshal(updatedItems)
	if err != nil {
		return nil
	}
	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        charID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		return err
	}
	return nil
}

// lookupActiveTurn resolves the invoker's active encounter and current turn.
// Returns nil when the user has no active encounter, the encounter has no
// current turn, or any lookup fails — i.e. "treat as out-of-combat" so
// combat.Equip skips resource gating.
func (h *EquipHandler) lookupActiveTurn(ctx context.Context, guildID, userID string) *refdata.Turn {
	if h.encounterProvider == nil {
		return nil
	}
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, guildID, userID)
	if err != nil {
		return nil
	}
	enc, err := h.encounterProvider.GetEncounter(ctx, encounterID)
	if err != nil {
		return nil
	}
	if !enc.CurrentTurnID.Valid {
		return nil
	}
	turn, err := h.encounterProvider.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		return nil
	}
	return &turn
}

// handleViaInventoryOnly is the legacy fallback used when no combat service
// is wired (test deploys). It flips the inventory JSON flag only — column
// writes + AC recalc + validation are NOT performed. This branch exists for
// backward compatibility with handler test fixtures predating SR-004.
func (h *EquipHandler) handleViaInventoryOnly(
	ctx context.Context,
	interaction *discordgo.Interaction,
	char refdata.Character,
	items []character.InventoryItem,
	slots []character.AttunementSlot,
	itemID string,
	offhand, armor bool,
) {
	result, err := inventory.Equip(inventory.EquipInput{
		Items:           items,
		ItemID:          itemID,
		OffHand:         offhand,
		Armor:           armor,
		AttunementSlots: slots,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%v", err))
		return
	}

	invJSON, err := json.Marshal(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save equipment changes. Please try again.")
		return
	}

	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save equipment changes. Please try again.")
		return
	}

	msg := result.Message
	if result.Warning != "" {
		msg += "\n" + result.Warning
	}
	respondEphemeral(h.session, interaction, msg)
}

// mirrorInventoryFlag sets Equipped/EquipSlot on the matching inventory
// item so the JSONB tracker stays in sync with the column write performed
// by combat.Equip. Returns the new slice + ok=true when the item was found.
// itemName "none" is the unequip variant — when ok=false the caller should
// skip the JSONB write entirely (combat.Equip already cleared the column,
// and there is no inventory row to flip for "none").
func mirrorInventoryFlag(items []character.InventoryItem, itemID string, offhand, armor bool) ([]character.InventoryItem, bool) {
	if itemID == "none" {
		// Clear inventory equip flags for the slot we just unequipped.
		// "none" + offhand=true clears off-hand items; armor=true clears
		// armor items; default clears main-hand items.
		slot := equipSlotFor(offhand, armor)
		updated := make([]character.InventoryItem, len(items))
		copy(updated, items)
		changed := false
		for i := range updated {
			if updated[i].Equipped && updated[i].EquipSlot == slot {
				updated[i].Equipped = false
				updated[i].EquipSlot = ""
				changed = true
			}
		}
		return updated, changed
	}

	idx := -1
	for i := range items {
		if items[i].ItemID == itemID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, false
	}

	slot := equipSlotFor(offhand, armor)
	updated := make([]character.InventoryItem, len(items))
	copy(updated, items)
	updated[idx].Equipped = true
	updated[idx].EquipSlot = slot
	return updated, true
}

// equipSlotFor returns the inventory JSON `equip_slot` string for the given
// offhand/armor flags. The mapping matches the legacy inventory.Equip path
// (main_hand / off_hand / armor) so existing readers don't need migration.
func equipSlotFor(offhand, armor bool) string {
	if armor {
		return "armor"
	}
	if offhand {
		return "off_hand"
	}
	return "main_hand"
}

// attunementWarning returns the "requires attunement" hint when the equipped
// item requires attunement and isn't currently attuned. Returns "" otherwise.
func attunementWarning(items []character.InventoryItem, slots []character.AttunementSlot, itemID string) string {
	if itemID == "none" {
		return ""
	}
	var item character.InventoryItem
	found := false
	for _, it := range items {
		if it.ItemID == itemID {
			item = it
			found = true
			break
		}
	}
	if !found || !item.RequiresAttunement {
		return ""
	}
	for _, s := range slots {
		if s.ItemID == itemID {
			return ""
		}
	}
	return fmt.Sprintf("⚠️ This item requires attunement. Use `/attune %s` during a short rest to activate its properties.", item.Name)
}
