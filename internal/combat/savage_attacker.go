package combat

import (
	"strings"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// savageAttackerUsedEffect is the once-per-turn key recorded in the usedEffects
// tracker when Savage Attacker rerolls a melee weapon's damage dice. ResolveAttack
// appends it to AttackResult.OncePerTurnEffectsFired, which Service.Attack /
// OffhandAttack / GWMBonusAttack already mark used — so a creature rerolls at most
// once per turn, mirroring the Sneak Attack / Cleave / Nick once-per-turn keys.
const savageAttackerUsedEffect = "savage_attacker"

// featsHaveName reports whether any already-parsed feature matches name
// (case-insensitive), mirroring HasFeatureByName's match but scanning a
// caller-parsed slice. populateAttackFES already unmarshals char.Features into a
// []CharacterFeature, so this avoids a second json.Unmarshal of the same bytes on
// the attack hot path.
func featsHaveName(feats []CharacterFeature, name string) bool {
	for _, f := range feats {
		if strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

// savageAttackerEligible reports whether Savage Attacker should reroll this
// attack's weapon damage: the attacker has the feat (input.SavageAttacker set by
// populateAttackFES), the attack is a melee weapon attack (the seeded feat is
// melee-only), and the reroll has not already been spent this turn. Keeping the
// melee + once-per-turn gates here means every attack path that funnels through
// ResolveAttack (Attack, OffhandAttack, GWMBonusAttack) shares one decision.
func savageAttackerEligible(input AttackInput, isMelee bool) bool {
	if !input.SavageAttacker || !isMelee {
		return false
	}
	return !input.UsedThisTurn[savageAttackerUsedEffect]
}

// rollWeaponDamageSavage rolls the weapon's damage and, when savage is true, rolls
// it a second time and keeps the higher total — Savage Attacker's "reroll the
// weapon's damage dice and use either total". The reroll covers the full
// (crit-doubled, when critical) weapon dice; the flat damage modifier is constant
// across both rolls, so the higher total is always the higher dice result. It
// returns the chosen total, its dice display, and roll result — the same shape as
// resolveWeaponDamage, so call sites swap it in transparently.
func rollWeaponDamageSavage(savage bool, weapon refdata.Weapon, dmgMod int, critical, twoHanded bool, roller *dice.Roller, monkLevel int) (int, string, *dice.RollResult) {
	dmg, diceStr, roll := resolveWeaponDamage(weapon, dmgMod, critical, twoHanded, roller, monkLevel)
	if !savage {
		return dmg, diceStr, roll
	}
	dmg2, diceStr2, roll2 := resolveWeaponDamage(weapon, dmgMod, critical, twoHanded, roller, monkLevel)
	if dmg2 > dmg {
		return dmg2, diceStr2, roll2
	}
	return dmg, diceStr, roll
}

// savageAttackerTag returns the player-visible Savage Attacker suffix when the
// attacker's Savage Attacker reroll fired on this attack, or "" otherwise.
// Mirrors sneakAttackTag — appended to the #combat-log damage line.
func savageAttackerTag(result AttackResult) string {
	if !result.SavageAttackerUsed {
		return ""
	}
	return " — \U0001f3b2 Savage Attacker!"
}
