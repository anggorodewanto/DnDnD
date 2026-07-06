package refdata

import (
	"sort"
	"testing"
)

// wiredFightingStyleSlugs is the exact set the combat engine consumes today
// (internal/combat: BuildFeatureDefinitions archery/defense/dueling/
// great_weapon_fighting + HasFightingStyle two_weapon_fighting). The catalog must
// hold precisely these — an id with no combat reader would be a pickable-but-inert
// dead-data option (the anti-pattern the 2024 coverage backlog warns against), and
// a wired slug missing from the catalog can never be picked in the builder.
var wiredFightingStyleSlugs = []string{
	"archery",
	"defense",
	"dueling",
	"great_weapon_fighting",
	"two_weapon_fighting",
}

func TestFightingStyleCatalog_MatchesWiredCombatSet(t *testing.T) {
	got := make([]string, 0, len(FightingStyleCatalog()))
	for _, s := range FightingStyleCatalog() {
		got = append(got, s.ID)
	}
	sort.Strings(got)

	want := append([]string(nil), wiredFightingStyleSlugs...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("catalog ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("catalog ids = %v, want %v", got, want)
		}
	}
}

func TestFightingStyleCatalog_SlugsCleanAndWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range FightingStyleCatalog() {
		if !slugRE.MatchString(s.ID) {
			t.Errorf("fighting style id %q is not a clean underscore slug (combat matches on it verbatim)", s.ID)
		}
		if seen[s.ID] {
			t.Errorf("duplicate fighting style id %q", s.ID)
		}
		seen[s.ID] = true
		if s.Name == "" || s.Description == "" {
			t.Errorf("fighting style %q has empty name/description", s.ID)
		}
	}
}

func TestFightingStyleGrantLevel(t *testing.T) {
	cases := []struct {
		class     string
		wantLevel int
		wantOK    bool
	}{
		{"fighter", 1, true},
		{"Fighter", 1, true}, // case-insensitive
		{"paladin", 2, true},
		{"ranger", 2, true},
		{"wizard", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		level, ok := FightingStyleGrantLevel(tc.class)
		if ok != tc.wantOK || level != tc.wantLevel {
			t.Errorf("FightingStyleGrantLevel(%q) = (%d,%v), want (%d,%v)", tc.class, level, ok, tc.wantLevel, tc.wantOK)
		}
	}
}

func TestFightingStyleByID(t *testing.T) {
	s, ok := FightingStyleByID()["archery"]
	if !ok {
		t.Fatal("catalog missing archery — combat reader depends on this exact slug")
	}
	if s.Name != "Archery" {
		t.Errorf("name = %q, want Archery", s.Name)
	}

	if _, ok := FightingStyleByID()["nonexistent_style"]; ok {
		t.Error("FightingStyleByID returned a hit for an unknown id")
	}
}
