package combat

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// Cunning Strike (2024 Rogue, level 5): "When you deal Sneak Attack damage, you
// can add one of the following effects, forgoing a number of Sneak Attack dice
// (the effect's cost) before rolling the rest." This slice wires the Trip effect
// (Cost 1 die): on a hit that dealt Sneak Attack damage, the target makes a
// Dexterity save (DC 8 + proficiency + the rogue's DEX modifier) or falls Prone.
//
// The die cost is subtracted from the Sneak Attack extra-damage dice in
// populateAttackFES (before the roll); the rider itself is resolved post-hit in
// Service.Attack (applyCunningStrikeTrip), synchronously, mirroring the Topple
// mastery's inline save-or-Prone (applyToppleSave). Deferred: the Poison and
// Withdraw effects (Poison has a re-save nuance; Withdraw needs the movement/OA
// trigger system that does not exist yet), and the "Large or smaller" size gate
// (Topple applies Prone without one today, so Trip matches for parity).

// cunningStrikeEffect is the seeded mechanical_effect slug that gates Cunning
// Strike. Detection is by slug via hasFeatureEffect (mirroring the Tactical
// Master / Evasion / Uncanny Dodge class-feature gates and the level-5 seed
// guard) so a rewording of the feature's display name can't silently break it.
const cunningStrikeEffect = "cunning_strike"

// sneakAttackFeatureName is the FeatureDefinition name whose extra-damage dice
// Cunning Strike forgoes, and the once-per-turn effect name that signals Sneak
// Attack actually dealt damage on a hit. The EffectExtraDamageDice type alone is
// not a discriminator (Hex / Hunter's Mark share it), so we key on the name.
const sneakAttackFeatureName = "Sneak Attack"

// cunningStrikeDiceCost returns the number of Sneak Attack dice a Cunning Strike
// option forgoes, or 0 for an unknown/empty choice (which makes the whole path
// inert). Trip costs one die.
func cunningStrikeDiceCost(choice string) int {
	if choice == "trip" {
		return 1
	}
	return 0
}

// reduceDiceCount subtracts `by` from the count of an "NdX" dice expression,
// preserving the die size and flooring the count at 0. A malformed expression
// (no 'd', or a non-integer count) is returned unchanged — defensive; the only
// caller feeds SneakAttackDice output ("Nd6"), which is always well-formed.
func reduceDiceCount(expr string, by int) string {
	if by <= 0 {
		return expr
	}
	count, rest, ok := strings.Cut(expr, "d")
	if !ok {
		return expr
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return expr
	}
	return strconv.Itoa(max(0, n-by)) + "d" + rest
}

// reduceSneakAttackDice forgoes `cost` dice from the Sneak Attack extra-damage
// effect in place, leaving every other feature (including sibling
// EffectExtraDamageDice riders like Hex) untouched. A no-op when cost <= 0 or no
// Sneak Attack effect is present.
func reduceSneakAttackDice(features []FeatureDefinition, cost int) {
	if cost <= 0 {
		return
	}
	for i := range features {
		if features[i].Name != sneakAttackFeatureName {
			continue
		}
		for j := range features[i].Effects {
			if features[i].Effects[j].Type != EffectExtraDamageDice {
				continue
			}
			features[i].Effects[j].Dice = reduceDiceCount(features[i].Effects[j].Dice, cost)
		}
	}
}

// sneakAttackDealt reports whether Sneak Attack fired (dealt damage) on this
// attack, read from the once-per-turn effect names populated during the FES
// damage pass. Cunning Strike may only be used "when you deal Sneak Attack
// damage".
func sneakAttackDealt(result AttackResult) bool {
	return slices.Contains(result.OncePerTurnEffectNames, sneakAttackFeatureName)
}

// recordCunningStrikeTrip records the Trip save DC on the result when the rogue
// opted into cunning:trip, the attack hit, and Sneak Attack actually dealt
// damage — the non-zero DC is the "trip fired" gate downstream. The eligibility
// (feature present) is already baked into input.CunningStrike by
// populateAttackFES, so a non-rogue's request never reaches here as "trip". The
// save itself is rolled later in Service.Attack (applyCunningStrikeTrip).
func recordCunningStrikeTrip(result *AttackResult, input AttackInput, dc int) {
	if input.CunningStrike != "trip" {
		return
	}
	if !result.Hit || !sneakAttackDealt(*result) {
		return
	}
	result.CunningStrikeTripDC = dc
}

// applyCunningStrikeTrip resolves the target's DEX save against the Cunning
// Strike Trip DC and applies Prone on a failure. A no-op unless
// result.CunningStrikeTrip was set in ResolveAttack. Mirrors applyToppleSave but
// on a DEX save; sets result.CunningStrikeTripSaved so the log can report the
// outcome.
func (s *Service) applyCunningStrikeTrip(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	if result.CunningStrikeTripDC <= 0 {
		return nil
	}
	dexSaveBonus, err := s.resolveCombatantSaveBonus(ctx, target, "dex")
	if err != nil {
		return fmt.Errorf("resolving cunning strike DEX save: %w", err)
	}
	d20Result, err := roller.RollD20(dexSaveBonus, dice.Normal)
	if err != nil {
		return fmt.Errorf("rolling cunning strike DEX save: %w", err)
	}
	if d20Result.Total >= result.CunningStrikeTripDC {
		result.CunningStrikeTripSaved = true
		return nil // save succeeds → no Prone
	}
	prone := CombatCondition{
		Condition:         "prone",
		DurationRounds:    0,
		SourceCombatantID: attacker.ID.String(),
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, prone); err != nil {
		return fmt.Errorf("applying prone from cunning strike trip: %w", err)
	}
	return nil
}
