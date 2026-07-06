package combat

import (
	"context"
	"fmt"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// Brutal Strike (2024 Barbarian, level 9): "If you use Reckless Attack, you can
// forgo any Advantage on one Strength-based attack roll of your choice on your
// turn. The attack roll deals an extra 1d10 damage on a hit, and you can cause
// one Brutal Strike effect." This slice wires the Forceful Blow effect: the
// target is pushed 15 ft straight away.
//
// The advantage is forgone in DetectAdvantage (AdvantageInput.ForgoAdvantage,
// set from input.BrutalStrike) — it clears ALL advantage on the roll, per RAW,
// not just the Reckless source; the Reckless "enemies have advantage against you"
// downside is written independently (applyRecklessMarker) and is unaffected. The
// +1d10 rides the Feature Effect System as an EffectExtraDamageDice
// (BrutalStrikeFeature, crit-doubled like Hex), injected in populateAttackFES.
// The effect resolves post-hit in Service.Attack (applyBrutalStrike).
//
// Deferred: Hamstring Blow (Speed −15 until the barbarian's next turn — the
// mastery Slow condition is a hardcoded −10 with no magnitude field, so −15 needs
// a new speed slug, not a table row); the "you may move 5 ft" self-movement rider
// of Forceful Blow; the L13 upgrade (two Brutal Strikes / two effects per turn);
// weapon-vs-generic damage typing subtleties. Two RAW preconditions also stay
// unenforced: (1) the 2024 "the chosen roll can't have Disadvantage" clause —
// eligibility is baked in populateAttackFES BEFORE DetectAdvantage computes
// disadvantage, so a poisoned+reckless barbarian can currently forgo advantage
// into a net-disadvantage roll and still collect +1d10; (2) the once-per-turn cap
// (a barbarian with Extra Attack can declare brutal on more than one attack — each
// still costs its own forgone Advantage). Do NOT "fix" the cap by setting
// OncePerTurn on BrutalStrikeFeature: usedEffects keys on the effect TYPE
// (EffectExtraDamageDice), which Sneak Attack and Hex share, so that would
// silently disable them — the cap needs per-feature-name keying first.

// brutalStrikeEffect is the seeded mechanical_effect slug that gates Brutal
// Strike (Barbarian L9). Detection is by slug via hasFeatureEffect, mirroring the
// Cunning Strike / Tactical Master class-feature gates and the level-9 seed guard.
const brutalStrikeEffect = "brutal_strike"

// brutalForcefulPushSquares is the Forceful Blow push distance: 15 ft = 3 squares
// (the mastery Push is 10 ft / 2 squares — Brutal deliberately does not reuse it).
const brutalForcefulPushSquares = 3

// BrutalStrikeFeature returns the FeatureDefinition for Brutal Strike's +1d10
// on-hit damage rider, of the weapon's damage type. Mirrors HexFeature: an
// EffectExtraDamageDice on TriggerOnDamageRoll, so it flows through
// buildFESDamageBreakdown and is crit-doubled like any extra damage dice.
func BrutalStrikeFeature(damageType string) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Brutal Strike",
		Source: "barbarian",
		Effects: []Effect{
			{
				Type:        EffectExtraDamageDice,
				Trigger:     TriggerOnDamageRoll,
				Dice:        "1d10",
				DamageTypes: []string{damageType},
			},
		},
	}
}

// brutalStrikeEligible reports whether a Brutal Strike may fire on this attack:
// the chosen effect is known, the attacker carries the feature, they are using
// Reckless Attack (declared this attack OR the transient reckless marker from an
// earlier attack this turn), and it is a Strength-based melee attack (Brutal
// Strike is a "Strength-based attack roll"). The choice check is first so a
// non-brutal attack short-circuits before the char.Features unmarshal. "forceful"
// is the only wired effect (Hamstring needs a new speed slug); when a second lands,
// promote this to a data-driven map that also collapses the applyBrutalStrike switch.
func brutalStrikeEligible(cmd AttackCommand, features pqtype.NullRawMessage, weapon refdata.Weapon, abilityUsed string) bool {
	if cmd.BrutalStrike != "forceful" {
		return false
	}
	if !hasFeatureEffect(features, brutalStrikeEffect) {
		return false
	}
	usingReckless := cmd.Reckless || HasCondition(cmd.Attacker.Conditions, "reckless")
	if !usingReckless {
		return false
	}
	return !IsRangedWeapon(weapon) && abilityUsed == "str"
}

// applyBrutalStrike resolves the chosen Brutal Strike effect on a hit. A no-op
// unless a brutal effect was recorded in ResolveAttack and the attack hit.
// Forceful Blow pushes the target 15 ft straight away (reusing the parameterized
// applyPushEffect). Mirrors applyCunningStrike's post-hit dispatch shape.
func (s *Service) applyBrutalStrike(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult) error {
	if !result.Hit {
		return nil
	}
	switch result.BrutalStrikeChoice {
	case "forceful":
		if err := s.applyPushEffect(ctx, attacker, target, brutalForcefulPushSquares); err != nil {
			return fmt.Errorf("applying brutal strike forceful blow push: %w", err)
		}
	}
	return nil
}
