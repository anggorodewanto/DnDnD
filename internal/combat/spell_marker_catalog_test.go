package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

// TestSpellMarkerDef_MatchesActionCatalog is a drift guard between the DM
// dashboard's accepted spell-marker keys (spellMarkerDef) and the action
// catalog's spell-gated entries (refdata.ActionCatalog Spells). Every catalog
// spell id that drives a /bonus move must also be a marker the dashboard
// endpoint can stamp, so the two ways to place/move a marker never diverge.
func TestSpellMarkerDef_MatchesActionCatalog(t *testing.T) {
	for _, e := range refdata.ActionCatalog() {
		for _, spellID := range e.Spells {
			if _, _, ok := spellMarkerDef(spellID); !ok {
				t.Errorf("catalog entry %q lists spell %q that spellMarkerDef does not accept — the dashboard cannot stamp a marker the /bonus move relies on", e.Key, spellID)
			}
		}
	}
}
