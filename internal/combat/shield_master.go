package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
)

// ShieldMasterShove handles /bonus shield (COV-9). After taking the Attack
// action, a character with the Shield Master feat who is holding a shield may
// shove a creature within 5 ft as a bonus action — either knocking it prone or
// pushing it 5 ft (cmd.Mode), using the same contested check as /shove.
//
// Unlike /shove (which costs an action), this spends the bonus action: it shares
// the shove core via resolveShove(..., ResourceBonusAction). The gates below run
// before resolveShove so a missing feat/shield never reaches the roll.
func (s *Service) ShieldMasterShove(ctx context.Context, cmd ShoveCommand, roller *dice.Roller) (ShoveResult, error) {
	if ok, reason := CanActRaw(cmd.Shover.Conditions); !ok {
		return ShoveResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return ShoveResult{}, err
	}

	if !cmd.Shover.CharacterID.Valid {
		return ShoveResult{}, fmt.Errorf("Shield Master shove requires a character (not NPC)")
	}
	char, err := s.store.GetCharacter(ctx, cmd.Shover.CharacterID.UUID)
	if err != nil {
		return ShoveResult{}, fmt.Errorf("getting character: %w", err)
	}
	if !HasFeatureByName(char.Features.RawMessage, "Shield Master") {
		return ShoveResult{}, fmt.Errorf("Shield Master shove requires the Shield Master feat")
	}

	// The feat triggers off the Attack action: an attack must already have been
	// made this turn (mirrors Crossbow Expert — the AttacksRemaining basis
	// correctly excludes a cast-a-spell action, unlike Turn.ActionUsed).
	maxAttacks := int32(s.resolveAttacksPerAction(ctx, char))
	if cmd.Turn.AttacksRemaining >= maxAttacks {
		return ShoveResult{}, fmt.Errorf("Shield Master shove requires you to have taken the Attack action this turn first")
	}

	if !s.hasEquippedShield(ctx, char) {
		return ShoveResult{}, fmt.Errorf("Shield Master shove requires a shield equipped")
	}

	return s.resolveShove(ctx, cmd, roller, ResourceBonusAction)
}
