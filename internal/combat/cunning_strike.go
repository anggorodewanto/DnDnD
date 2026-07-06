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
// (the effect's cost) before rolling the rest." Each effect makes the target save
// (DC 8 + proficiency + the rogue's DEX modifier) or suffer a condition. Wired
// effects (cunningStrikeRiders): Trip (Cost 1, DEX save or Prone) and Poison
// (Cost 1, CON save or Poisoned).
//
// The die cost is subtracted from the Sneak Attack extra-damage dice in
// populateAttackFES (before the roll); the rider is resolved post-hit in
// Service.Attack (applyCunningStrike), synchronously, mirroring the Topple
// mastery's inline save-or-condition (applyToppleSave).
//
// cunningStrikeRiders is the "save-or-condition family": every entry is a target
// save that lands an indefinite condition on a failure. Deferred effects that do
// NOT fit that shape need more than a new row (documented here so the next slice
// inherits the seam analysis):
//   - Withdraw (move without provoking OA) has no save and no condition — a
//     different resolution CATEGORY. Add a separate non-save handler when the
//     movement/OA trigger exists; do NOT widen the rider struct with optional
//     movement fields.
//   - Daze (a condition until the end of a turn) needs a duration: add
//     durationRounds/expiresOn to the rider (zero-value stays today's indefinite,
//     so Trip/Poison are unaffected), thread the current round into
//     applyCunningStrike to seed CombatCondition.StartedRound, and note that
//     isExpired keys expiry to the SOURCE's turn — "end of the TARGET's next turn"
//     (Daze RAW) needs an expiry-keying change, not just a duration.
//
// Also deferred: the Poisoner's Kit requirement on Poison (a binary inventory
// check — the one deferral most worth closing, no new infra); the per-turn
// re-save on the Poisoned condition (no repeated-save scheduler — the same
// indefinite-until-teardown model COV-2 uses for save-or-suck conditions); the
// "Large or smaller" size gate on Trip (Topple applies Prone without one today);
// and forgoing more than one die / stacking multiple effects on one Sneak Attack.

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

// cunningStrikeRider describes one Cunning Strike effect: the number of Sneak
// Attack dice forgone (diceCost), the ability the TARGET saves with, the
// condition applied on a failed save, and the player-facing label / fail-outcome
// text for the combat log. The save DC is always 8 + prof + the rogue's DEX
// (computed once in ResolveAttack), independent of which ability the target rolls.
type cunningStrikeRider struct {
	diceCost    int
	saveAbility string
	condition   string
	label       string // effect name in the log, e.g. "Trip"
	onFail      string // outcome text on a failed save, e.g. "knocked Prone"
}

// cunningStrikeRiders is the closed set of wired Cunning Strike effects. Adding
// an effect is one entry here plus one /attack cunning Choice — the resolution
// (dice forgone, save, condition, log) is fully data-driven. Withdraw is absent:
// it needs a movement/OA trigger that does not exist yet.
var cunningStrikeRiders = map[string]cunningStrikeRider{
	"trip":   {diceCost: 1, saveAbility: "dex", condition: "prone", label: "Trip", onFail: "knocked Prone"},
	"poison": {diceCost: 1, saveAbility: "con", condition: "poisoned", label: "Poison", onFail: "Poisoned"},
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

// recordCunningStrike records the chosen Cunning Strike effect + its save DC on
// the result when the rogue opted into a known cunning effect, the attack hit,
// and Sneak Attack actually dealt damage — a non-empty CunningStrikeChoice is the
// "rider fired" gate downstream. The eligibility (feature present, choice known)
// is already baked into input.CunningStrike by populateAttackFES, so a
// non-rogue's request never reaches here. The save itself is rolled later in
// Service.Attack (applyCunningStrike).
func recordCunningStrike(result *AttackResult, input AttackInput, dc int) {
	if _, ok := cunningStrikeRiders[input.CunningStrike]; !ok {
		return
	}
	if !result.Hit || !sneakAttackDealt(*result) {
		return
	}
	result.CunningStrikeChoice = input.CunningStrike
	result.CunningStrikeSaveDC = dc
}

// applyCunningStrike resolves the target's save against the Cunning Strike DC and
// applies the rider's condition on a failure. A no-op unless a rider was recorded
// in ResolveAttack. Mirrors applyToppleSave, generalized over the rider's save
// ability + condition; sets result.CunningStrikeSaved so the log can report the
// outcome.
func (s *Service) applyCunningStrike(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	rider, ok := cunningStrikeRiders[result.CunningStrikeChoice]
	if !ok {
		return nil
	}
	saveBonus, err := s.resolveCombatantSaveBonus(ctx, target, rider.saveAbility)
	if err != nil {
		return fmt.Errorf("resolving cunning strike %s save: %w", rider.saveAbility, err)
	}
	d20Result, err := roller.RollD20(saveBonus, dice.Normal)
	if err != nil {
		return fmt.Errorf("rolling cunning strike %s save: %w", rider.saveAbility, err)
	}
	if d20Result.Total >= result.CunningStrikeSaveDC {
		result.CunningStrikeSaved = true
		return nil // save succeeds → no condition
	}
	cond := CombatCondition{
		Condition:         rider.condition,
		DurationRounds:    0,
		SourceCombatantID: attacker.ID.String(),
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, cond); err != nil {
		return fmt.Errorf("applying %s from cunning strike %s: %w", rider.condition, result.CunningStrikeChoice, err)
	}
	return nil
}
