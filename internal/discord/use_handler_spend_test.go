package discord

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// The turn-resource spend used to be a read-modify-write of all 11 turn
// columns whose error was discarded, running AFTER the inventory write. Two
// consequences, both pinned below: a resource the player had already spent was
// silently double-spent from a stale snapshot, and any spend failure still
// cost them the item. The spend now runs as a targeted compare-and-set BEFORE
// anything is persisted, so a rejected spend leaves the potion in the bag.

// newSpendFixture builds a wounded in-combat /use handler. It reuses the
// combat-HP fixture because both HP stores must be observable to prove the
// "nothing was persisted" half of a rejected spend.
func newSpendFixture(t *testing.T) (*useCombatHPFixture, uuid.UUID) {
	t.Helper()
	combatantID := uuid.New()
	encounterID := uuid.New()
	fx := newUseCombatHPFixture(t,
		refdata.Combatant{ID: combatantID, HpCurrent: 4, HpMax: 24, IsAlive: true},
		refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: combatantID},
		true,
	)
	return fx, encounterID
}

// newMagicItemChargeFixture builds an attuned, charged wand so the /use
// magic-item branch (which costs an Action, not a bonus action) is reachable.
func newMagicItemChargeFixture(t *testing.T, turn refdata.Turn, inCombat bool) (
	*mockInventorySession, *mockUseCharacterStore, *mockUseCombatProvider, *UseHandler,
) {
	t.Helper()

	campID := uuid.New()
	charID := uuid.New()

	itemsJSON, err := json.Marshal([]character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Charges: 5, MaxCharges: 7},
	})
	assert.NoError(t, err)
	attunementJSON, err := json.Marshal([]character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	})
	assert.NoError(t, err)

	sess := &mockInventorySession{}
	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}
	combatProv := &mockUseCombatProvider{turn: turn, inCombat: inCombat}

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
	return sess, store, combatProv, handler
}

// sql.ErrNoRows is the CAS's WHERE clause matching nothing: the resource was
// already spent by a command that landed between this request's turn read and
// its write. The player keeps the potion.
func TestUseHandler_PotionInCombat_SpendRejected_DoesNotConsumeItem(t *testing.T) {
	fx, _ := newSpendFixture(t)
	fx.combatProv.spendErr = sql.ErrNoRows

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "already spent")
	assert.Empty(t, fx.store.updatedInventory, "a rejected spend must not consume the potion")
	assert.Empty(t, fx.combatProv.hpUpdates, "a rejected spend must not heal")
	assert.Zero(t, fx.store.updatedHPCurrent)
}

func TestUseHandler_PotionInCombat_SpendErrors_DoesNotConsumeItem(t *testing.T) {
	fx, _ := newSpendFixture(t)
	fx.combatProv.spendErr = errors.New("connection reset")

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "Failed to save turn resources")
	assert.NotContains(t, fx.sess.lastResponse, "already spent", "a transport failure is not a spent resource")
	assert.Empty(t, fx.store.updatedInventory, "a failed spend must not consume the potion")
	assert.Empty(t, fx.combatProv.hpUpdates, "a failed spend must not heal")
}

func TestUseHandler_MagicItemCharge_SpendRejected_DoesNotConsumeCharge(t *testing.T) {
	sess, store, combatProv, handler := newMagicItemChargeFixture(t, refdata.Turn{ID: uuid.New()}, true)
	combatProv.spendErr = sql.ErrNoRows

	handler.Handle(makeUseInteraction("guild1", "user1", "wand-of-fireballs"))

	assert.Contains(t, sess.lastResponse, "already spent")
	assert.Empty(t, store.updatedInventory, "a rejected spend must not burn a charge")
}

// Out of combat there is no turn to spend against and no turn to lock, so both
// the CAS and the gate must stay entirely out of the path.
func TestUseHandler_PotionOutOfCombat_NoSpendNoGate(t *testing.T) {
	fx := newUseCombatHPFixture(t, refdata.Combatant{}, refdata.Turn{}, false)
	gate := &stubTurnGate{}
	fx.handler.SetTurnGate(gate)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "Healing Potion", "out-of-combat /use must still succeed")
	assert.NotEmpty(t, fx.store.updatedInventory, "the potion is still consumed out of combat")
	assert.Empty(t, fx.combatProv.spends, "out-of-combat /use spends no turn resource")
	assert.Zero(t, gate.calls, "out-of-combat /use must not touch the turn gate")
}

// persistUse's UpdateCombatantHP is a full-column overwrite computed from a
// snapshot read earlier in the request, so the write section has to hold the
// advisory lock — AcquireAndRun, not AcquireAndRelease.
func TestUseHandler_PotionInCombat_WriteSectionRunsUnderTurnGate(t *testing.T) {
	fx, encounterID := newSpendFixture(t)
	gate := &stubTurnGate{}
	fx.handler.SetTurnGate(gate)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Equal(t, 1, gate.runCalls, "expected AcquireAndRun to fire exactly once")
	assert.True(t, gate.ranInsideLock, "the spend + persist must run with the lock held")
	assert.Equal(t, encounterID, gate.lastEnc)
	assert.Equal(t, "user1", gate.lastUserID)
	assert.Len(t, fx.combatProv.spends, 1)
	assert.NotEmpty(t, fx.store.updatedInventory)
}

func TestUseHandler_PotionInCombat_GateRejection_NoWrites(t *testing.T) {
	fx, _ := newSpendFixture(t)
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{CurrentCharacterName: "Sabinnet"}}
	fx.handler.SetTurnGate(gate)

	fx.handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	assert.Contains(t, fx.sess.lastResponse, "Sabinnet")
	assert.Equal(t, discordgo.MessageFlagsEphemeral, fx.sess.lastFlags)
	assert.Empty(t, fx.combatProv.spends, "a gate rejection must not spend")
	assert.Empty(t, fx.store.updatedInventory, "a gate rejection must not consume the potion")
	assert.Empty(t, fx.combatProv.hpUpdates, "a gate rejection must not heal")
}
