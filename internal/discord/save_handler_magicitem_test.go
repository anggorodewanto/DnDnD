package discord

import (
	"encoding/json"
	"testing"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-006 / Phase 88a — buildSaveFeatureEffects must surface magic-item
// `modify_saving_throw` passive effects (e.g. Cloak of Protection +1).
// Before the fix, only classes + features fed the FES; equipped + attuned
// magic items contributed nothing to /save.
func TestBuildSaveFeatureEffects_CloakOfProtection_AddsSaveBonus(t *testing.T) {
	inv, err := json.Marshal([]character.InventoryItem{
		{
			ItemID:             "cloak-of-protection",
			Name:               "Cloak of Protection",
			Type:               "magic_item",
			IsMagic:            true,
			RequiresAttunement: true,
			Equipped:           true,
			MagicProperties:    `[{"type":"modify_ac","modifier":1},{"type":"modify_saving_throw","modifier":1}]`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	attunement, err := json.Marshal([]character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	})
	if err != nil {
		t.Fatal(err)
	}

	char := refdata.Character{
		Inventory:       pqtype.NullRawMessage{RawMessage: inv, Valid: true},
		AttunementSlots: pqtype.NullRawMessage{RawMessage: attunement, Valid: true},
	}

	defs := buildSaveFeatureEffects(char)
	if len(defs) == 0 {
		t.Fatalf("expected magic-item feature definitions, got 0")
	}
	found := false
	for _, d := range defs {
		if d.Name == "Cloak of Protection" && d.Source == "magic_item" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Cloak of Protection in feature defs, got: %+v", defs)
	}
}

// Negative control: attuned-required Cloak of Protection that the character
// has not attuned must not appear in the feature defs.
func TestBuildSaveFeatureEffects_CloakOfProtection_UnattunedDropped(t *testing.T) {
	inv, _ := json.Marshal([]character.InventoryItem{
		{
			ItemID:             "cloak-of-protection",
			Name:               "Cloak of Protection",
			Type:               "magic_item",
			IsMagic:            true,
			RequiresAttunement: true,
			Equipped:           true,
			MagicProperties:    `[{"type":"modify_saving_throw","modifier":1}]`,
		},
	})

	char := refdata.Character{
		Inventory: pqtype.NullRawMessage{RawMessage: inv, Valid: true},
	}

	defs := buildSaveFeatureEffects(char)
	for _, d := range defs {
		if d.Name == "Cloak of Protection" {
			t.Errorf("unattuned Cloak of Protection must not appear in feature defs, got: %+v", defs)
		}
	}
}

// Bad inventory JSON degrades to nil (no panic, no error).
func TestBuildSaveFeatureEffects_BadInventoryJSON_DegradesToNil(t *testing.T) {
	char := refdata.Character{
		Inventory: pqtype.NullRawMessage{RawMessage: []byte("not json"), Valid: true},
	}
	// Should not panic; the magic-item path returns nil and classes/features
	// are also empty so the helper returns nil.
	if defs := buildSaveFeatureEffects(char); defs != nil {
		t.Errorf("expected nil on bad inventory JSON, got %+v", defs)
	}
}
