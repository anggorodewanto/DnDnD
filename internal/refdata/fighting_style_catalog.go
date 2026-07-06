package refdata

import "strings"

// FightingStyle is one Fighting Style a Fighter/Paladin/Ranger picks for the
// seeded "choose_fighting_style" feature. ID is the clean-slug mechanical_effect
// the combat engine matches verbatim (e.g. "archery" ->
// internal/combat/feature_integration.go ArcheryFeature, "two_weapon_fighting" ->
// attack.go off-hand damage). The builder resolves the player's pick into a
// character.Feature{MechanicalEffect: ID} (internal/portal/fighting_style.go),
// exactly as a warlock invocation id doubles as its combat slug.
type FightingStyle struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChooseFightingStyleEffect is the mechanical_effect the Fighter/Paladin/Ranger
// class seed writes for the unresolved Fighting Style feature. The builder swaps
// it for the player's concrete pick (internal/portal/fighting_style.go), mirroring
// ChoosePactBoonEffect. Seed and stripper share this const so they cannot drift.
const ChooseFightingStyleEffect = "choose_fighting_style"

// fightingStyles is the authored Fighting Style catalog. It holds ONLY the styles
// the combat engine actually consumes today — so every pickable option produces a
// real mechanical effect rather than a silently-inert one. The remaining PHB
// styles (Protection, Blind Fighting, Interception, Thrown Weapon Fighting,
// Unarmed Fighting) are deliberately omitted until each gains a combat rider; add
// the catalog entry alongside the wiring, never ahead of it (dead-data guard,
// enforced by TestFightingStyleCatalog_MatchesWiredCombatSet). Per-class style
// restrictions (2024 limits some styles to certain classes) are not modeled — any
// granting class may pick any wired style, matching the engine's current openness.
var fightingStyles = []FightingStyle{
	{ID: "archery", Name: "Archery", Description: "You gain a +2 bonus to attack rolls you make with Ranged weapons."},
	{ID: "defense", Name: "Defense", Description: "While you wear Light, Medium, or Heavy armor, you gain a +1 bonus to Armor Class."},
	{ID: "dueling", Name: "Dueling", Description: "When you wield a Melee weapon in one hand and no other weapons, you gain a +2 bonus to damage rolls with that weapon."},
	{ID: "great_weapon_fighting", Name: "Great Weapon Fighting", Description: "When you roll a 1 or 2 on a damage die for an attack you make with a Melee weapon that you hold with two hands, you can reroll the die, and you must use the new roll."},
	{ID: "two_weapon_fighting", Name: "Two-Weapon Fighting", Description: "When you make an attack with a weapon in your other hand while Two-Weapon Fighting, you can add your ability modifier to the damage of that attack."},
}

// fightingStyleGrantLevels maps a class id to the level at which it grants the
// Fighting Style feature (the choose_fighting_style seed key: fighter L1,
// paladin/ranger L2 — seed_classes.go). Lives here beside the catalog + placeholder
// const so all fighting-style ref data is co-located, mirroring how the warlock
// grant rule (PactBoonGranted) sits beside the pact-boon catalog.
var fightingStyleGrantLevels = map[string]int{
	"fighter": 1,
	"paladin": 2,
	"ranger":  2,
}

// FightingStyleGrantLevel returns the level at which the given class grants the
// Fighting Style feature, and whether it grants one at all. A class not in the
// table never grants a fighting style.
func FightingStyleGrantLevel(classID string) (level int, granted bool) {
	level, granted = fightingStyleGrantLevels[strings.ToLower(classID)]
	return level, granted
}

// FightingStyleCatalog returns the authored fighting-style catalog.
func FightingStyleCatalog() []FightingStyle {
	return fightingStyles
}

// FightingStyleByID indexes the catalog by id for O(1) pick validation/resolution.
func FightingStyleByID() map[string]FightingStyle {
	byID := make(map[string]FightingStyle, len(fightingStyles))
	for _, s := range fightingStyles {
		byID[s.ID] = s
	}
	return byID
}
