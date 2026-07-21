package portal

import (
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestCantripsKnown(t *testing.T) {
	cases := []struct {
		class string
		level int
		want  int
	}{
		{"wizard", 1, 3}, {"wizard", 4, 4}, {"wizard", 10, 5},
		{"cleric", 1, 3}, {"druid", 1, 2}, {"bard", 1, 2},
		{"sorcerer", 1, 4}, {"sorcerer", 10, 6},
		{"warlock", 3, 2}, {"warlock", 4, 3},
		{"Wizard", 1, 3}, // case-insensitive
		{"paladin", 5, 0}, {"ranger", 5, 0}, {"fighter", 5, 0},
		{"wizard", 0, 3}, // level floored to 1
	}
	for _, c := range cases {
		if got := cantripsKnown(c.class, c.level, ""); got != c.want {
			t.Errorf("cantripsKnown(%q, %d) = %d, want %d", c.class, c.level, got, c.want)
		}
	}
}

func TestSpellsKnown(t *testing.T) {
	cases := []struct {
		class   string
		level   int
		want    int
		isKnown bool
	}{
		{"bard", 1, 4, true}, {"bard", 20, 22, true},
		{"sorcerer", 1, 2, true}, {"ranger", 1, 0, true}, {"ranger", 2, 2, true},
		{"warlock", 1, 2, true},
		{"wizard", 1, 0, false}, // prepared caster, not "known"
		{"cleric", 1, 0, false}, {"fighter", 1, 0, false},
		{"bard", 25, 22, true}, // level clamped to 20
	}
	for _, c := range cases {
		got, ok := spellsKnown(c.class, c.level)
		if ok != c.isKnown || got != c.want {
			t.Errorf("spellsKnown(%q, %d) = (%d, %v), want (%d, %v)", c.class, c.level, got, ok, c.want, c.isKnown)
		}
	}
}

func TestLeveledSpellCap(t *testing.T) {
	cases := []struct {
		class string
		level int
		mod   int
		want  int
	}{
		{"wizard", 1, 3, 4},   // prepared: 3 + 1
		{"cleric", 1, 0, 1},   // prepared: max(1, 0+1)
		{"wizard", 1, -1, 1},  // prepared: floored to 1
		{"bard", 1, 5, 4},     // known: ability mod ignored
		{"sorcerer", 1, 0, 2}, // known
		{"ranger", 1, 5, 0},   // known: 0 at level 1
		{"paladin", 1, 3, 0},  // half-caster: none before level 2
		{"paladin", 2, 2, 3},  // half-caster: max(1, 2 + 2/2)
		{"fighter", 5, 5, 0},  // non-caster
	}
	for _, c := range cases {
		if got := leveledSpellCap(c.class, c.level, c.mod, ""); got != c.want {
			t.Errorf("leveledSpellCap(%q, %d, %d) = %d, want %d", c.class, c.level, c.mod, got, c.want)
		}
	}
}

func TestSpellBudget_ExcludesCantripsFromLeveledCap(t *testing.T) {
	// The review's example: a level-1 wizard with INT 16 (+3) should get
	// 3 cantrips + 4 prepared = 7 total, not 4.
	if got := spellBudget("wizard", 1, 3, ""); got != 7 {
		t.Errorf("spellBudget(wizard, 1, +3) = %d, want 7", got)
	}
	// Level-1 bard (known caster): 2 cantrips + 4 spells known = 6.
	if got := spellBudget("bard", 1, 3, ""); got != 6 {
		t.Errorf("spellBudget(bard, 1, +3) = %d, want 6", got)
	}
	// Level-1 sorcerer: 4 cantrips + 2 known = 6.
	if got := spellBudget("sorcerer", 1, 3, ""); got != 6 {
		t.Errorf("spellBudget(sorcerer, 1, +3) = %d, want 6", got)
	}
	// Level-1 ranger: 0 cantrips + 0 known = 0.
	if got := spellBudget("ranger", 1, 3, ""); got != 0 {
		t.Errorf("spellBudget(ranger, 1, +3) = %d, want 0", got)
	}
}

func TestCantripsKnown_ThirdCasterSubclasses(t *testing.T) {
	cases := []struct {
		class    string
		level    int
		subclass string
		want     int
	}{
		{"fighter", 3, "eldritch-knight", 2}, {"fighter", 9, "eldritch-knight", 2},
		{"fighter", 10, "eldritch-knight", 3},
		{"fighter", 3, "Eldritch Knight", 2}, // spaced + cased variant
		{"rogue", 3, "arcane-trickster", 3}, {"rogue", 10, "arcane-trickster", 4},
		{"fighter", 2, "eldritch-knight", 0}, // not a caster yet
		{"fighter", 5, "champion", 0},        // wrong subclass
		{"fighter", 5, "", 0},                // no subclass
	}
	for _, c := range cases {
		if got := cantripsKnown(c.class, c.level, c.subclass); got != c.want {
			t.Errorf("cantripsKnown(%q, %d, %q) = %d, want %d", c.class, c.level, c.subclass, got, c.want)
		}
	}
}

func TestLeveledSpellCap_ThirdCasterSubclasses(t *testing.T) {
	cases := []struct {
		class    string
		level    int
		mod      int
		subclass string
		want     int
	}{
		{"fighter", 3, 5, "eldritch-knight", 3}, // ability mod ignored (known table)
		{"fighter", 4, 5, "eldritch-knight", 4},
		{"fighter", 7, 5, "eldritch-knight", 5},
		{"fighter", 20, 5, "eldritch-knight", 13},
		{"rogue", 3, 0, "arcane-trickster", 3},
		{"fighter", 2, 5, "eldritch-knight", 0}, // not a caster yet
		{"fighter", 5, 5, "champion", 0},        // wrong subclass
		{"fighter", 5, 5, "", 0},                // no subclass
	}
	for _, c := range cases {
		if got := leveledSpellCap(c.class, c.level, c.mod, c.subclass); got != c.want {
			t.Errorf("leveledSpellCap(%q, %d, %d, %q) = %d, want %d", c.class, c.level, c.mod, c.subclass, got, c.want)
		}
	}
}

func TestSpellBudget_ThirdCasterSubclasses(t *testing.T) {
	// EK L3: 2 cantrips + 3 spells known = 5.
	if got := spellBudget("fighter", 3, 5, "eldritch-knight"); got != 5 {
		t.Errorf("spellBudget(fighter, 3, +5, eldritch-knight) = %d, want 5", got)
	}
	// AT L3: 3 cantrips + 3 spells known = 6.
	if got := spellBudget("rogue", 3, 0, "arcane-trickster"); got != 6 {
		t.Errorf("spellBudget(rogue, 3, +0, arcane-trickster) = %d, want 6", got)
	}
	// Plain fighter L3 (no EK): 0.
	if got := spellBudget("fighter", 3, 5, "champion"); got != 0 {
		t.Errorf("spellBudget(fighter, 3, +5, champion) = %d, want 0", got)
	}
}

func TestMulticlassSpellBudget(t *testing.T) {
	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 14, CHA: 12}
	wizardBudget := spellBudget("wizard", 1, character.AbilityModifier(scores.INT), "")  // 3 cantrips + (3+1) prepared = 7
	clericBudget := spellBudget("cleric", 1, character.AbilityModifier(scores.WIS), "")  // 3 cantrips + (2+1) prepared = 6
	wizard3Budget := spellBudget("wizard", 3, character.AbilityModifier(scores.INT), "") // 3 cantrips + (3+3) prepared = 9

	cases := []struct {
		name    string
		classes []character.ClassEntry
		wantCap int
		wantOK  bool
	}{
		{
			name:    "single-class wizard matches spellBudget",
			classes: []character.ClassEntry{{Class: "wizard", Level: 1}},
			wantCap: wizardBudget,
			wantOK:  true,
		},
		{
			name:    "wizard1 + cleric1 sums both budgets",
			classes: []character.ClassEntry{{Class: "wizard", Level: 1, IsPrimary: true}, {Class: "cleric", Level: 1}},
			wantCap: wizardBudget + clericBudget,
			wantOK:  true,
		},
		{
			name:    "non-caster primary + caster secondary uses the caster budget",
			classes: []character.ClassEntry{{Class: "fighter", Level: 1, IsPrimary: true}, {Class: "wizard", Level: 3}},
			wantCap: wizard3Budget,
			wantOK:  true,
		},
		{
			name:    "two non-casters yield no cap",
			classes: []character.ClassEntry{{Class: "fighter", Level: 1, IsPrimary: true}, {Class: "barbarian", Level: 1}},
			wantCap: 0,
			wantOK:  false,
		},
		{
			name:    "empty class list yields no cap",
			classes: nil,
			wantCap: 0,
			wantOK:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotCap, gotOK := multiclassSpellBudget(c.classes, scores)
			if gotCap != c.wantCap || gotOK != c.wantOK {
				t.Errorf("multiclassSpellBudget = (%d, %v), want (%d, %v)", gotCap, gotOK, c.wantCap, c.wantOK)
			}
		})
	}
}

// TestValidateSpellCount_Multiclass proves the bug fix: a Fighter1/Wizard3 build
// (non-caster primary) must surface the wizard's full budget. A submission whose
// spell count lands between the old primary-only cap (0, fighter) and the new
// summed cap now passes instead of being rejected.
func TestValidateSpellCount_Multiclass(t *testing.T) {
	scores := PointBuyScores{STR: 14, DEX: 12, CON: 14, INT: 16, WIS: 10, CHA: 10}
	wizard3Budget := spellBudget("wizard", 3, character.AbilityModifier(scores.Character().INT), "") // 9
	spells := make([]string, wizard3Budget)
	for i := range spells {
		spells[i] = "s"
	}
	sub := CharacterSubmission{
		Name:          "Gish",
		Race:          "human",
		Classes:       []character.ClassEntry{{Class: "fighter", Level: 1, IsPrimary: true}, {Class: "wizard", Level: 3}},
		AbilityScores: scores,
		Spells:        spells,
	}
	if errs := validateSpellCount(sub); len(errs) != 0 {
		t.Errorf("validateSpellCount(fighter1/wizard3, %d spells) = %v, want no errors", wizard3Budget, errs)
	}
	// One over the summed budget must still be rejected.
	sub.Spells = append(sub.Spells, "extra")
	if errs := validateSpellCount(sub); len(errs) == 0 {
		t.Errorf("validateSpellCount(fighter1/wizard3, %d spells) = no errors, want a too-many-spells error", len(sub.Spells))
	}
}

// TestSpellCountCap_Multiclass checks the summed cap through the submission entry
// point (wizard1 + cleric1) and that two non-casters report no cap.
func TestSpellCountCap_Multiclass(t *testing.T) {
	scores := PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 14, CHA: 12}
	wizardBudget := spellBudget("wizard", 1, character.AbilityModifier(scores.Character().INT), "")
	clericBudget := spellBudget("cleric", 1, character.AbilityModifier(scores.Character().WIS), "")

	caster := CharacterSubmission{
		Name:          "Cleric-Wizard",
		Race:          "human",
		Classes:       []character.ClassEntry{{Class: "wizard", Level: 1, IsPrimary: true}, {Class: "cleric", Level: 1}},
		AbilityScores: scores,
	}
	if cap, ok := spellCountCap(caster); !ok || cap != wizardBudget+clericBudget {
		t.Errorf("spellCountCap(wizard1/cleric1) = (%d, %v), want (%d, true)", cap, ok, wizardBudget+clericBudget)
	}

	nonCasters := CharacterSubmission{
		Name:          "Brute",
		Race:          "human",
		Classes:       []character.ClassEntry{{Class: "fighter", Level: 1, IsPrimary: true}, {Class: "barbarian", Level: 1}},
		AbilityScores: scores,
	}
	if cap, ok := spellCountCap(nonCasters); ok || cap != 0 {
		t.Errorf("spellCountCap(fighter1/barbarian1) = (%d, %v), want (0, false)", cap, ok)
	}
}

// TestValidateSpellCount_TomeCantripsDoNotCount proves Pact-of-the-Tome bonus
// cantrips in the dedicated field never count against the chosen-spell cap: a
// warlock at exactly the cap (8 at L4: 3 cantrips + 5 known) plus 3 tome
// cantrips validates, while moving those 3 into the counted `spells` list
// overflows and fails.
func TestValidateSpellCount_TomeCantripsDoNotCount(t *testing.T) {
	spells := make([]string, 8) // warlock L4 spell budget
	for i := range spells {
		spells[i] = fmt.Sprintf("s%d", i)
	}
	sub := warlockSub(4, spells, "pact_of_the_tome", nil)
	sub.TomeCantrips = []string{"guidance", "minor-illusion", "prestidigitation"}
	if errs := validateSpellCount(sub); len(errs) != 0 {
		t.Errorf("validateSpellCount(warlock4, 8 spells + 3 tome cantrips) = %v, want no errors", errs)
	}

	// The same three cantrips placed in the counted list overflow the cap.
	inSpells := append(append([]string{}, spells...), "guidance", "minor-illusion", "prestidigitation")
	overflow := warlockSub(4, inSpells, "pact_of_the_tome", nil)
	if errs := validateSpellCount(overflow); len(errs) == 0 {
		t.Error("validateSpellCount(warlock4, 11 spells) = no errors, want a too-many-spells error")
	}
}

// TestValidateSpellCount_EldritchKnight verifies the end-to-end validation path:
// an EK L3 with 2 cantrips + 3 spells (= 5, the budget) passes, but 6 is rejected.
func TestValidateSpellCount_EldritchKnight(t *testing.T) {
	base := func(spells []string) CharacterSubmission {
		return CharacterSubmission{
			Name:          "EK",
			Race:          "human",
			Classes:       []character.ClassEntry{{Class: "fighter", Subclass: "eldritch-knight", Level: 3, IsPrimary: true}},
			AbilityScores: PointBuyScores{STR: 16, DEX: 14, CON: 14, INT: 16, WIS: 10, CHA: 10},
			Spells:        spells,
		}
	}
	ok := base([]string{"a", "b", "c", "d", "e"}) // 5 == budget
	if errs := validateSpellCount(ok); len(errs) != 0 {
		t.Errorf("validateSpellCount(EK L3, 5 spells) = %v, want no errors", errs)
	}
	over := base([]string{"a", "b", "c", "d", "e", "f"}) // 6 > budget
	if errs := validateSpellCount(over); len(errs) == 0 {
		t.Errorf("validateSpellCount(EK L3, 6 spells) = no errors, want a too-many-spells error")
	}
}
