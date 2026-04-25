package character

import (
	"testing"
)

func TestParseInventoryItems_NilWhenInvalid(t *testing.T) {
	items, err := ParseInventoryItems([]byte(`[{"name":"x"}]`), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items when valid=false, got %v", items)
	}
}

func TestParseInventoryItems_NilWhenEmpty(t *testing.T) {
	items, err := ParseInventoryItems(nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items when raw is empty, got %v", items)
	}
}

func TestParseInventoryItems_RoundTrip(t *testing.T) {
	raw := []byte(`[{"item_id":"sword","name":"Sword","quantity":1,"type":"weapon"}]`)
	items, err := ParseInventoryItems(raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ItemID != "sword" || items[0].Quantity != 1 {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

func TestParseInventoryItems_InvalidJSON(t *testing.T) {
	_, err := ParseInventoryItems([]byte(`not json`), true)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseAttunementSlots_NilWhenInvalid(t *testing.T) {
	slots, err := ParseAttunementSlots([]byte(`[]`), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slots != nil {
		t.Fatalf("expected nil slots when valid=false, got %v", slots)
	}
}

func TestParseAttunementSlots_NilWhenEmpty(t *testing.T) {
	slots, err := ParseAttunementSlots(nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slots != nil {
		t.Fatalf("expected nil slots when empty, got %v", slots)
	}
}

func TestParseAttunementSlots_RoundTrip(t *testing.T) {
	raw := []byte(`[{"item_id":"ring-of-protection","name":"Ring of Protection"}]`)
	slots, err := ParseAttunementSlots(raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slots) != 1 || slots[0].ItemID != "ring-of-protection" {
		t.Fatalf("unexpected slot: %+v", slots)
	}
}

func TestParseAttunementSlots_InvalidJSON(t *testing.T) {
	_, err := ParseAttunementSlots([]byte(`{`), true)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMarshalInventory_RoundTrip(t *testing.T) {
	items := []InventoryItem{{ItemID: "sword", Name: "Sword", Quantity: 2, Type: "weapon"}}
	raw, err := MarshalInventory(items)
	if err != nil {
		t.Fatalf("MarshalInventory error: %v", err)
	}
	parsed, err := ParseInventoryItems(raw, true)
	if err != nil {
		t.Fatalf("Parse after marshal error: %v", err)
	}
	if len(parsed) != 1 || parsed[0].ItemID != "sword" || parsed[0].Quantity != 2 {
		t.Fatalf("round trip mismatch: got %+v", parsed)
	}
}

func TestMarshalAttunementSlots_RoundTrip(t *testing.T) {
	slots := []AttunementSlot{{ItemID: "ring", Name: "Ring"}}
	raw, err := MarshalAttunementSlots(slots)
	if err != nil {
		t.Fatalf("MarshalAttunementSlots error: %v", err)
	}
	parsed, err := ParseAttunementSlots(raw, true)
	if err != nil {
		t.Fatalf("Parse after marshal error: %v", err)
	}
	if len(parsed) != 1 || parsed[0].ItemID != "ring" {
		t.Fatalf("round trip mismatch: got %+v", parsed)
	}
}

func TestFormatClassSummary_SingleClass(t *testing.T) {
	got := FormatClassSummary([]ClassEntry{{Class: "Fighter", Level: 5}})
	if got != "Fighter 5" {
		t.Fatalf("expected 'Fighter 5', got %q", got)
	}
}

func TestFormatClassSummary_WithSubclass(t *testing.T) {
	got := FormatClassSummary([]ClassEntry{{Class: "Wizard", Subclass: "Evocation", Level: 3}})
	if got != "Wizard 3 (Evocation)" {
		t.Fatalf("expected 'Wizard 3 (Evocation)', got %q", got)
	}
}

func TestFormatClassSummary_Multiclass(t *testing.T) {
	got := FormatClassSummary([]ClassEntry{
		{Class: "Fighter", Level: 5},
		{Class: "Wizard", Subclass: "Evocation", Level: 3},
	})
	want := "Fighter 5 / Wizard 3 (Evocation)"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatClassSummary_Empty(t *testing.T) {
	got := FormatClassSummary(nil)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Fire Bolt":       "fire-bolt",
		"  Magic Missile": "magic-missile",
		"Cure Wounds  ":   "cure-wounds",
		"ALREADYLOWER":    "alreadylower",
	}
	for input, want := range cases {
		if got := Slugify(input); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", input, got, want)
		}
	}
}
