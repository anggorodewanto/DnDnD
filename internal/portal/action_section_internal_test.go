package portal

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

// findGroup returns the action group with the given label, or nil.
func findGroup(groups []ActionGroup, label string) *ActionGroup {
	for i := range groups {
		if groups[i].Economy == label {
			return &groups[i]
		}
	}
	return nil
}

// hasAction reports whether a group contains an action with the given name.
func hasAction(g *ActionGroup, name string) bool {
	if g == nil {
		return false
	}
	for _, a := range g.Actions {
		if a.Name == name {
			return true
		}
	}
	return false
}

func TestBuildActionGroups_UniversalForEveryone(t *testing.T) {
	groups := buildActionGroups([]character.ClassEntry{{Class: "Wizard", Level: 5}})

	actions := findGroup(groups, "Actions")
	if !hasAction(actions, "Dash") || !hasAction(actions, "Dodge") || !hasAction(actions, "Attack") {
		t.Errorf("universal actions (Dash/Dodge/Attack) missing for wizard: %+v", actions)
	}
	// Class-gated barbarian ability must not appear for a wizard.
	if hasAction(findGroup(groups, "Bonus Actions"), "Rage") {
		t.Errorf("Rage should not appear for a wizard")
	}
}

func TestBuildActionGroups_ClassGatedAppears(t *testing.T) {
	groups := buildActionGroups([]character.ClassEntry{{Class: "Barbarian", Level: 1}})
	bonus := findGroup(groups, "Bonus Actions")
	if !hasAction(bonus, "Rage") {
		t.Fatalf("Rage missing for barbarian: %+v", bonus)
	}
	for _, a := range bonus.Actions {
		if a.Name != "Rage" {
			continue
		}
		if a.Command != "/bonus rage" {
			t.Errorf("Rage command = %q, want /bonus rage", a.Command)
		}
		if a.Classes != "Barbarian" {
			t.Errorf("Rage gating label = %q, want Barbarian", a.Classes)
		}
	}
}

func TestBuildActionGroups_MinLevelGate(t *testing.T) {
	// Rogue 1 has not earned Cunning Action (level 2).
	if hasAction(findGroup(buildActionGroups([]character.ClassEntry{{Class: "Rogue", Level: 1}}), "Bonus Actions"), "Cunning Action") {
		t.Errorf("Cunning Action should require rogue level 2")
	}
	// Rogue 2 has it.
	if !hasAction(findGroup(buildActionGroups([]character.ClassEntry{{Class: "Rogue", Level: 2}}), "Bonus Actions"), "Cunning Action") {
		t.Errorf("Cunning Action should appear for rogue level 2")
	}
}

func TestBuildActionGroups_MultiClassLabel(t *testing.T) {
	groups := buildActionGroups([]character.ClassEntry{{Class: "Cleric", Level: 3}})
	actions := findGroup(groups, "Actions")
	for _, a := range actions.Actions {
		if a.Name == "Channel Divinity" {
			if a.Classes != "Cleric / Paladin" {
				t.Errorf("Channel Divinity gating label = %q, want 'Cleric / Paladin'", a.Classes)
			}
			return
		}
	}
	t.Errorf("Channel Divinity missing for cleric 3")
}

func TestBuildActionGroups_GroupOrderAndOnlyNonEmpty(t *testing.T) {
	groups := buildActionGroups([]character.ClassEntry{{Class: "Fighter", Level: 3}})
	// Expected order of the labels that are present.
	want := []string{"Actions", "Bonus Actions", "Reactions", "Other"}
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.Economy
		if len(g.Actions) == 0 {
			t.Errorf("group %q is empty but was emitted", g.Economy)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("group labels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("group order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildActionGroups_NoClassesStillUniversal(t *testing.T) {
	groups := buildActionGroups(nil)
	if !hasAction(findGroup(groups, "Actions"), "Dash") {
		t.Errorf("universal Dash missing when character has no classes")
	}
	// Universal bonus action (off-hand attack) is always present.
	if !hasAction(findGroup(groups, "Bonus Actions"), "Off-Hand Attack") {
		t.Errorf("universal Off-Hand Attack missing when character has no classes")
	}
}
