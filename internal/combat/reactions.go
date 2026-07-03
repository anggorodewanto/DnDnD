package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ReactionOption is a single reaction a targeted PC may declare in the pre-roll
// reaction window. The DM/bot presents these before the attacker rolls; the
// chosen option's ACBonus is folded into the attack via AttackCommand.
// ReactionACBonus so nothing is resolved retroactively. Modelled as a list so
// future reactions (Shield, etc.) drop in as additional options.
type ReactionOption struct {
	ID      string // stable slug, e.g. "defensive-duelist"
	Label   string // button label, e.g. "Defensive Duelist (+3 AC)"
	ACBonus int    // AC added against the incoming attack if chosen
	Reason  string // short reason for the combat log, e.g. "Defensive Duelist"
}

// defensiveDuelistReaction returns the Defensive Duelist option when the target
// has the feat and is wielding a finesse weapon. The reaction adds the target's
// proficiency bonus to AC against one melee attack (2024 rules). Pure: reaction
// availability (a free reaction) is gated by the caller.
func defensiveDuelistReaction(featuresJSON []byte, mainHand refdata.Weapon, profBonus int) (ReactionOption, bool) {
	if !HasFeatureByName(featuresJSON, "Defensive Duelist") {
		return ReactionOption{}, false
	}
	if !HasProperty(mainHand, "finesse") {
		return ReactionOption{}, false
	}
	return ReactionOption{
		ID:      "defensive-duelist",
		Label:   fmt.Sprintf("Defensive Duelist (+%d AC)", profBonus),
		ACBonus: profBonus,
		Reason:  "Defensive Duelist",
	}, true
}

// AvailableReactions returns the reaction options a targeted PC may use against
// an incoming attack, for the pre-roll reaction window. Returns empty for NPC
// targets, for PCs whose reaction is already spent this round, and for PCs with
// no qualifying reaction. Built to be extended with more +AC reactions later.
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
	return opts, nil
}

// FormatReactionDeclared renders the #combat-log / DM-timeline line announcing
// that a targeted PC used a reaction in the pre-roll window, before the attack
// is rolled.
func FormatReactionDeclared(defenderName string, opt ReactionOption) string {
	return fmt.Sprintf("🛡️ %s uses %s — +%d AC", defenderName, opt.Reason, opt.ACBonus)
}
