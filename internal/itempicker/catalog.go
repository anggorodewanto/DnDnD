package itempicker

import "github.com/ab/dndnd/internal/refdata"

// CatalogAmmunition returns the ammunition rows of the canonical item catalog
// (refdata SSOT) as picker entries. Sourcing from the catalog — rather than a
// hand-kept static list — means the ID matches what characters already store
// (e.g. "crossbow-bolt"), so a DM "add" call stacks onto the existing bundle
// instead of creating an id-less duplicate row.
func CatalogAmmunition() []AmmunitionItem {
	out := []AmmunitionItem{}
	for _, e := range refdata.ItemCatalog() {
		if e.Category != "ammunition" {
			continue
		}
		out = append(out, AmmunitionItem{ID: e.ID, Name: e.Name})
	}
	return out
}

// MergedGear returns the adventuring-gear picker list: the descriptive built-in
// StaticGear plus every "gear" row from the canonical catalog that the static
// list doesn't already cover. Deduplicated by ID with the static entry winning
// (it carries description + cost), so packs / foci / tools / clothing that only
// live in the catalog (e.g. "arcane-focus", "dungeoneers-pack") become pickable
// while the richer static rows keep their flavor text.
func MergedGear() []GearItem {
	static := StaticGear()
	seen := make(map[string]struct{}, len(static))
	out := make([]GearItem, 0, len(static))
	for _, g := range static {
		seen[g.ID] = struct{}{}
		out = append(out, g)
	}
	for _, e := range refdata.ItemCatalog() {
		if e.Category != "gear" {
			continue
		}
		if _, ok := seen[e.ID]; ok {
			continue
		}
		seen[e.ID] = struct{}{}
		out = append(out, GearItem{ID: e.ID, Name: e.Name})
	}
	return out
}
