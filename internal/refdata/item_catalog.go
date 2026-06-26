package refdata

// ItemCatalogEntry is one row of the canonical item catalog — the SSOT that
// gives every equipment id a uniform name / category / default quantity.
//
// This kills the slug/type/quantity drift class (issues.md ISSUE-013, -015):
// before this, item metadata was scattered across the portal builder's
// hand-maintained knownWeapons/knownArmor/knownAmmo maps, a parallel JS
// fallback set, and combat's "Bolts"/"Arrows" substring heuristics, with ammo
// and adventuring gear having no canonical row at all.
//
// Weapons and armor keep their detailed stat tables (weapons, armor); the
// catalog derives their name/category from the same seed slices, so a weapon's
// display name lives in exactly one place. Ammunition and gear, which had no
// home before, are authored here.
type ItemCatalogEntry struct {
	ID              string
	Name            string
	Category        string // "weapon" | "armor" | "ammunition" | "gear"
	DefaultQuantity int
	Stackable       bool
}

// ammoCatalog holds the SRD ammunition ids. DefaultQuantity is the size of a
// fresh bundle (used when a starting-equipment entry carries no ":N" override).
var ammoCatalog = []ItemCatalogEntry{
	{ID: "crossbow-bolt", Name: "Crossbow Bolts", Category: "ammunition", DefaultQuantity: 20, Stackable: true},
	{ID: "arrow", Name: "Arrows", Category: "ammunition", DefaultQuantity: 20, Stackable: true},
	{ID: "sling-bullet", Name: "Sling Bullets", Category: "ammunition", DefaultQuantity: 20, Stackable: true},
	{ID: "blowgun-needle", Name: "Blowgun Needles", Category: "ammunition", DefaultQuantity: 50, Stackable: true},
}

// gearCatalog holds the adventuring gear / packs / tools / foci / clothing
// referenced by starting_equipment.go and backgrounds_gen.go. Every concrete
// (non-"any-") id used by the builder MUST appear here or in ammoCatalog /
// the weapon+armor seeds — the contract test
// TestItemCatalog_CoversAllBuilderEquipmentIDs (internal/portal) enforces it,
// so future drift fails CI the way ISSUE-013's background contract tests do.
var gearCatalog = []ItemCatalogEntry{
	// Equipment packs.
	{ID: "explorers-pack", Name: "Explorer's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "diplomats-pack", Name: "Diplomat's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "entertainers-pack", Name: "Entertainer's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "priests-pack", Name: "Priest's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "dungeoneers-pack", Name: "Dungeoneer's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "burglars-pack", Name: "Burglar's Pack", Category: "gear", DefaultQuantity: 1},
	{ID: "scholars-pack", Name: "Scholar's Pack", Category: "gear", DefaultQuantity: 1},

	// Spellcasting foci / arcane gear.
	{ID: "druidic-focus", Name: "Druidic Focus", Category: "gear", DefaultQuantity: 1},
	{ID: "arcane-focus", Name: "Arcane Focus", Category: "gear", DefaultQuantity: 1},
	{ID: "component-pouch", Name: "Component Pouch", Category: "gear", DefaultQuantity: 1},
	{ID: "spellbook", Name: "Spellbook", Category: "gear", DefaultQuantity: 1},
	{ID: "holy-symbol", Name: "Holy Symbol", Category: "gear", DefaultQuantity: 1},

	// Tools & kits.
	{ID: "thieves-tools", Name: "Thieves' Tools", Category: "gear", DefaultQuantity: 1},
	{ID: "disguise-kit", Name: "Disguise Kit", Category: "gear", DefaultQuantity: 1},
	{ID: "con-tools", Name: "Con Tools", Category: "gear", DefaultQuantity: 1},
	{ID: "artisans-tools", Name: "Artisan's Tools", Category: "gear", DefaultQuantity: 1},
	{ID: "herbalism-kit", Name: "Herbalism Kit", Category: "gear", DefaultQuantity: 1},
	{ID: "dice-set", Name: "Set of Dice", Category: "gear", DefaultQuantity: 1},
	{ID: "musical-instrument", Name: "Musical Instrument", Category: "gear", DefaultQuantity: 1},
	{ID: "hunting-trap", Name: "Hunting Trap", Category: "gear", DefaultQuantity: 1},

	// Clothing.
	{ID: "vestments", Name: "Vestments", Category: "gear", DefaultQuantity: 1},
	{ID: "common-clothes", Name: "Common Clothes", Category: "gear", DefaultQuantity: 1},
	{ID: "fine-clothes", Name: "Fine Clothes", Category: "gear", DefaultQuantity: 1},
	{ID: "dark-common-clothes", Name: "Dark Common Clothes", Category: "gear", DefaultQuantity: 1},
	{ID: "travelers-clothes", Name: "Traveler's Clothes", Category: "gear", DefaultQuantity: 1},
	{ID: "costume", Name: "Costume", Category: "gear", DefaultQuantity: 1},

	// Books, papers & writing.
	{ID: "prayer-book", Name: "Prayer Book", Category: "gear", DefaultQuantity: 1},
	{ID: "scroll-case", Name: "Scroll Case", Category: "gear", DefaultQuantity: 1},
	{ID: "ink-bottle", Name: "Bottle of Ink", Category: "gear", DefaultQuantity: 1},
	{ID: "quill", Name: "Quill", Category: "gear", DefaultQuantity: 1},
	{ID: "letter", Name: "Letter", Category: "gear", DefaultQuantity: 1},
	{ID: "letter-of-introduction", Name: "Letter of Introduction", Category: "gear", DefaultQuantity: 1},
	{ID: "scroll-of-pedigree", Name: "Scroll of Pedigree", Category: "gear", DefaultQuantity: 1},
	{ID: "map-of-city", Name: "Map of your City", Category: "gear", DefaultQuantity: 1},

	// Miscellaneous adventuring gear.
	{ID: "incense", Name: "Incense", Category: "gear", DefaultQuantity: 1, Stackable: true},
	{ID: "crowbar", Name: "Crowbar", Category: "gear", DefaultQuantity: 1},
	{ID: "shovel", Name: "Shovel", Category: "gear", DefaultQuantity: 1},
	{ID: "iron-pot", Name: "Iron Pot", Category: "gear", DefaultQuantity: 1},
	{ID: "winter-blanket", Name: "Winter Blanket", Category: "gear", DefaultQuantity: 1},
	{ID: "signet-ring", Name: "Signet Ring", Category: "gear", DefaultQuantity: 1},
	{ID: "staff", Name: "Staff", Category: "gear", DefaultQuantity: 1},
	{ID: "trophy", Name: "Trophy", Category: "gear", DefaultQuantity: 1},
	{ID: "small-knife", Name: "Small Knife", Category: "gear", DefaultQuantity: 1},
	{ID: "belaying-pin", Name: "Belaying Pin", Category: "gear", DefaultQuantity: 1},
	{ID: "silk-rope-50ft", Name: "Silk Rope (50 ft)", Category: "gear", DefaultQuantity: 1},
	{ID: "insignia-of-rank", Name: "Insignia of Rank", Category: "gear", DefaultQuantity: 1},
	{ID: "pet-mouse", Name: "Pet Mouse", Category: "gear", DefaultQuantity: 1},
	{ID: "token", Name: "Token of Remembrance", Category: "gear", DefaultQuantity: 1},
}

// ItemCount is the number of rows in the canonical item catalog (and thus the
// items table after seeding). Derived from ItemCatalog so it never drifts from
// the data, mirroring the WeaponCount/ArmorCount/... constants in seeder.go.
var ItemCount = len(ItemCatalog())

// ItemCatalog returns the full canonical catalog: a row for every equipment id.
// Weapon and armor rows are derived from the same seed slices that populate the
// weapons/armor tables (single source for their display names); ammunition and
// gear rows are authored in ammoCatalog / gearCatalog.
func ItemCatalog() []ItemCatalogEntry {
	weapons := weaponSeeds()
	armor := armorSeeds()
	out := make([]ItemCatalogEntry, 0, len(weapons)+len(armor)+len(ammoCatalog)+len(gearCatalog))
	for _, w := range weapons {
		out = append(out, ItemCatalogEntry{ID: w.ID, Name: w.Name, Category: "weapon", DefaultQuantity: 1})
	}
	for _, a := range armor {
		out = append(out, ItemCatalogEntry{ID: a.ID, Name: a.Name, Category: "armor", DefaultQuantity: 1})
	}
	out = append(out, ammoCatalog...)
	out = append(out, gearCatalog...)
	return out
}

// ItemCatalogByID returns the catalog keyed by id for O(1) lookups.
func ItemCatalogByID() map[string]ItemCatalogEntry {
	entries := ItemCatalog()
	byID := make(map[string]ItemCatalogEntry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	return byID
}
