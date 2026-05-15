package dashboard

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSidebarNav_LinksUseHashRouting(t *testing.T) {
	// Verify that sidebar links that correspond to Svelte hash routes
	// point to /dashboard/app/#section instead of /dashboard/section.
	hashRouted := map[string]bool{
		"Encounter Builder":  true,
		"Stat Block Library": true,
		"Character Overview": true,
		"Character Approval": true,
		"Map Editor":         true,
		"Asset Library":      true,
	}
	for _, entry := range SidebarNav {
		if hashRouted[entry.Label] {
			if !strings.HasPrefix(entry.Path, "/dashboard/app/#") {
				t.Errorf("sidebar entry %q should use hash routing, got path %q", entry.Label, entry.Path)
			}
		}
	}
}

// mockEncounterLister implements EncounterLister for testing.
type mockEncounterLister struct {
	active []string
	saved  []string
}

func (m *mockEncounterLister) ListActiveEncounterNames(_ context.Context, _ uuid.UUID) ([]string, error) {
	return m.active, nil
}

func (m *mockEncounterLister) ListSavedEncounterNames(_ context.Context, _ uuid.UUID) ([]string, error) {
	return m.saved, nil
}

func TestHandler_LookupEncounters_PopulatesData(t *testing.T) {
	h := NewHandler(nil, nil)
	h.encounterLister = &mockEncounterLister{
		active: []string{"Battle at the Bridge"},
		saved:  []string{"Ambush Template", "Boss Fight"},
	}

	cid := uuid.New().String()
	active, saved := h.lookupEncounters(context.Background(), cid)
	if len(active) != 1 || active[0] != "Battle at the Bridge" {
		t.Errorf("expected 1 active encounter, got %v", active)
	}
	if len(saved) != 2 {
		t.Errorf("expected 2 saved encounters, got %v", saved)
	}
}

func TestHandler_LookupEncounters_NilLister(t *testing.T) {
	h := NewHandler(nil, nil)
	active, saved := h.lookupEncounters(context.Background(), uuid.New().String())
	if len(active) != 0 || len(saved) != 0 {
		t.Errorf("expected empty slices with nil lister, got active=%v saved=%v", active, saved)
	}
}
