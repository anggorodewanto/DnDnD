package combat

import "fmt"

// Frenzy (2024 Barbarian, Path of the Berserker, level 3): "If you use Reckless
// Attack while your Rage is active, you deal extra damage to the first target
// you hit on your turn with a Strength-based attack. To determine the extra
// damage, roll a number of d6s equal to your Rage Damage bonus, and add them
// together. The damage has the same type as the weapon or Unarmed Strike used
// for the attack."
//
// This is a passive on-hit rider, not a command: nothing is declared, no action
// economy is spent, and there is no /bonus subcommand. It rides the Feature
// Effect System as an EffectExtraDamageDice (FrenzyFeature, crit-doubled like
// Hex / Brutal Strike), injected in populateAttackFES when frenzyEligible passes.
//
// Note the gate is "Strength-based attack", NOT "Strength-based melee attack"
// (which is how the sibling Brutal Strike is worded) — so a thrown weapon that
// uses Strength qualifies and there is deliberately no melee restriction here.

// frenzyUsedEffect is the once-per-turn key recorded in the usedEffects tracker
// when Frenzy's extra dice land. ResolveAttack appends it to
// AttackResult.OncePerTurnEffectsFired on a hit, which Service.Attack /
// OffhandAttack / GWMBonusAttack already mark used — so "the first target you
// hit on your turn" holds across every attack path, mirroring the Savage
// Attacker / Sneak Attack keys.
//
// A dedicated key (rather than Effect.Conditions.OncePerTurn) is required
// because that flag keys on the effect TYPE, which Sneak Attack and Hex share —
// setting it here would silently disable them. See the note in brutal_strike.go.
const frenzyUsedEffect = "frenzy"

// FrenzyFeature returns the FeatureDefinition for Frenzy's extra on-hit damage:
// Nd6 where N is the barbarian's Rage Damage bonus, of the weapon's (or Unarmed
// Strike's) damage type. Mirrors BrutalStrikeFeature — an EffectExtraDamageDice
// on TriggerOnDamageRoll, so it flows through buildFESDamageBreakdown and is
// crit-doubled like any extra damage dice.
func FrenzyFeature(rageDamageBonus int, damageType string) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Frenzy",
		Source: "barbarian",
		Effects: []Effect{
			{
				Type:        EffectExtraDamageDice,
				Trigger:     TriggerOnDamageRoll,
				Dice:        fmt.Sprintf("%dd6", rageDamageBonus),
				DamageTypes: []string{damageType},
			},
		},
	}
}

// frenzyEligible reports whether Frenzy's extra dice may ride this attack: the
// Rage is active, the attacker is using Reckless Attack (declared this attack OR
// the transient reckless marker from an earlier attack this turn — the same
// idiom brutalStrikeEligible uses), the attack is Strength-based, the
// once-per-turn allowance is unspent, and the attacker carries the feature.
//
// Detection is by feature NAME rather than by mechanical_effect slug. Frenzy is
// a subclass feature, and per the project's known "seeded features don't
// backfill" behaviour an already-created Berserker keeps whatever slug their
// features JSON was written with — but the name is "Frenzy" either way, so the
// name check is the one gate that fires for both existing and freshly-built
// characters. It also costs nothing: populateAttackFES has already unmarshalled
// char.Features into the []CharacterFeature this scans (see featsHaveName).
func frenzyEligible(cmd AttackCommand, feats []CharacterFeature, abilityUsed string, isRaging bool, usedThisTurn map[string]bool) bool {
	if !isRaging {
		return false
	}
	usingReckless := cmd.Reckless || HasCondition(cmd.Attacker.Conditions, "reckless")
	if !usingReckless {
		return false
	}
	if abilityUsed != "str" {
		return false
	}
	if usedThisTurn[frenzyUsedEffect] {
		return false
	}
	return featsHaveName(feats, "Frenzy")
}

// recordFrenzy spends Frenzy's once-per-turn allowance on a hit. The extra dice
// themselves already rode input.Features through buildFESDamageBreakdown; this
// only stamps the key so the next attack this turn skips the rider. Called from
// both of ResolveAttack's hit paths (auto-crit and rolled hit) — RAW ties the
// allowance to "the first target you HIT", so a miss leaves it available.
func recordFrenzy(result *AttackResult, input AttackInput) {
	if !input.Frenzy || !result.Hit {
		return
	}
	result.OncePerTurnEffectsFired = append(result.OncePerTurnEffectsFired, frenzyUsedEffect)
}
