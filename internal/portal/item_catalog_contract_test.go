package portal

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

// TestItemCatalog_CoversAllBuilderEquipmentIDs is the drift guard for ISSUE-017:
// every concrete equipment id the builder can seed — from class starting
// equipment (classStartingEquipment) and from backgrounds (backgroundEquipmentByID)
// — must resolve to a row in the canonical item catalog (refdata.ItemCatalog).
//
// If a new id is added to starting_equipment.go or backgrounds.json without a
// matching catalog entry, this fails CI, the same way ISSUE-013's
// TestBackground*_AllBuilderBackgrounds contract tests lock background slugs.
// This is what kills the slug/type/quantity drift class permanently.
func TestItemCatalog_CoversAllBuilderEquipmentIDs(t *testing.T) {
	catalog := refdata.ItemCatalogByID()

	seen := make(map[string]bool)
	var missing []string
	check := func(raw string) {
		// A token may batch comma-separated items ("light-crossbow:1,crossbow-bolt:20")
		// and each may carry a ":N" quantity — split both, mirroring
		// EquipmentToInventoryWithEquipped.
		for _, entry := range strings.Split(raw, ",") {
			id, _, _ := parseEquipmentEntry(strings.TrimSpace(entry))
			if id == "" || strings.HasPrefix(id, "any-") || seen[id] {
				continue // skip empty / "any-*" choice placeholders
			}
			seen[id] = true
			if _, ok := catalog[id]; !ok {
				missing = append(missing, id)
			}
		}
	}

	for _, packs := range classStartingEquipment {
		for _, pack := range packs {
			for _, choice := range pack.Choices {
				for _, opt := range choice.Options {
					check(opt)
				}
			}
			for _, g := range pack.Guaranteed {
				check(g)
			}
		}
	}
	for _, equip := range backgroundEquipmentByID {
		for _, id := range equip {
			check(id)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("item catalog missing %d builder equipment id(s): %v", len(missing), missing)
	}
}
