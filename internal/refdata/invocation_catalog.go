package refdata

// invocation_catalog.go is the canonical SSOT for Warlock Pact Boons and
// Eldritch Invocations (2014 PHB rules, to match the warlock class seed which
// grants Pact Boon at level 3 and Eldritch Invocations at level 2).
//
// Like item_catalog.go this Go slice is the single source of truth: a
// //go:generate step (scripts/gen_invocations_catalog) emits it to
// portal/svelte/src/lib/invocations-catalog.json so the builder picker and the
// backend validation/resolution read the same data. A make …-check drift guard
// keeps the two in sync (mirroring make items-catalog-check).
//
// SLUG CONTRACT: ids are lowercase-underscore slugs, and an invocation id
// doubles as the character.Feature.MechanicalEffect the combat engine matches
// (agonizing_blast -> internal/combat/agonizing_blast.go). Resolving a picked
// invocation therefore persists a Feature{MechanicalEffect: <invocation id>},
// a CLEAN slug — never the JSON-array mechanical_effect shape the level-up feat
// path writes (which the equality-based combat matchers can never match).

// ChoosePactBoonEffect is the mechanical_effect the warlock class seed writes
// for the unresolved Pact Boon feature (seed_classes.go). It is the SSOT for
// that placeholder string: the portal resolver strips a feature carrying this
// effect once the player picks a boon, so both the seed and the stripper must
// reference this one constant or a resolved character keeps a stale placeholder.
const ChoosePactBoonEffect = "choose_pact_boon"

// Edition tags which D&D 5e ruleset a pact boon / invocation belongs to. An
// EMPTY Edition means the entry is edition-agnostic — present, and mechanically
// the same, in both the 2014 and 2024 PHB (the common case). "2014" / "2024"
// flag entries that exist in only one edition (e.g. Pact of the Talisman is a
// 2014 Tasha's boon dropped from the 2024 base pacts; Eldritch Mind is new in
// 2024).
//
// This is advisory catalog data surfaced as a picker badge. There is no global
// edition selector in the codebase (2024 mechanics elsewhere are baked in and
// toggled per-action, e.g. the /attack gwm2024 flag), so the level-gate helpers
// InvocationsKnown / PactBoonGranted stay 2014-shaped; the tag documents
// provenance and lets the builder show which edition an entry comes from.
const (
	Edition2014 = "2014"
	Edition2024 = "2024"
)

// PactBoon is one of the four gifts a patron can bestow at warlock level 3.
// Pact boons have no mechanical combat consumer yet (they are inert until a
// reader is written), but they gate several invocations via RequiresPactBoon.
type PactBoon struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Edition tags the ruleset this boon belongs to ("" = both, see Edition consts).
	Edition string `json:"edition,omitempty"`
}

// InvocationPrereq models the gates an invocation can carry. Zero value = no
// prerequisite. Fields mirror the feat-prereq pattern (levelup.FeatPrerequisites)
// extended with warlock-specific gates.
type InvocationPrereq struct {
	// MinWarlockLevel is the minimum warlock class level required (0 = none).
	MinWarlockLevel int `json:"min_warlock_level,omitempty"`
	// RequiresPactBoon is a pact-boon id that must be chosen first ("" = none).
	RequiresPactBoon string `json:"requires_pact_boon,omitempty"`
	// RequiresEldritchBlast is true for invocations that improve the eldritch
	// blast cantrip (the character must know eldritch-blast).
	RequiresEldritchBlast bool `json:"requires_eldritch_blast,omitempty"`
}

// Invocation is one Eldritch Invocation. ID is the clean-slug mechanical
// effect the combat engine reads (see slug contract above). GrantsSpells lists
// spell ids the invocation makes castable — wired into the castable spell list
// as a follow-up (P5); the picker + mechanical slugs land first.
type Invocation struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Prereq       InvocationPrereq `json:"prereq"`
	GrantsSpells []string         `json:"grants_spells,omitempty"`
	// Edition tags the ruleset this invocation belongs to ("" = both, see Edition consts).
	Edition string `json:"edition,omitempty"`
}

// pactBoons is the authored pact-boon catalog. Blade/Chain/Tome are the 2014
// PHB boons; Talisman is the Tasha's addition (the doc's "4 pact boons").
var pactBoons = []PactBoon{
	{ID: "pact_of_the_blade", Name: "Pact of the Blade", Description: "You can use your action to create a pact weapon in your empty hand. You are proficient with it while you wield it, and can use your Charisma for its attack and damage rolls."},
	{ID: "pact_of_the_chain", Name: "Pact of the Chain", Description: "You learn the find familiar spell and can cast it as a ritual. Your familiar can take special forms (imp, pseudodragon, quasit, or sprite) and you can forgo an attack to let it use its reaction to attack."},
	{ID: "pact_of_the_tome", Name: "Pact of the Tome", Description: "Your patron gives you a Book of Shadows holding three cantrips of your choice from any class's spell list, which you can cast at will."},
	{ID: "pact_of_the_talisman", Name: "Pact of the Talisman", Description: "Your patron gives you an amulet. While you wear it, you can add a d4 to a failed ability check a number of times per long rest equal to your proficiency bonus.", Edition: Edition2014},
}

// invocations is the authored Eldritch Invocation catalog: a canonical 2014 PHB
// subset (~20) spanning the prereq shapes — eldritch-blast riders, pact-gated,
// level-gated, and freely-taken utility grants.
var invocations = []Invocation{
	// --- Eldritch Blast riders (require the eldritch blast cantrip) ---
	{ID: "agonizing_blast", Name: "Agonizing Blast", Description: "When you cast eldritch blast, add your Charisma modifier to the damage it deals on a hit.", Prereq: InvocationPrereq{RequiresEldritchBlast: true}},
	{ID: "eldritch_spear", Name: "Eldritch Spear", Description: "When you cast eldritch blast, its range is 300 feet.", Prereq: InvocationPrereq{RequiresEldritchBlast: true}},
	{ID: "repelling_blast", Name: "Repelling Blast", Description: "When you hit a creature with eldritch blast, you can push it up to 10 feet away from you in a straight line.", Prereq: InvocationPrereq{RequiresEldritchBlast: true}},

	// --- Freely taken (no prerequisite) ---
	{ID: "armor_of_shadows", Name: "Armor of Shadows", Description: "You can cast mage armor on yourself at will, without expending a spell slot or material components.", GrantsSpells: []string{"mage-armor"}},
	{ID: "beast_speech", Name: "Beast Speech", Description: "You can cast speak with animals at will, without expending a spell slot.", GrantsSpells: []string{"speak-with-animals"}},
	{ID: "beguiling_influence", Name: "Beguiling Influence", Description: "You gain proficiency in the Deception and Persuasion skills."},
	{ID: "devils_sight", Name: "Devil's Sight", Description: "You can see normally in darkness, both magical and nonmagical, to a distance of 120 feet."},
	{ID: "eldritch_sight", Name: "Eldritch Sight", Description: "You can cast detect magic at will, without expending a spell slot.", GrantsSpells: []string{"detect-magic"}},
	{ID: "fiendish_vigor", Name: "Fiendish Vigor", Description: "You can cast false life on yourself at will as a 1st-level spell, without expending a spell slot or material components.", GrantsSpells: []string{"false-life"}},
	{ID: "mask_of_many_faces", Name: "Mask of Many Faces", Description: "You can cast disguise self at will, without expending a spell slot.", GrantsSpells: []string{"disguise-self"}},
	{ID: "misty_visions", Name: "Misty Visions", Description: "You can cast silent image at will, without expending a spell slot or material components.", GrantsSpells: []string{"silent-image"}},
	{ID: "thief_of_five_fates", Name: "Thief of Five Fates", Description: "You can cast bane once using a warlock spell slot. You can't do so again until you finish a long rest.", GrantsSpells: []string{"bane"}},

	// --- Pact-boon gated ---
	{ID: "book_of_ancient_secrets", Name: "Book of Ancient Secrets", Description: "You can inscribe magic ritual spells into your Book of Shadows and cast them as rituals.", Prereq: InvocationPrereq{RequiresPactBoon: "pact_of_the_tome"}},
	{ID: "voice_of_the_chain_master", Name: "Voice of the Chain Master", Description: "You can communicate telepathically with your familiar and perceive through its senses as long as you are on the same plane.", Prereq: InvocationPrereq{RequiresPactBoon: "pact_of_the_chain"}},
	{ID: "thirsting_blade", Name: "Thirsting Blade", Description: "You can attack with your pact weapon twice, instead of once, whenever you take the Attack action on your turn.", Prereq: InvocationPrereq{RequiresPactBoon: "pact_of_the_blade", MinWarlockLevel: 5}},
	{ID: "lifedrinker", Name: "Lifedrinker", Description: "When you hit a creature with your pact weapon, it takes extra necrotic damage equal to your Charisma modifier (minimum 1).", Prereq: InvocationPrereq{RequiresPactBoon: "pact_of_the_blade", MinWarlockLevel: 12}},

	// --- Level gated ---
	{ID: "one_with_shadows", Name: "One with Shadows", Description: "When you are in an area of dim light or darkness, you can use your action to become invisible until you move or take an action or reaction.", Prereq: InvocationPrereq{MinWarlockLevel: 5}},
	{ID: "bewitching_whispers", Name: "Bewitching Whispers", Description: "You can cast compulsion once using a warlock spell slot. You can't do so again until you finish a long rest.", Prereq: InvocationPrereq{MinWarlockLevel: 7}, GrantsSpells: []string{"compulsion"}},
	{ID: "sculptor_of_flesh", Name: "Sculptor of Flesh", Description: "You can cast polymorph once using a warlock spell slot. You can't do so again until you finish a long rest.", Prereq: InvocationPrereq{MinWarlockLevel: 7}, GrantsSpells: []string{"polymorph"}},
	{ID: "ascendant_step", Name: "Ascendant Step", Description: "You can cast levitate on yourself at will, without expending a spell slot or material components.", Prereq: InvocationPrereq{MinWarlockLevel: 9}, GrantsSpells: []string{"levitate"}},

	// --- 2024 PHB additions (new or reworked in the 2024 rules) ---
	// These carry Edition:"2024". Their prereqs map onto the existing struct
	// (MinWarlockLevel / RequiresPactBoon); the boon-gated ones reference the
	// 2014 boon ids since Pact Boon is still a level-3 choice under this seed.
	{ID: "eldritch_mind", Name: "Eldritch Mind", Description: "You have advantage on Constitution saving throws that you make to maintain concentration.", Edition: Edition2024},
	{ID: "lessons_of_the_first_ones", Name: "Lessons of the First Ones", Description: "You have received esoteric lessons from your patron. You gain one Origin feat of your choice. (This invocation can be taken more than once.)", Edition: Edition2024},
	{ID: "otherworldly_leap", Name: "Otherworldly Leap", Description: "You always have the jump spell prepared. You can cast it without expending a spell slot.", Prereq: InvocationPrereq{MinWarlockLevel: 5}, GrantsSpells: []string{"jump"}, Edition: Edition2024},
	{ID: "investment_of_the_chain_master", Name: "Investment of the Chain Master", Description: "Your Pact of the Chain familiar gains a flying and swimming speed of 40 feet, and its attacks count as magical. As a bonus action you can command it to take the Attack action, and you can give it temporary hit points equal to your Warlock level.", Prereq: InvocationPrereq{MinWarlockLevel: 5, RequiresPactBoon: "pact_of_the_chain"}, Edition: Edition2024},
	{ID: "gift_of_the_protectors", Name: "Gift of the Protectors", Description: "When a creature whose name is written in your Book of Shadows is reduced to 0 hit points but not killed outright, that creature instead drops to 1 hit point. Once used, this invocation can't be used again until you finish a long rest.", Prereq: InvocationPrereq{MinWarlockLevel: 9, RequiresPactBoon: "pact_of_the_tome"}, Edition: Edition2024},
}

// PactBoonCatalog returns the authored pact-boon catalog.
func PactBoonCatalog() []PactBoon {
	out := make([]PactBoon, len(pactBoons))
	copy(out, pactBoons)
	return out
}

// InvocationCatalog returns the authored Eldritch Invocation catalog.
func InvocationCatalog() []Invocation {
	out := make([]Invocation, len(invocations))
	copy(out, invocations)
	return out
}

// PactBoonByID returns the pact-boon catalog keyed by id for O(1) lookups.
func PactBoonByID() map[string]PactBoon {
	byID := make(map[string]PactBoon, len(pactBoons))
	for _, b := range pactBoons {
		byID[b.ID] = b
	}
	return byID
}

// InvocationByID returns the invocation catalog keyed by id for O(1) lookups.
func InvocationByID() map[string]Invocation {
	byID := make(map[string]Invocation, len(invocations))
	for _, inv := range invocations {
		byID[inv.ID] = inv
	}
	return byID
}

// InvocationsKnown returns how many Eldritch Invocations a warlock knows at the
// given warlock class level (2014 PHB Warlock table). It scales with warlock
// level specifically, not total character level.
func InvocationsKnown(warlockLevel int) int {
	switch {
	case warlockLevel < 2:
		return 0
	case warlockLevel < 5:
		return 2
	case warlockLevel < 7:
		return 3
	case warlockLevel < 9:
		return 4
	case warlockLevel < 12:
		return 5
	case warlockLevel < 15:
		return 6
	case warlockLevel < 18:
		return 7
	default:
		return 8
	}
}

// PactBoonGranted reports whether the warlock has access to a Pact Boon choice
// (2014 PHB: Pact Boon is a level-3 feature).
func PactBoonGranted(warlockLevel int) bool {
	return warlockLevel >= 3
}
