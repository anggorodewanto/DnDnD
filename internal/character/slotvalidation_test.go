package character

import (
	"testing"
)

func TestParseSpellSlotsJSON(t *testing.T) {
	t.Run("empty input yields empty map", func(t *testing.T) {
		got, err := ParseSpellSlotsJSON(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})
	t.Run("parses string-keyed levels", func(t *testing.T) {
		got, err := ParseSpellSlotsJSON([]byte(`{"1":{"current":2,"max":4},"3":{"current":0,"max":2}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[1] != (SlotInfo{Current: 2, Max: 4}) || got[3] != (SlotInfo{Current: 0, Max: 2}) {
			t.Fatalf("unexpected parse: %v", got)
		}
	})
	t.Run("non-integer key errors", func(t *testing.T) {
		if _, err := ParseSpellSlotsJSON([]byte(`{"x":{"current":1,"max":1}}`)); err == nil {
			t.Fatalf("expected error for non-integer key")
		}
	})
	t.Run("malformed JSON errors", func(t *testing.T) {
		if _, err := ParseSpellSlotsJSON([]byte(`{bad`)); err == nil {
			t.Fatalf("expected error for malformed JSON")
		}
	})
}

func TestMarshalSpellSlotsJSON_RoundTrips(t *testing.T) {
	in := map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 1, Max: 3}}
	raw, err := MarshalSpellSlotsJSON(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	got, err := ParseSpellSlotsJSON(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got[1] != in[1] || got[2] != in[2] || len(got) != 2 {
		t.Fatalf("round-trip mismatch: %v -> %s -> %v", in, raw, got)
	}
}

func TestValidateSpellSlots(t *testing.T) {
	tests := []struct {
		name    string
		slots   map[int]SlotInfo
		wantErr bool
	}{
		{"nil is valid", nil, false},
		{"empty is valid", map[int]SlotInfo{}, false},
		{"valid single level", map[int]SlotInfo{1: {Current: 2, Max: 4}}, false},
		{"valid current equals max", map[int]SlotInfo{3: {Current: 3, Max: 3}}, false},
		{"valid zero current", map[int]SlotInfo{1: {Current: 0, Max: 2}}, false},
		{"level too low", map[int]SlotInfo{0: {Current: 0, Max: 1}}, true},
		{"level too high", map[int]SlotInfo{10: {Current: 0, Max: 1}}, true},
		{"negative max", map[int]SlotInfo{1: {Current: 0, Max: -1}}, true},
		{"negative current", map[int]SlotInfo{1: {Current: -1, Max: 2}}, true},
		{"current exceeds max", map[int]SlotInfo{2: {Current: 3, Max: 2}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpellSlots(tt.slots)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePactSlots(t *testing.T) {
	tests := []struct {
		name    string
		pact    PactMagicSlots
		wantErr bool
	}{
		{"zero value is valid (no pact magic)", PactMagicSlots{}, false},
		{"valid full", PactMagicSlots{SlotLevel: 2, Current: 2, Max: 2}, false},
		{"valid depleted", PactMagicSlots{SlotLevel: 5, Current: 0, Max: 4}, false},
		{"slot level too low", PactMagicSlots{SlotLevel: 0, Current: 0, Max: 2}, true},
		{"slot level too high", PactMagicSlots{SlotLevel: 6, Current: 1, Max: 2}, true},
		{"negative max", PactMagicSlots{SlotLevel: 2, Current: 0, Max: -1}, true},
		{"negative current", PactMagicSlots{SlotLevel: 2, Current: -1, Max: 2}, true},
		{"current exceeds max", PactMagicSlots{SlotLevel: 2, Current: 3, Max: 2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePactSlots(tt.pact)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
