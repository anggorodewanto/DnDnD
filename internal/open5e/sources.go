package open5e

// Source describes a single Open5e document slug that the DM can enable
// for a campaign. The slug is the value Open5e itself returns in
// document__slug fields and matches what the per-campaign filter compares
// against (see filter.go / campaign_lookup.go).
type Source struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Publisher   string `json:"publisher,omitempty"`
	Description string `json:"description,omitempty"`
}

// catalog is the curated list of Open5e document slugs the dashboard
// surfaces in the per-campaign toggle UI. The list intentionally lives in
// Go (not Postgres or JS) so the backend remains the single source of
// truth — the Svelte panel fetches it via GET /api/open5e/sources.
//
// Slugs match the canonical Open5e API document__slug values; titles and
// publisher names are taken from each publisher's listed OGL/CC content.
var catalog = []Source{
	{Slug: "wotc-srd", Title: "5e SRD (Wizards of the Coast)", Publisher: "Wizards of the Coast", Description: "Core 5e System Reference Document content."},
	{Slug: "tome-of-beasts", Title: "Tome of Beasts", Publisher: "Kobold Press", Description: "400+ new monsters."},
	{Slug: "creature-codex", Title: "Creature Codex", Publisher: "Kobold Press", Description: "Additional monsters and NPCs."},
	{Slug: "deep-magic", Title: "Deep Magic", Publisher: "Kobold Press", Description: "Hundreds of new spells across magic schools."},
	{Slug: "tome-of-beasts-2", Title: "Tome of Beasts 2", Publisher: "Kobold Press", Description: "Further bestiary expansion (Tome of Beasts II)."},
	{Slug: "tome-of-beasts-3", Title: "Tome of Beasts 3", Publisher: "Kobold Press", Description: "Tome of Beasts III bestiary."},
	{Slug: "menagerie", Title: "Level Up Advanced 5e Monstrous Menagerie", Publisher: "EN Publishing", Description: "Advanced 5e monsters."},
	{Slug: "a5e", Title: "Level Up Advanced 5e", Publisher: "EN Publishing", Description: "Advanced 5e core content."},
	{Slug: "vom", Title: "Vault of Magic", Publisher: "Kobold Press", Description: "Magic items collection."},
	{Slug: "kp", Title: "Kobold Press Open Content", Publisher: "Kobold Press", Description: "Misc open Kobold Press content."},
	{Slug: "toh", Title: "Tome of Heroes", Publisher: "Kobold Press", Description: "Player-facing options and lineages."},
}

// Catalog returns a copy of the curated Open5e source list the dashboard
// surfaces to DMs. Callers may freely mutate the returned slice.
func Catalog() []Source {
	out := make([]Source, len(catalog))
	copy(out, catalog)
	return out
}

// CatalogSlugs returns just the slug strings from Catalog, in the same
// order. Useful for validation paths that only need the allow-list.
func CatalogSlugs() []string {
	out := make([]string, 0, len(catalog))
	for _, s := range catalog {
		out = append(out, s.Slug)
	}
	return out
}

// IsKnownSource reports whether the given slug exists in the curated
// catalog. Empty strings and unknown slugs return false.
func IsKnownSource(slug string) bool {
	if slug == "" {
		return false
	}
	for _, s := range catalog {
		if s.Slug == slug {
			return true
		}
	}
	return false
}
