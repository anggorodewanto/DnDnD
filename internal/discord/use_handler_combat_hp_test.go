package discord

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// A character sheet and its live combat token keep separate HP (see
// project_two_hp_stores_overlay): `characters.hp_current` is the between-
// sessions store, `combatants.hp_current` is what the fight actually reads.
// `/use` healed the sheet, so drinking a potion mid-combat healed the wrong
// number — and because the sheet still sat at full HP the clamp
// `min(roll, HPMax-HPCurrent)` zeroed the healing outright, consuming the
// potion and the bonus action for nothing. These tests pin the potion to the
// combatant whenever one is live.

// useCombatHPFixture builds a /use handler whose character sheet is at full
// HP while its combat token is wounded — the exact split that hid the bug.
type useCombatHPFixture struct {
	sess       *mockInventorySession
	store      *mockUseCharacterStore
	combatProv *mockUseCombatProvider
	handler    *UseHandler
	charID     uuid.UUID
}

func newUseCombatHPFixture(t *testing.T, combatant refdata.Combatant, turn refdata.Turn, inCombat bool) *useCombatHPFixture {
	t.Helper()

	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	itemsJSON, err := json.Marshal(items)
	assert.NoError(t, err)

	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}
	combatProv := &mockUseCombatProvider{
		turn:         turn,
		inCombat:     inCombat,
		combatant:    combatant,
		hasCombatant: combatant.ID != uuid.Nil,
	}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         charID,
			CampaignID: campID,
			Name:       "Vale",
			HpCurrent:  31, // sheet reads full health...
			HpMax:      31, // ...and a DIFFERENT max from the token below, so
			//                 tests can tell the two HP stores apart.
			Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		func(max int) int { return 3 }, // 2d4+2 -> 3+3+2 = 8
		combatProv,
	)

	return &useCombatHPFixture{sess: sess, store: store, combatProv: combatProv, handler: handler, charID: charID}
}

func TestUseHandler_PotionInCombat_HealsCombatantNotSheet(t *testing.T) {
	combatantID := uuid.New()
	turnID := uuid.New()
	fx := newUseCombatHPFixture(t,
		// Token max (24) deliberately differs from the sheet max (31) so both
		// halves of the store selection — current AND max — are pinned.
		refdata.Combatant{ID: combatantID, HpCurrent: 4, HpMax: 24, TempHp: 5, IsAlive: true},
		refdata.Turn{ID: turnID, CombatantID: combatantID},
		true,
	)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	if assert.Len(t, fx.combatProv.hpUpdates, 1, "potion must write the combat token") {
		assert.Equal(t, combatantID, fx.combatProv.hpUpdates[0].ID)
		assert.Equal(t, int32(12), fx.combatProv.hpUpdates[0].HpCurrent, "4 + 8 healed")
		assert.True(t, fx.combatProv.hpUpdates[0].IsAlive)
		assert.Equal(t, int32(5), fx.combatProv.hpUpdates[0].TempHp, "healing must not disturb the separate temp-HP pool")
	}
	assert.Zero(t, fx.store.updatedHPCurrent, "character-sheet HP must not be rewritten mid-combat")
	assert.NotEmpty(t, fx.store.updatedInventory, "the potion must still be consumed from the sheet inventory")
	assert.Contains(t, fx.sess.lastResponse, "12/24", "player must be told their real combat HP and the token's max")
}

func TestUseHandler_PotionInCombat_ClampsToCombatantMax(t *testing.T) {
	combatantID := uuid.New()
	turnID := uuid.New()
	fx := newUseCombatHPFixture(t,
		// 20 + 8 rolled would be 28, past the token's max of 24 but still
		// under the sheet's 31 — so only clamping to the token passes.
		refdata.Combatant{ID: combatantID, HpCurrent: 20, HpMax: 24, IsAlive: true},
		refdata.Turn{ID: turnID, CombatantID: combatantID},
		true,
	)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	if assert.Len(t, fx.combatProv.hpUpdates, 1) {
		assert.Equal(t, int32(24), fx.combatProv.hpUpdates[0].HpCurrent, "healing clamps to the combatant's max, not the sheet's")
	}
}

// is_alive=false means three failed death saves, not merely dying. A potion
// must not undo that without the DM.
func TestUseHandler_PotionWhileDead_Rejected(t *testing.T) {
	combatantID := uuid.New()
	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: combatantID, HpCurrent: 0, HpMax: 24, IsAlive: false},
		refdata.Turn{ID: uuid.New(), CombatantID: combatantID},
		true,
	)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "dead")
	assert.Empty(t, fx.combatProv.hpUpdates, "a potion must not resurrect a dead combatant")
	assert.Empty(t, fx.store.updatedInventory)
}

// A consumable with no auto-resolved healing still consumes the item and goes
// to the DM queue — but must not touch the combat token's HP.
func TestUseHandler_NonHealingConsumableInCombat_ConsumesWithoutHPWrite(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "mystery-draught", Name: "Mystery Draught", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, err := json.Marshal(items)
	assert.NoError(t, err)

	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}
	combatProv := &mockUseCombatProvider{
		turn:         refdata.Turn{ID: turnID, CombatantID: combatantID},
		inCombat:     true,
		combatant:    refdata.Combatant{ID: combatantID, HpCurrent: 4, HpMax: 24, IsAlive: true},
		hasCombatant: true,
	}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         charID,
			CampaignID: campID,
			Name:       "Vale",
			HpCurrent:  31,
			HpMax:      31,
			Inventory:  pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		func(max int) int { return 3 },
		combatProv,
	)

	handler.Handle(makeUseInteraction("guild1", "user1", "mystery-draught"))

	assert.NotEmpty(t, store.updatedInventory, "the draught is still consumed")
	assert.Empty(t, combatProv.hpUpdates, "a non-healing consumable must not write HP")
	assert.Zero(t, store.updatedHPCurrent, "nor sheet HP")
}

// A potion is a Bonus Action, so it is only legal on your own turn. The turn
// lookup returns the encounter's *current* turn regardless of whose it is, so
// without an ownership check an off-turn /use validated and spent the active
// combatant's economy — potentially an enemy's.
func TestUseHandler_PotionOffTurn_RejectedWithoutTouchingAnotherTurn(t *testing.T) {
	combatantID := uuid.New()
	someoneElsesTurn := refdata.Turn{ID: uuid.New(), CombatantID: uuid.New()}
	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: combatantID, HpCurrent: 4, HpMax: 31, IsAlive: true},
		someoneElsesTurn,
		true,
	)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "your turn")
	assert.NotZero(t, fx.sess.lastFlags&discordgo.MessageFlagsEphemeral,
		"an off-turn rejection must stay ephemeral")
	assert.Empty(t, fx.combatProv.spends, "must not spend another combatant's action economy")
	assert.Empty(t, fx.combatProv.hpUpdates, "must not heal on a turn that is not yours")
	assert.Empty(t, fx.store.updatedInventory, "a rejected /use must not consume the potion")
}

// A downed or otherwise incapacitated character cannot take actions at all, so
// they cannot drink their own potion. This matters more now that healing writes
// the combat token: without the guard a self-/use would raise a dying
// combatant's HP above 0 while leaving them Unconscious with their death-save
// tally intact.
func TestUseHandler_PotionWhileIncapacitated_Rejected(t *testing.T) {
	combatantID := uuid.New()
	turnID := uuid.New()
	conditions, err := json.Marshal([]map[string]any{{"condition": "unconscious"}})
	assert.NoError(t, err)

	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: combatantID, HpCurrent: 0, HpMax: 31, IsAlive: true, Conditions: conditions},
		refdata.Turn{ID: turnID, CombatantID: combatantID},
		true,
	)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "cannot act")
	assert.Empty(t, fx.combatProv.hpUpdates, "an incapacitated character must not heal themselves back up")
	assert.Empty(t, fx.store.updatedInventory, "a rejected /use must not consume the potion")
}

// A failed combatant lookup must not fall back to the character sheet: that
// fallback is precisely the bug (sheet at full HP silently swallows the
// healing), so an unreadable combat state has to abort the whole use.
func TestUseHandler_CombatantLookupError_AbortsWithoutConsumingItem(t *testing.T) {
	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: uuid.New(), HpCurrent: 4, HpMax: 31, IsAlive: true},
		refdata.Turn{ID: uuid.New()},
		true,
	)
	fx.combatProv.combatantErr = errors.New("db down")

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "combat state")
	assert.Empty(t, fx.store.updatedInventory, "the potion must survive a failed lookup")
	assert.Empty(t, fx.combatProv.hpUpdates)
}

// The HP write is deliberately ordered BEFORE the inventory write so that a
// failure costs the player nothing: the potion must still be in the bag. The
// reverse ordering reproduces the original bug — potion gone, HP unmoved.
func TestUseHandler_CombatantHPWriteError_LeavesPotionUnconsumed(t *testing.T) {
	combatantID := uuid.New()
	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: combatantID, HpCurrent: 4, HpMax: 24, IsAlive: true},
		refdata.Turn{ID: uuid.New(), CombatantID: combatantID},
		true,
	)
	fx.combatProv.hpUpdateErr = errors.New("write failed")

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "Failed to save")
	assert.NotContains(t, fx.sess.lastResponse, "healed", "a failed HP write must not report healing")
	assert.Empty(t, fx.store.updatedInventory, "a failed heal must not cost the player the potion")
}

// The magic-item charge branch of /use shares the same turn lookup, so it
// needs the same ownership guard — activating a wand on someone else's turn
// would otherwise spend that combatant's Action.
func TestUseHandler_MagicItemOffTurn_Rejected(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Charges: 5, MaxCharges: 7},
	}
	itemsJSON, err := json.Marshal(items)
	assert.NoError(t, err)

	attunement := []character.AttunementSlot{{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"}}
	attunementJSON, err := json.Marshal(attunement)
	assert.NoError(t, err)

	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}
	combatProv := &mockUseCombatProvider{
		turn:         refdata.Turn{ID: uuid.New(), CombatantID: uuid.New()}, // not this character's
		inCombat:     true,
		combatant:    refdata.Combatant{ID: combatantID, HpCurrent: 20, HpMax: 30, IsAlive: true},
		hasCombatant: true,
	}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: attunementJSON, Valid: true},
		}},
		store,
		nil,
		combatProv,
	)

	handler.Handle(makeUseInteraction("guild1", "user1", "wand-of-fireballs"))

	assert.Contains(t, sess.lastResponse, "your turn")
	assert.Empty(t, combatProv.spends, "must not spend another combatant's action")
	assert.Empty(t, store.updatedInventory, "a rejected activation must not burn a charge")
}

// Out of combat there is no token, so the character sheet stays authoritative.
func TestUseHandler_PotionOutOfCombat_StillHealsSheet(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, err := json.Marshal(items)
	assert.NoError(t, err)

	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}
	combatProv := &mockUseCombatProvider{inCombat: false}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         charID,
			CampaignID: campID,
			Name:       "Vale",
			HpCurrent:  10,
			HpMax:      31,
			Inventory:  pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		func(max int) int { return 3 },
		combatProv,
	)

	handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Equal(t, int32(18), store.updatedHPCurrent, "10 + 8 healed on the sheet")
	assert.Empty(t, combatProv.hpUpdates, "no combat token to write")
}
