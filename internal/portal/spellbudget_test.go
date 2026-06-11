package portal

import "testing"

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
		if got := cantripsKnown(c.class, c.level); got != c.want {
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
		if got := leveledSpellCap(c.class, c.level, c.mod); got != c.want {
			t.Errorf("leveledSpellCap(%q, %d, %d) = %d, want %d", c.class, c.level, c.mod, got, c.want)
		}
	}
}

func TestSpellBudget_ExcludesCantripsFromLeveledCap(t *testing.T) {
	// The review's example: a level-1 wizard with INT 16 (+3) should get
	// 3 cantrips + 4 prepared = 7 total, not 4.
	if got := spellBudget("wizard", 1, 3); got != 7 {
		t.Errorf("spellBudget(wizard, 1, +3) = %d, want 7", got)
	}
	// Level-1 bard (known caster): 2 cantrips + 4 spells known = 6.
	if got := spellBudget("bard", 1, 3); got != 6 {
		t.Errorf("spellBudget(bard, 1, +3) = %d, want 6", got)
	}
	// Level-1 sorcerer: 4 cantrips + 2 known = 6.
	if got := spellBudget("sorcerer", 1, 3); got != 6 {
		t.Errorf("spellBudget(sorcerer, 1, +3) = %d, want 6", got)
	}
	// Level-1 ranger: 0 cantrips + 0 known = 0.
	if got := spellBudget("ranger", 1, 3); got != 0 {
		t.Errorf("spellBudget(ranger, 1, +3) = %d, want 0", got)
	}
}
