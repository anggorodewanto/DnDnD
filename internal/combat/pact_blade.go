package combat

// pactOfTheBladeEffectID is the clean-slug mechanical_effect the builder writes
// for the Pact of the Blade boon (refdata.invocation_catalog). It rides the same
// features JSONB column as invocations/feats, so HasInvocation matches it.
const pactOfTheBladeEffectID = "pact_of_the_blade"

// effectiveAbilityMod returns the ability modifier for a weapon attack, applying
// the Pact of the Blade CHA substitution when input.PactBladeCHA is set. 2024
// Pact of the Blade lets the warlock "use Charisma" for a pact weapon's attack
// and damage rolls; the choice is taken player-optimally as the higher of the
// weapon's normal ability (STR/DEX/finesse/monk) and CHA. Without the boon it is
// exactly abilityModForWeapon.
func effectiveAbilityMod(input AttackInput) int {
	base := abilityModForWeapon(input.Scores, input.Weapon, input.MonkLevel)
	if input.PactBladeCHA {
		return max(base, AbilityModifier(input.Scores.Cha))
	}
	return base
}
