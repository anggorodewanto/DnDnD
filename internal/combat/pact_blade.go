package combat

// pactOfTheBladeEffectID and the pact-weapon invocation slugs are the clean-slug
// mechanical_effects the builder writes (refdata.invocation_catalog). They ride
// the same features JSONB column as feats/class features, so HasInvocation matches
// them. lifedrinker + thirsting_blade both require Pact of the Blade (COV-6).
const (
	pactOfTheBladeEffectID = "pact_of_the_blade"
	lifedrinkerEffectID    = "lifedrinker"
	thirstingBladeEffectID = "thirsting_blade"
)

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
