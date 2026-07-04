package refdata

import (
	"regexp"
	"testing"
)

// slugRE is the clean-slug contract every pact-boon / invocation id must honor:
// lowercase + underscores only, no hyphens. This matters because an invocation's
// id doubles as the character.Feature.MechanicalEffect the combat engine matches
// (e.g. agonizing_blast -> internal/combat/agonizing_blast.go). A hyphen slug or
// the JSON-array feat encoding would silently never match.
var slugRE = regexp.MustCompile(`^[a-z][a-z_]*$`)

func TestInvocationCatalog_HasAgonizingBlastCleanSlug(t *testing.T) {
	inv, ok := InvocationByID()["agonizing_blast"]
	if !ok {
		t.Fatal("catalog missing agonizing_blast — combat reader depends on this exact slug")
	}
	if inv.Name != "Agonizing Blast" {
		t.Errorf("name = %q, want Agonizing Blast", inv.Name)
	}
	if !inv.Prereq.RequiresEldritchBlast {
		t.Error("Agonizing Blast must require the eldritch blast cantrip")
	}
	if inv.Prereq.MinWarlockLevel != 0 {
		t.Errorf("min warlock level = %d, want 0 (2014 Agonizing Blast has no level gate)", inv.Prereq.MinWarlockLevel)
	}
}

func TestInvocationCatalog_SlugsUniqueAndClean(t *testing.T) {
	seen := map[string]bool{}
	invs := InvocationCatalog()
	if len(invs) < 18 {
		t.Errorf("catalog has %d invocations, want >= 18", len(invs))
	}
	for _, inv := range invs {
		if !slugRE.MatchString(inv.ID) {
			t.Errorf("invocation id %q is not a clean underscore slug", inv.ID)
		}
		if seen[inv.ID] {
			t.Errorf("duplicate invocation id %q", inv.ID)
		}
		seen[inv.ID] = true
		if inv.Name == "" || inv.Description == "" {
			t.Errorf("invocation %q missing name/description", inv.ID)
		}
	}
}

func TestInvocationCatalog_PrereqBoonRefsExist(t *testing.T) {
	boons := PactBoonByID()
	for _, inv := range InvocationCatalog() {
		if inv.Prereq.RequiresPactBoon == "" {
			continue
		}
		if _, ok := boons[inv.Prereq.RequiresPactBoon]; !ok {
			t.Errorf("invocation %q requires unknown pact boon %q", inv.ID, inv.Prereq.RequiresPactBoon)
		}
	}
}

func TestPactBoonCatalog_HasFourBoons(t *testing.T) {
	boons := PactBoonCatalog()
	if len(boons) != 4 {
		t.Fatalf("got %d pact boons, want 4", len(boons))
	}
	want := map[string]bool{
		"pact_of_the_blade":    false,
		"pact_of_the_chain":    false,
		"pact_of_the_tome":     false,
		"pact_of_the_talisman": false,
	}
	seen := map[string]bool{}
	for _, b := range boons {
		if !slugRE.MatchString(b.ID) {
			t.Errorf("pact boon id %q is not a clean underscore slug", b.ID)
		}
		if seen[b.ID] {
			t.Errorf("duplicate pact boon id %q", b.ID)
		}
		seen[b.ID] = true
		if _, ok := want[b.ID]; ok {
			want[b.ID] = true
		}
		if b.Name == "" || b.Description == "" {
			t.Errorf("pact boon %q missing name/description", b.ID)
		}
	}
	for id, present := range want {
		if !present {
			t.Errorf("expected pact boon %q missing from catalog", id)
		}
	}
}

func TestInvocationsKnown_Table(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{1, 0}, {2, 2}, {3, 2}, {4, 2}, {5, 3}, {6, 3}, {7, 4}, {8, 4},
		{9, 5}, {10, 5}, {11, 5}, {12, 6}, {13, 6}, {14, 6}, {15, 7},
		{16, 7}, {17, 7}, {18, 8}, {19, 8}, {20, 8},
	}
	for _, c := range cases {
		if got := InvocationsKnown(c.level); got != c.want {
			t.Errorf("InvocationsKnown(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestPactBoonGranted_AtLevel3(t *testing.T) {
	for lvl := range 3 {
		if PactBoonGranted(lvl) {
			t.Errorf("PactBoonGranted(%d) = true, want false", lvl)
		}
	}
	for _, lvl := range []int{3, 5, 20} {
		if !PactBoonGranted(lvl) {
			t.Errorf("PactBoonGranted(%d) = false, want true", lvl)
		}
	}
}

func TestInvocationByID_RoundTrips(t *testing.T) {
	if len(InvocationByID()) != len(InvocationCatalog()) {
		t.Error("InvocationByID size differs from catalog (duplicate ids?)")
	}
	if len(PactBoonByID()) != len(PactBoonCatalog()) {
		t.Error("PactBoonByID size differs from catalog (duplicate ids?)")
	}
}

func TestInvocationCatalog_Includes2024Additions(t *testing.T) {
	byID := InvocationByID()
	cases := []struct {
		id           string
		minWL        int
		requiresBoon string
	}{
		{"eldritch_mind", 0, ""},
		{"lessons_of_the_first_ones", 0, ""},
		{"otherworldly_leap", 5, ""},
		{"investment_of_the_chain_master", 5, "pact_of_the_chain"},
		{"gift_of_the_protectors", 9, "pact_of_the_tome"},
	}
	for _, c := range cases {
		inv, ok := byID[c.id]
		if !ok {
			t.Errorf("catalog missing 2024 invocation %q", c.id)
			continue
		}
		if inv.Edition != Edition2024 {
			t.Errorf("%q edition = %q, want %q", c.id, inv.Edition, Edition2024)
		}
		if inv.Prereq.MinWarlockLevel != c.minWL {
			t.Errorf("%q min warlock level = %d, want %d", c.id, inv.Prereq.MinWarlockLevel, c.minWL)
		}
		if inv.Prereq.RequiresPactBoon != c.requiresBoon {
			t.Errorf("%q requires boon = %q, want %q", c.id, inv.Prereq.RequiresPactBoon, c.requiresBoon)
		}
	}
}

func TestInvocationCatalog_OtherworldlyLeapGrantsJump(t *testing.T) {
	inv := InvocationByID()["otherworldly_leap"]
	found := false
	for _, s := range inv.GrantsSpells {
		if s == "jump" {
			found = true
		}
	}
	if !found {
		t.Errorf("otherworldly_leap should grant jump, grants %v", inv.GrantsSpells)
	}
}

func TestPactBoon_TalismanTagged2014(t *testing.T) {
	b, ok := PactBoonByID()["pact_of_the_talisman"]
	if !ok {
		t.Fatal("catalog missing pact_of_the_talisman")
	}
	if b.Edition != Edition2014 {
		t.Errorf("pact_of_the_talisman edition = %q, want %q (Tasha's/2014, dropped from the 2024 base pacts)", b.Edition, Edition2014)
	}
}

// TestCatalog_EditionValuesValid pins the small closed set of edition tags: ""
// (edition-agnostic / both PHBs), "2014", or "2024". A typo'd tag would silently
// mis-badge an entry in the picker.
func TestCatalog_EditionValuesValid(t *testing.T) {
	valid := map[string]bool{"": true, Edition2014: true, Edition2024: true}
	for _, inv := range InvocationCatalog() {
		if !valid[inv.Edition] {
			t.Errorf("invocation %q has invalid edition %q", inv.ID, inv.Edition)
		}
	}
	for _, b := range PactBoonCatalog() {
		if !valid[b.Edition] {
			t.Errorf("pact boon %q has invalid edition %q", b.ID, b.Edition)
		}
	}
}

func TestInvocationCatalog_GrantSpellExamples(t *testing.T) {
	byID := InvocationByID()
	cases := map[string]string{
		"mask_of_many_faces": "disguise-self",
		"armor_of_shadows":   "mage-armor",
	}
	for id, spell := range cases {
		inv, ok := byID[id]
		if !ok {
			t.Errorf("catalog missing %q", id)
			continue
		}
		found := false
		for _, s := range inv.GrantsSpells {
			if s == spell {
				found = true
			}
		}
		if !found {
			t.Errorf("%q should grant spell %q, grants %v", id, spell, inv.GrantsSpells)
		}
	}
}
