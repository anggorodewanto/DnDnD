package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// ReactionOption is a single reaction a targeted PC may declare against an
// incoming attack. The DM/bot presents these in the Turn Builder before executing.
// There are two flavors, distinguished by when they resolve — both forward-only,
// nothing retroactive:
//   - +AC reactions (Defensive Duelist): resolve at roll time — ACBonus is folded
//     into the hit check, so the only transition is hit→miss (damage untouched).
//     Defensive Duelist's bonus then lingers against melee attacks until the
//     start of the defender's next turn (defensive_duelist_ac marker) without
//     costing a second reaction.
//   - damage-halving reactions (Uncanny Dodge): resolve post-hit — HalveDamage
//     halves the pre-rolled damage before it is written to HP, and (being tied to
//     a hit) is only consumed when the attack lands.
//
// Modelled as a list so future reactions drop in as additional options.
type ReactionOption struct {
	ID          string `json:"id"`                     // stable slug, e.g. "defensive-duelist"
	Label       string `json:"label"`                  // button label, e.g. "Defensive Duelist (+3 AC)"
	ACBonus     int    `json:"ac_bonus"`               // AC added against the incoming attack if chosen (+AC reactions)
	HalveDamage bool   `json:"halve_damage,omitempty"` // halve the incoming attack's damage (Uncanny Dodge); ACBonus is 0
	Reason      string `json:"reason"`                 // short reason for the combat log, e.g. "Defensive Duelist"
}

// defensiveDuelistReaction returns the Defensive Duelist option when the target
// has the feat and is wielding a finesse weapon. The reaction adds the target's
// proficiency bonus to AC against the triggering melee attack AND — per 2024 PHB
// p.203 — against every melee attack until the start of the defender's next
// turn; ExecuteEnemyTurn stamps that lingering half via
// applyLingeringDefensiveDuelistAC. Pure: reaction availability (a free
// reaction) is gated by the caller.
func defensiveDuelistReaction(featuresJSON []byte, mainHand refdata.Weapon, profBonus int) (ReactionOption, bool) {
	if !HasFeatureByName(featuresJSON, "Defensive Duelist") {
		return ReactionOption{}, false
	}
	if !HasProperty(mainHand, "finesse") {
		return ReactionOption{}, false
	}
	return ReactionOption{
		ID:      defensiveDuelistReactionID,
		Label:   fmt.Sprintf("Defensive Duelist (+%d AC)", profBonus),
		ACBonus: profBonus,
		Reason:  "Defensive Duelist",
	}, true
}

// uncannyDodgeReaction returns the Uncanny Dodge option when the target has the
// feature (Rogue 5+, seeded as the `uncanny_dodge` mechanical effect). Unlike a
// +AC reaction it does not change hit/miss; it halves the incoming attack's damage
// at execute time, before the damage is written to HP (no retroactive heal-back).
// Pure: reaction availability (a free reaction) is gated by the caller.
func uncannyDodgeReaction(features pqtype.NullRawMessage) (ReactionOption, bool) {
	if !hasFeatureEffect(features, "uncanny_dodge") {
		return ReactionOption{}, false
	}
	return ReactionOption{
		ID:          "uncanny-dodge",
		Label:       "Uncanny Dodge (halve damage)",
		HalveDamage: true,
		Reason:      "Uncanny Dodge",
	}, true
}

// AvailableReactions returns the reaction options a targeted PC may declare
// against an incoming attack. Returns empty for NPC targets, for PCs whose
// reaction is already spent this round, and for PCs with no qualifying reaction.
// Built to be extended with more reactions later.
func (s *Service) AvailableReactions(ctx context.Context, target refdata.Combatant, encounterID uuid.UUID) ([]ReactionOption, error) {
	if !target.CharacterID.Valid {
		return nil, nil
	}
	free, err := s.CanDeclareReaction(ctx, encounterID, target.ID)
	if err != nil {
		return nil, fmt.Errorf("checking reaction availability: %w", err)
	}
	if !free {
		return nil, nil
	}

	char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
	if err != nil {
		return nil, fmt.Errorf("loading target character: %w", err)
	}

	var mainHand refdata.Weapon
	if char.EquippedMainHand.Valid && char.EquippedMainHand.String != "" {
		if w, werr := s.store.GetWeapon(ctx, char.EquippedMainHand.String); werr == nil {
			mainHand = w
		}
	}

	var opts []ReactionOption
	if dd, ok := defensiveDuelistReaction(char.Features.RawMessage, mainHand, int(char.ProficiencyBonus)); ok {
		opts = append(opts, dd)
	}
	if ud, ok := uncannyDodgeReaction(char.Features); ok {
		opts = append(opts, ud)
	}
	return opts, nil
}

// FormatReactionDeclared renders the #combat-log / DM-timeline line announcing
// that a targeted PC used a reaction against an incoming attack.
func FormatReactionDeclared(defenderName string, opt ReactionOption) string {
	if opt.HalveDamage {
		return fmt.Sprintf("🛡️ %s uses %s — halves the damage", defenderName, opt.Reason)
	}
	return fmt.Sprintf("🛡️ %s uses %s — +%d AC", defenderName, opt.Reason, opt.ACBonus)
}

// applyReactionToRoll re-evaluates a pre-rolled attack result against a
// reaction-boosted AC. The enemy-turn plan pre-rolls each attack at the target's
// base AC; when the DM applies a +AC reaction at execute time we recompute the
// hit against baseAC+acBonus. A reaction only raises AC, so the only possible
// transition is hit→miss — damage is left untouched (the execute loop simply
// skips damage when Hit is false). A natural 20 always hits and a natural 1
// always misses, matching RollAttack.
func applyReactionToRoll(r *AttackRollResult, baseAC, acBonus int) {
	if r.Critical {
		r.Hit = true
		return
	}
	if r.ToHitRoll == 1 {
		r.Hit = false
		return
	}
	r.Hit = r.ToHitTotal >= baseAC+acBonus
}

// markPCReactionUsed records that a targeted PC spent a reaction during an enemy
// turn by writing a used declaration stamped with the current round. It does NOT
// go through ResolveReaction (which requires the PC to have a turn row this
// round) so it works no matter where the PC sits in initiative — the used_on_round
// stamp is what CanDeclareReaction/AvailableReactions read to block a second
// reaction against a later attacker in the same round.
func (s *Service) markPCReactionUsed(ctx context.Context, encounterID, targetID uuid.UUID, opt ReactionOption) error {
	activeTurn, err := s.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("getting active turn: %w", err)
	}
	decl, err := s.DeclareReaction(ctx, encounterID, targetID, opt.Reason)
	if err != nil {
		return fmt.Errorf("declaring reaction: %w", err)
	}
	if _, err := s.stampReactionUsed(ctx, decl.ID, activeTurn.RoundNumber); err != nil {
		return fmt.Errorf("marking reaction used: %w", err)
	}
	return nil
}
