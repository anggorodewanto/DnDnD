package discord

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-007: /equip success path (inventory-only legacy fallback) MUST publish
// OnCharacterUpdated so the persistent #character-cards message refreshes.
func TestEquipHandler_FiresCardUpdater_OnSuccess(t *testing.T) {
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
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	require.Len(t, rec.calls, 1, "expected exactly one OnCharacterUpdated call on /equip success")
	assert.Equal(t, charID, rec.calls[0])
}
