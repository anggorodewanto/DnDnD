package refdata

// Metamagic is one Metamagic option a Sorcerer picks for the seeded
// "choose_2_metamagic_options" feature. ID is the clean-slug mechanical_effect
// the combat cast path matches verbatim (e.g. "quickened" ->
// internal/combat/metamagic.go validateSingleMetamagicOption + sorcery.go cost
// map). The builder resolves the player's picks into
// character.Feature{MechanicalEffect: ID} entries (internal/portal/metamagic.go),
// exactly as a warlock invocation id doubles as its combat slug, and the cast
// path gates /cast <option> on the character carrying that feature
// (combat.HasMetamagic).
type Metamagic struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChooseMetamagicEffect is the mechanical_effect the Sorcerer class seed writes
// for the unresolved Metamagic feature (Sorcerer L3). The builder swaps it for
// the player's concrete picks (internal/portal/metamagic.go), mirroring
// ChooseFightingStyleEffect / ChoosePactBoonEffect. Seed and stripper share this
// const so they cannot drift.
const ChooseMetamagicEffect = "choose_2_metamagic_options"

// metamagics is the authored Metamagic catalog. It holds ONLY the options the
// combat cast path actually consumes today — so every pickable option produces a
// real cast-time effect rather than a silently-inert one. The 2024 options
// Seeking Spell and Transmuted Spell are deliberately omitted: the cast path has
// no flag, sorcery-point cost, or validator for them, so seeding them would be
// dead data. Add the catalog entry alongside the wiring, never ahead of it
// (dead-data guard, enforced by TestMetamagicCatalog_MatchesWiredCombatSet).
var metamagics = []Metamagic{
	{ID: "careful", Name: "Careful Spell", Description: "When you cast a spell that forces other creatures to make a saving throw, you can protect some of them: a number equal to your Charisma modifier (minimum 1) automatically succeed on their save."},
	{ID: "distant", Name: "Distant Spell", Description: "When you cast a spell that has a range of at least 5 feet, you can double its range; a spell with a range of touch instead gains a range of 30 feet."},
	{ID: "empowered", Name: "Empowered Spell", Description: "When you roll damage for a spell, you can reroll a number of the damage dice up to your Charisma modifier (minimum 1) and must use the new rolls."},
	{ID: "extended", Name: "Extended Spell", Description: "When you cast a spell that has a duration of 1 minute or longer, you can double its duration, to a maximum of 24 hours."},
	{ID: "heightened", Name: "Heightened Spell", Description: "When you cast a spell that forces a creature to make a saving throw, you can give one target of the spell disadvantage on its first save against the spell."},
	{ID: "quickened", Name: "Quickened Spell", Description: "When you cast a spell that has a casting time of an action, you can cast it using a bonus action instead."},
	{ID: "subtle", Name: "Subtle Spell", Description: "When you cast a spell, you can cast it without any verbal or somatic components."},
	{ID: "twinned", Name: "Twinned Spell", Description: "When you cast a spell that targets only one creature and doesn't have a range of self, you can target a second creature in range with the same spell."},
}

// MetamagicCatalog returns the authored metamagic catalog.
func MetamagicCatalog() []Metamagic {
	return metamagics
}

// MetamagicByID indexes the catalog by id for O(1) pick validation/resolution.
func MetamagicByID() map[string]Metamagic {
	byID := make(map[string]Metamagic, len(metamagics))
	for _, m := range metamagics {
		byID[m.ID] = m
	}
	return byID
}

// MetamagicKnown returns how many Metamagic options a Sorcerer knows at the given
// sorcerer class level (2024 PHB: 2 at level 3, +1 at level 10, +1 at level 17).
// It scales with sorcerer level specifically, not total character level, mirroring
// InvocationsKnown. Zero below level 3 (the feature is not yet granted).
func MetamagicKnown(sorcererLevel int) int {
	switch {
	case sorcererLevel < 3:
		return 0
	case sorcererLevel < 10:
		return 2
	case sorcererLevel < 17:
		return 3
	default:
		return 4
	}
}
