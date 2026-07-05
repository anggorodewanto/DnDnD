package refdata

import (
	"strings"
	"testing"
)

// TestActionCatalog_NoDuplicateKeys guards the SSOT invariant: every entry's
// Key is unique so ActionCatalogByKey is lossless and the portal/dispatch
// contract test can map keys 1:1.
func TestActionCatalog_NoDuplicateKeys(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range ActionCatalog() {
		if seen[e.Key] {
			t.Errorf("duplicate action key %q", e.Key)
		}
		seen[e.Key] = true
	}
}

// TestActionCatalog_EntriesWellFormed asserts every row carries the fields the
// character sheet renders (Key/Name/Command/Summary) and a known economy, and
// that gating is internally consistent (universal rows carry no class gate;
// non-universal rows name at least one class).
func TestActionCatalog_EntriesWellFormed(t *testing.T) {
	valid := map[ActionEconomy]bool{
		EconomyAction:      true,
		EconomyBonusAction: true,
		EconomyReaction:    true,
		EconomyFree:        true,
	}
	for _, e := range ActionCatalog() {
		if strings.TrimSpace(e.Key) == "" {
			t.Errorf("entry %+v has empty Key", e)
		}
		if strings.TrimSpace(e.Name) == "" {
			t.Errorf("entry %q has empty Name", e.Key)
		}
		if strings.TrimSpace(e.Command) == "" {
			t.Errorf("entry %q has empty Command", e.Key)
		}
		if strings.TrimSpace(e.Summary) == "" {
			t.Errorf("entry %q has empty Summary", e.Key)
		}
		if !valid[e.Economy] {
			t.Errorf("entry %q has unknown economy %q", e.Key, e.Economy)
		}
		if e.Universal && len(e.Classes) > 0 {
			t.Errorf("entry %q is Universal but lists classes %v", e.Key, e.Classes)
		}
		if !e.Universal && len(e.Classes) == 0 && len(e.Feats) == 0 {
			t.Errorf("entry %q is not Universal and lists no gating class or feat", e.Key)
		}
		for _, c := range e.Classes {
			if c != strings.ToLower(c) {
				t.Errorf("entry %q gating class %q must be lower-case", e.Key, c)
			}
		}
	}
}

// TestActionCatalog_ByKeyRoundTrips asserts ActionCatalogByKey contains every
// catalog row keyed by its Key.
func TestActionCatalog_ByKeyRoundTrips(t *testing.T) {
	byKey := ActionCatalogByKey()
	all := ActionCatalog()
	if len(byKey) != len(all) {
		t.Fatalf("ActionCatalogByKey len=%d, ActionCatalog len=%d", len(byKey), len(all))
	}
	for _, e := range all {
		got, ok := byKey[e.Key]
		if !ok {
			t.Errorf("key %q missing from ActionCatalogByKey", e.Key)
			continue
		}
		if got.Name != e.Name {
			t.Errorf("key %q: ByKey name %q != catalog name %q", e.Key, got.Name, e.Name)
		}
	}
}

// TestActionCatalog_HasCoreUniversalActions spot-checks that the always-on
// actions every character can take are present and flagged universal.
func TestActionCatalog_HasCoreUniversalActions(t *testing.T) {
	byKey := ActionCatalogByKey()
	for _, key := range []string{"dash", "disengage", "dodge", "help", "hide", "attack"} {
		e, ok := byKey[key]
		if !ok {
			t.Errorf("core universal action %q missing from catalog", key)
			continue
		}
		if !e.Universal {
			t.Errorf("core action %q should be Universal", key)
		}
	}
}

// TestAvailableActions_UniversalAlwaysPresent asserts a character with no
// matching class still sees every universal action.
func TestAvailableActions_UniversalAlwaysPresent(t *testing.T) {
	got := AvailableActions(map[string]int{"wizard": 5})
	gotKeys := keySet(got)

	for _, e := range ActionCatalog() {
		if e.Universal && !gotKeys[e.Key] {
			t.Errorf("universal action %q missing for wizard", e.Key)
		}
	}
	// A class-gated barbarian action must NOT show for a wizard.
	if gotKeys["rage"] {
		t.Errorf("rage should not be available to a wizard")
	}
}

// TestAvailableActions_ClassGating asserts a class-gated action appears only
// when the character has the class at or above the required level.
func TestAvailableActions_ClassGating(t *testing.T) {
	// Rogue 1 has not yet earned Cunning Action (level 2).
	if keySet(AvailableActions(map[string]int{"rogue": 1}))["cunning-action"] {
		t.Errorf("cunning-action should require rogue level 2")
	}
	// Rogue 2 has it.
	if !keySet(AvailableActions(map[string]int{"rogue": 2}))["cunning-action"] {
		t.Errorf("cunning-action should be available to a rogue level 2")
	}
	// Barbarian 1 has Rage.
	if !keySet(AvailableActions(map[string]int{"barbarian": 1}))["rage"] {
		t.Errorf("rage should be available to a barbarian level 1")
	}
}

// TestAvailableActions_ClassMatchCaseInsensitive guards that gating matches the
// stored ClassEntry casing regardless of case (classes are matched with
// EqualFold across the combat package).
func TestAvailableActions_ClassMatchCaseInsensitive(t *testing.T) {
	if !keySet(AvailableActions(map[string]int{"Barbarian": 3}))["rage"] {
		t.Errorf("rage gating should be case-insensitive on class name")
	}
}

// TestAvailableActions_PreservesCatalogOrder asserts the filtered slice keeps
// the catalog's declaration order so rendering is deterministic.
func TestAvailableActions_PreservesCatalogOrder(t *testing.T) {
	got := AvailableActions(map[string]int{"barbarian": 5})
	lastIdx := -1
	order := map[string]int{}
	for i, e := range ActionCatalog() {
		order[e.Key] = i
	}
	for _, e := range got {
		idx := order[e.Key]
		if idx < lastIdx {
			t.Fatalf("AvailableActions out of catalog order at %q", e.Key)
		}
		lastIdx = idx
	}
}

func keySet(entries []ActionCatalogEntry) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Key] = true
	}
	return out
}
