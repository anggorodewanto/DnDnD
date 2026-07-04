package portal

import (
	"testing"

	"github.com/sqlc-dev/pqtype"
)

// P5: extractSpells surfaces invocation-granted spells (character_data.
// granted_spells) alongside the known spells, tagging granted-only entries
// Source "invocation" and de-duplicating against the known list.
func TestExtractSpells_IncludesGrantedSpells(t *testing.T) {
	raw := pqtype.NullRawMessage{
		RawMessage: []byte(`{"spells":["fire-bolt","disguise-self"],"granted_spells":["disguise-self","mage-armor"]}`),
		Valid:      true,
	}

	entries := extractSpells(raw)

	byID := map[string]SpellDisplayEntry{}
	for _, e := range entries {
		if _, dup := byID[e.ID]; dup {
			t.Errorf("duplicate entry for %q", e.ID)
		}
		byID[e.ID] = e
	}

	if _, ok := byID["fire-bolt"]; !ok {
		t.Error("known spell fire-bolt missing")
	}
	mage, ok := byID["mage-armor"]
	if !ok {
		t.Fatal("granted-only spell mage-armor missing")
	}
	if mage.Source != "invocation" {
		t.Errorf("mage-armor Source = %q, want invocation", mage.Source)
	}
	// disguise-self is in both lists: the known entry wins (no duplicate, not
	// re-tagged as invocation).
	if got := byID["disguise-self"]; got.Source == "invocation" {
		t.Error("disguise-self is a known spell; must not be re-tagged as invocation")
	}
}

func TestSpellHeadline(t *testing.T) {
	tests := []struct {
		name  string
		entry SpellDisplayEntry
		want  string
	}{
		{"cantrip with school", SpellDisplayEntry{Level: 0, School: "evocation"}, "Evocation cantrip"},
		{"cantrip no school", SpellDisplayEntry{Level: 0, School: ""}, "Cantrip"},
		{"first level", SpellDisplayEntry{Level: 1, School: "abjuration"}, "1st-level abjuration"},
		{"second level", SpellDisplayEntry{Level: 2, School: "evocation"}, "2nd-level evocation"},
		{"third level", SpellDisplayEntry{Level: 3, School: "evocation"}, "3rd-level evocation"},
		{"ninth level", SpellDisplayEntry{Level: 9, School: "necromancy"}, "9th-level necromancy"},
		{"leveled no school", SpellDisplayEntry{Level: 4, School: ""}, "4th-level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spellHeadline(tt.entry); got != tt.want {
				t.Errorf("spellHeadline(%+v) = %q, want %q", tt.entry, got, tt.want)
			}
		})
	}
}
