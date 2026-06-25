package portal

import "testing"

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
