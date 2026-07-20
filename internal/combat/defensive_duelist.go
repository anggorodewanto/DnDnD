package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Defensive Duelist (2024 PHB p.203, "Parry"):
//
//	"If you're holding a Finesse weapon and another creature hits you with a
//	 melee attack, you can take a Reaction to add your Proficiency Bonus to your
//	 Armor Class, potentially causing the attack to miss you. You gain this bonus
//	 to your AC against melee attacks until the start of your next turn."
//
// One Reaction therefore buys the +PB against EVERY melee attack until the start
// of the defender's next turn — not just the triggering swing. The reaction is
// still spent exactly once; only the AC duration lingers.

// defensiveDuelistReactionID is the stable slug of the +AC reaction option. The
// lingering-AC marker is stamped only for this reaction, so a post-hit
// damage-halving reaction (Uncanny Dodge) can never inherit the duration.
const defensiveDuelistReactionID = "defensive-duelist"

// defensiveDuelistACCondition is the transient marker carrying the lingering
// +PB AC. It expires through the generic start_of_turn condition-expiry sweep
// (processExpiredConditions), sourced to the defender so it clears at the start
// of THEIR next turn — the same seam Steady Aim uses for its advantage marker.
// It is stripped at combat end by ClearCombatConditions (any ExpiresOn marker).
const defensiveDuelistACCondition = "defensive_duelist_ac"

// meleeReachThresholdFt is the largest reach_ft an attack may declare and still
// count as melee.
//
// Caveat: the seeded creature data overloads a single `reach_ft` field for both
// melee reach AND ranged range (a longbow is stored as reach_ft 150, a javelin
// as 30), and AttackStep does not carry the CreatureAttackEntry.RangeFt field at
// all — so melee/ranged cannot be read off the step exactly. 20ft is the largest
// true melee reach in the seed data (gargantuan dragon tails); everything from
// 25ft up is a thrown or fired weapon. The two known misclassifications are the
// 30ft-reach Balor Whip and Kraken Tentacle, which are melee but read as ranged.
const meleeReachThresholdFt = 20

// isMeleeAttackStep reports whether an attack step is a melee attack, so the
// lingering Defensive Duelist AC applies to it. A zero reach_ft means
// unspecified (the seeded swarm "Bites" entries) and is treated as melee.
func isMeleeAttackStep(a *AttackStep) bool {
	if a == nil {
		return false
	}
	return a.ReachFt <= meleeReachThresholdFt
}

// lingeringDefensiveDuelistAC returns the AC bonus a defender still carries from
// a Defensive Duelist reaction spent earlier this round, or 0. RAW scopes the
// lingering bonus to melee attacks, so a ranged swing gets nothing.
func lingeringDefensiveDuelistAC(target refdata.Combatant, attack *AttackStep) int {
	if !isMeleeAttackStep(attack) {
		return 0
	}
	marker, ok := GetCondition(target.Conditions, defensiveDuelistACCondition)
	if !ok {
		return 0
	}
	return marker.ACBonus
}

// applyLingeringDefensiveDuelistAC stamps the marker that keeps the +PB up
// against melee attacks until the start of the defender's next turn. Called
// once, right after the reaction is marked spent — it grants no extra reaction.
// Re-stamping is a no-op: the bonus is the defender's proficiency bonus either
// way, and a duplicate marker would just be dead weight in the JSONB array.
func (s *Service) applyLingeringDefensiveDuelistAC(ctx context.Context, defenderID uuid.UUID, acBonus int) error {
	if acBonus <= 0 {
		return nil
	}
	defender, err := s.store.GetCombatant(ctx, defenderID)
	if err != nil {
		return fmt.Errorf("getting defender: %w", err)
	}
	if HasCondition(defender.Conditions, defensiveDuelistACCondition) {
		return nil
	}
	marker := CombatCondition{
		Condition:         defensiveDuelistACCondition,
		DurationRounds:    1,
		SourceCombatantID: defenderID.String(),
		ExpiresOn:         "start_of_turn",
		ACBonus:           acBonus,
	}
	if _, _, err := s.ApplyCondition(ctx, defenderID, marker); err != nil {
		return fmt.Errorf("applying Defensive Duelist AC marker: %w", err)
	}
	return nil
}
