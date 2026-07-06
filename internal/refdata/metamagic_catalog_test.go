package refdata

import (
	"sort"
	"testing"
)

// wiredMetamagicSlugs is the exact set the combat engine consumes today
// (internal/combat/metamagic.go validateSingleMetamagicOption switch +
// internal/combat/sorcery.go sorcery-point cost map). The catalog must hold
// precisely these — a metamagic id with no cast-time consumer would be a
// pickable-but-inert dead-data option (the anti-pattern the 2024 coverage
// backlog warns against), and a wired slug missing from the catalog can never be
// picked in the builder (COV-15 mirror of wiredFightingStyleSlugs). The 2024
// options seeking/transmuted are deliberately absent: the cast path has no flag,
// cost, or validator for them, so adding them here would be dead data.
var wiredMetamagicSlugs = []string{
	"careful",
	"distant",
	"empowered",
	"extended",
	"heightened",
	"quickened",
	"subtle",
	"twinned",
}

func TestMetamagicCatalog_MatchesWiredCombatSet(t *testing.T) {
	got := make([]string, 0, len(MetamagicCatalog()))
	for _, m := range MetamagicCatalog() {
		got = append(got, m.ID)
	}
	sort.Strings(got)

	want := append([]string(nil), wiredMetamagicSlugs...)
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

func TestMetamagicCatalog_SlugsCleanAndWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range MetamagicCatalog() {
		if !slugRE.MatchString(m.ID) {
			t.Errorf("metamagic id %q is not a clean underscore slug (combat matches on it verbatim)", m.ID)
		}
		if seen[m.ID] {
			t.Errorf("duplicate metamagic id %q", m.ID)
		}
		seen[m.ID] = true
		if m.Name == "" || m.Description == "" {
			t.Errorf("metamagic %q has empty name/description", m.ID)
		}
	}
}

func TestMetamagicKnown(t *testing.T) {
	cases := []struct {
		sorcererLevel int
		want          int
	}{
		{0, 0},
		{2, 0}, // Metamagic unlocks at Sorcerer L3
		{3, 2},
		{9, 2},
		{10, 3}, // +1 at L10
		{16, 3},
		{17, 4}, // +1 at L17
		{20, 4},
	}
	for _, tc := range cases {
		if got := MetamagicKnown(tc.sorcererLevel); got != tc.want {
			t.Errorf("MetamagicKnown(%d) = %d, want %d", tc.sorcererLevel, got, tc.want)
		}
	}
}

func TestMetamagicByID(t *testing.T) {
	m, ok := MetamagicByID()["quickened"]
	if !ok {
		t.Fatal("catalog missing quickened — combat cast path depends on this exact slug")
	}
	if m.Name != "Quickened Spell" {
		t.Errorf("name = %q, want Quickened Spell", m.Name)
	}

	if _, ok := MetamagicByID()["nonexistent_metamagic"]; ok {
		t.Error("MetamagicByID returned a hit for an unknown id")
	}
}
