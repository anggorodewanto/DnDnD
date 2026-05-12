package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-006 / Phase 88a / F-14 — turn-start speed must absorb magic-item
// `modify_speed` passive effects. Boots of Speed are an equipped + attuned
// magic item that adds 30ft to base speed. Before the fix, the FES
// FeatureDefinitions never included the boots so the +30 was lost.
func TestResolveTurnResources_BootsOfSpeed_AddSpeed(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}

	inv, err := json.Marshal([]character.InventoryItem{
		{
			ItemID:             "boots-of-speed",
			Name:               "Boots of Speed",
			Type:               "magic_item",
			IsMagic:            true,
			RequiresAttunement: true,
			Equipped:           true,
			MagicProperties:    `[{"type":"modify_speed","modifier":30}]`,
		},
	})
	require.NoError(t, err)
	attunement, err := json.Marshal([]character.AttunementSlot{
		{ItemID: "boots-of-speed", Name: "Boots of Speed"},
	})
	require.NoError(t, err)

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:              charID,
			SpeedFt:         30,
			Classes:         json.RawMessage(`[]`),
			Inventory:       pqtype.NullRawMessage{RawMessage: inv, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: attunement, Valid: true},
		}, nil
	}

	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, int32(60), speed, "base speed 30ft + Boots of Speed +30ft = 60ft")
}

// Negative control: unattuned Boots of Speed contribute nothing because the
// item requires attunement and the slot list is empty.
func TestResolveTurnResources_BootsOfSpeed_UnattunedNoBonus(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}

	inv, _ := json.Marshal([]character.InventoryItem{
		{
			ItemID:             "boots-of-speed",
			Name:               "Boots of Speed",
			Type:               "magic_item",
			IsMagic:            true,
			RequiresAttunement: true,
			Equipped:           true,
			MagicProperties:    `[{"type":"modify_speed","modifier":30}]`,
		},
	})

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:        charID,
			SpeedFt:   30,
			Classes:   json.RawMessage(`[]`),
			Inventory: pqtype.NullRawMessage{RawMessage: inv, Valid: true},
		}, nil
	}

	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, int32(30), speed, "boots requiring attunement but not attuned must not add speed")
}
