package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// mockEquipStore captures inventory updates for equip tests.
type mockEquipStore struct {
	updatedInventory json.RawMessage
	char             refdata.Character
	err              error
}

func (m *mockEquipStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	if m.err != nil {
		return refdata.Character{}, m.err
	}
	m.updatedInventory = arg.Inventory.RawMessage
	return m.char, nil
}

func makeEquipInteraction(guildID, userID, itemID string, offhand, armor bool) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
	}
	if offhand {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "offhand", Type: discordgo.ApplicationCommandOptionBoolean, Value: true,
		})
	}
	if armor {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "armor", Type: discordgo.ApplicationCommandOptionBoolean, Value: true,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "equip",
			Options: opts,
		},
	}
}

func TestEquipHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Longsword")
	assert.Contains(t, sess.lastResponse, "Equipped")

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.updatedInventory, &updatedItems))
	assert.True(t, updatedItems[0].Equipped)
	assert.Equal(t, "main_hand", updatedItems[0].EquipSlot)
}

func TestEquipHandler_UnattunedWarning(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeEquipInteraction("guild1", "user1", "cloak-of-protection", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Equipped")
	assert.Contains(t, sess.lastResponse, "requires attunement")
	assert.Contains(t, sess.lastResponse, "/attune")
}

func TestEquipHandler_AttunedNoWarning(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeEquipInteraction("guild1", "user1", "cloak-of-protection", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Equipped")
	assert.NotContains(t, sess.lastResponse, "requires attunement")
}

func TestEquipHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestEquipHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestEquipHandler_EmptyItemID(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{},
		&mockInventoryCharacterLookup{},
		&mockEquipStore{},
	)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "equip",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "specify an item")
}

func TestEquipHandler_BadInventoryJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read inventory")
}

func TestEquipHandler_BadAttunementJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	itemsJSON, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read attunement")
}

func TestEquipHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	itemsJSON, _ := json.Marshal([]character.InventoryItem{})
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not found")
}

func TestEquipHandler_StoreError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockEquipStore{err: assert.AnError},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

// ===========================================================================
// SR-004: /equip routes through combat.Equip so columns + AC + validation
// are honored. The handler must also still maintain the inventory JSON flag
// for magic-item bonus readers (internal/magicitem/effects.go) and the
// portal character sheet.
// ===========================================================================

// mockEquipCombatService captures EquipCommand invocations and returns a
// canned EquipResult/err. Implements EquipCombatService.
type mockEquipCombatService struct {
	lastCmd    combat.EquipCommand
	result     combat.EquipResult
	err        error
	callCount  int
}

func (m *mockEquipCombatService) Equip(ctx context.Context, cmd combat.EquipCommand) (combat.EquipResult, error) {
	m.callCount++
	m.lastCmd = cmd
	if m.err != nil {
		return combat.EquipResult{}, m.err
	}
	return m.result, nil
}

// mockEquipEncounterProvider satisfies the in-combat lookup interface.
// Returns an active encounter when activeErr is nil; the turn returned is
// preset via `turn`.
type mockEquipEncounterProvider struct {
	encounterID uuid.UUID
	activeErr   error
	encounter   refdata.Encounter
	encErr      error
	turn        refdata.Turn
	turnErr     error
}

func (m *mockEquipEncounterProvider) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if m.activeErr != nil {
		return uuid.Nil, m.activeErr
	}
	return m.encounterID, nil
}

func (m *mockEquipEncounterProvider) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.encounter, m.encErr
}

func (m *mockEquipEncounterProvider) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.turn, m.turnErr
}

// makeSR004Setup builds a baseline campaign/character/inventory used by the
// SR-004 wiring tests. The character has a longsword + shield + chain-mail
// in inventory and DEX 14 (+2).
func makeSR004Setup() (*mockInventorySession, refdata.Campaign, refdata.Character, *mockEquipStore) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
		{ItemID: "greatsword", Name: "Greatsword", Quantity: 1, Type: "weapon"},
		{ItemID: "shield", Name: "Shield", Quantity: 1, Type: "armor"},
		{ItemID: "chain-mail", Name: "Chain Mail", Quantity: 1, Type: "armor"},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	char := refdata.Character{
		ID:              charID,
		CampaignID:      campID,
		Name:            "Aria",
		Ac:              12,
		AbilityScores:   json.RawMessage(`{"str":12,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
		Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}
	return sess, refdata.Campaign{ID: campID}, char, &mockEquipStore{char: char}
}

// TestEquipHandler_SR004_RoutesThroughCombatEquip — /equip routes through
// combat.Equip when wired so the EquippedMainHand column is written via
// EquipCommand.Character + result.Character. Before SR-004 the wired path
// only flipped the inventory JSON flag.
func TestEquipHandler_SR004_RoutesThroughCombatEquip(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()

	updated := char
	updated.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: updated,
			CombatLog: "⚔️ Aria equips Longsword (main hand)",
		},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError}) // out-of-combat

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	require.Equal(t, 1, combatSvc.callCount, "combat.Equip should be invoked")
	assert.Equal(t, "longsword", combatSvc.lastCmd.ItemName)
	assert.False(t, combatSvc.lastCmd.Offhand)
	assert.False(t, combatSvc.lastCmd.Armor)
	assert.Nil(t, combatSvc.lastCmd.Turn, "no active encounter -> Turn nil")
	assert.Contains(t, sess.lastResponse, "Longsword")

	// Handler must STILL update the inventory JSON flag so magic-item
	// effects + portal display continue to work.
	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.updatedInventory, &updatedItems))
	var ls character.InventoryItem
	for _, it := range updatedItems {
		if it.ItemID == "longsword" {
			ls = it
		}
	}
	assert.True(t, ls.Equipped)
	assert.Equal(t, "main_hand", ls.EquipSlot)
}

// TestEquipHandler_SR004_TwoHandedWithShield_Rejected — combat.Equip rejects
// two-handed weapons when off-hand is occupied. The handler must surface the
// error to the user instead of falling through to a JSON-flag flip.
func TestEquipHandler_SR004_TwoHandedWithShield_Rejected(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}

	combatSvc := &mockEquipCombatService{
		err: fmt.Errorf("cannot equip Greatsword — off-hand must be free for two-handed weapons"),
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError})

	handler.Handle(makeEquipInteraction("g", "u", "greatsword", false, false))

	assert.Equal(t, 1, combatSvc.callCount)
	assert.Contains(t, sess.lastResponse, "off-hand must be free for two-handed")
	assert.Empty(t, store.updatedInventory, "inventory must not be written when combat.Equip rejects")
}

// TestEquipHandler_SR004_ArmorInCombat_Rejected — combat.Equip refuses armor
// changes during combat. The handler must pass the active turn to combat.Equip
// so that gating fires, and must surface the spec'd error.
func TestEquipHandler_SR004_ArmorInCombat_Rejected(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()
	encID := uuid.New()
	turnID := uuid.New()

	combatSvc := &mockEquipCombatService{
		err: fmt.Errorf("You can't don or doff armor during combat."),
	}

	turn := refdata.Turn{ID: turnID, EncounterID: encID}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{
		encounterID: encID,
		encounter: refdata.Encounter{
			ID:            encID,
			Status:        "active",
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turn: turn,
	})

	handler.Handle(makeEquipInteraction("g", "u", "chain-mail", false, true))

	assert.Equal(t, 1, combatSvc.callCount)
	require.NotNil(t, combatSvc.lastCmd.Turn, "in-combat: Turn must be passed to combat.Equip")
	assert.Equal(t, turnID, combatSvc.lastCmd.Turn.ID)
	assert.True(t, combatSvc.lastCmd.Armor)
	assert.Contains(t, sess.lastResponse, "can't don or doff armor during combat")
}

// TestEquipHandler_SR004_NoneUnequip_ClearsInventoryFlag — `/equip none`
// must clear the matching main_hand item's `Equipped` flag so the JSON
// view stays consistent with the column write performed by combat.Equip.
func TestEquipHandler_SR004_NoneUnequip_ClearsInventoryFlag(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()

	// Pre-condition: longsword is currently flagged equipped in inventory.
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Equipped: true, EquipSlot: "main_hand"},
	}
	itemsJSON, _ := json.Marshal(items)
	char.Inventory = pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true}

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: char,
			CombatLog: "⚔️ Aria unequips main hand (unarmed strike)",
		},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError})

	handler.Handle(makeEquipInteraction("g", "u", "none", false, false))

	assert.Equal(t, 1, combatSvc.callCount)
	assert.Equal(t, "none", combatSvc.lastCmd.ItemName)

	var got []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.updatedInventory, &got))
	assert.False(t, got[0].Equipped, "longsword's Equipped flag must be cleared")
	assert.Equal(t, "", got[0].EquipSlot)
}

// TestEquipHandler_SR004_InventorySaveError — when UpdateCharacterInventory
// errors during the combat-path JSON mirror write, the user sees the save
// error (not a stale success message).
func TestEquipHandler_SR004_InventorySaveError(t *testing.T) {
	sess, camp, char, _ := makeSR004Setup()

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: char,
			CombatLog: "⚔️ Aria equips Longsword (main hand)",
		},
	}

	failingStore := &mockEquipStore{err: assert.AnError}
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		failingStore,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError})

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

// TestEquipHandler_SR004_AttunementWarning_SurfacedViaCombat — magic items
// that require attunement must still surface the /attune hint when routed
// through combat.Equip (not just the legacy inventory.Equip path).
func TestEquipHandler_SR004_AttunementWarning_SurfacedViaCombat(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	char := refdata.Character{
		ID:              charID,
		CampaignID:      campID,
		Name:            "Aria",
		Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: char,
			CombatLog: "⚔️ Aria equips Cloak of Protection",
		},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: char},
		&mockEquipStore{char: char},
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError})

	handler.Handle(makeEquipInteraction("g", "u", "cloak-of-protection", false, false))

	assert.Contains(t, sess.lastResponse, "requires attunement")
	assert.Contains(t, sess.lastResponse, "/attune")
}

// TestEquipHandler_SR004_NilEncounterProvider_TreatsAsOutOfCombat —
// when no encounter provider is wired, the handler must pass Turn=nil so
// combat.Equip runs the out-of-combat code path.
func TestEquipHandler_SR004_NilEncounterProvider_TreatsAsOutOfCombat(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: char,
			CombatLog: "⚔️ Aria equips Longsword (main hand)",
		},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	// SetEncounterProvider intentionally NOT called.

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	require.Equal(t, 1, combatSvc.callCount)
	assert.Nil(t, combatSvc.lastCmd.Turn)
}

// TestEquipHandler_SR004_NoCurrentTurn_TreatsAsOutOfCombat — an encounter
// with no active turn (between rounds, or freshly-created) must be treated
// as out-of-combat. Turn must be nil.
func TestEquipHandler_SR004_NoCurrentTurn_TreatsAsOutOfCombat(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()
	encID := uuid.New()

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{Character: char, CombatLog: "ok"},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{
		encounterID: encID,
		encounter:   refdata.Encounter{ID: encID, CurrentTurnID: uuid.NullUUID{Valid: false}},
	})

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	require.Equal(t, 1, combatSvc.callCount)
	assert.Nil(t, combatSvc.lastCmd.Turn)
}

// TestEquipHandler_SR004_TurnLookupFails_TreatsAsOutOfCombat — when
// GetTurn fails we degrade to nil Turn rather than blocking the action.
func TestEquipHandler_SR004_TurnLookupFails_TreatsAsOutOfCombat(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()
	encID := uuid.New()
	turnID := uuid.New()

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{Character: char, CombatLog: "ok"},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{
		encounterID: encID,
		encounter: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turnErr: assert.AnError,
	})

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	require.Equal(t, 1, combatSvc.callCount)
	assert.Nil(t, combatSvc.lastCmd.Turn)
}

// TestEquipHandler_SR004_GetEncounterFails_TreatsAsOutOfCombat — when
// GetEncounter fails (e.g. encounter deleted between resolver hit and lookup),
// degrade to nil Turn.
func TestEquipHandler_SR004_GetEncounterFails_TreatsAsOutOfCombat(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{Character: char, CombatLog: "ok"},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{
		encounterID: uuid.New(),
		encErr:      assert.AnError,
	})

	handler.Handle(makeEquipInteraction("g", "u", "longsword", false, false))

	require.Equal(t, 1, combatSvc.callCount)
	assert.Nil(t, combatSvc.lastCmd.Turn)
}

// TestEquipHandler_SR004_ArmorOutOfCombat_ACRecalculated — the new
// EquipResult.NewAC must be surfaced. The handler should display the AC
// change (proxy for "recalc hook invoked"). combat.Equip already performs
// the recalc; the handler just needs to wire the result back.
func TestEquipHandler_SR004_ArmorOutOfCombat_ACRecalculated(t *testing.T) {
	sess, camp, char, store := makeSR004Setup()

	updated := char
	updated.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}
	updated.Ac = 16

	combatSvc := &mockEquipCombatService{
		result: combat.EquipResult{
			Character: updated,
			CombatLog: "🛡️ Aria dons Chain Mail (AC 12 → 16)",
			ACChanged: true,
			OldAC:     12,
			NewAC:     16,
		},
	}

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: camp},
		&mockInventoryCharacterLookup{char: char},
		store,
	)
	handler.SetCombatService(combatSvc)
	handler.SetEncounterProvider(&mockEquipEncounterProvider{activeErr: assert.AnError})

	handler.Handle(makeEquipInteraction("g", "u", "chain-mail", false, true))

	assert.Equal(t, 1, combatSvc.callCount)
	assert.True(t, combatSvc.lastCmd.Armor)
	// The user-facing message must show the AC recalc result.
	assert.Contains(t, sess.lastResponse, "Chain Mail")
	assert.Contains(t, sess.lastResponse, "16")
}
