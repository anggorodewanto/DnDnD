package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
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

// shieldMasterDexSaveBonus returns the save-roll bonus Shield Master's third
// rider grants a target: the equipped shield's AC bonus, added to a DEX saving
// throw against an effect that "targets only you". Returns 0 unless the save is a
// DEX save AND the target is a non-incapacitated PC with the Shield Master feat
// wielding a shield (RAW: the bonus applies only "if you aren't incapacitated").
//
// The caller (the single-target save-spell enqueue) already establishes the
// "targets only you" prerequisite — non-AoE, one target — so AoE effects (which
// target more than the saver) never reach here. COV-9.
func (s *Service) shieldMasterDexSaveBonus(ctx context.Context, target refdata.Combatant, saveAbility string) int {
	if !strings.EqualFold(saveAbility, "dex") {
		return 0
	}
	if IsIncapacitatedRaw(target.Conditions) {
		return 0
	}
	if !target.CharacterID.Valid {
		return 0
	}
	char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
	if err != nil {
		return 0
	}
	if !HasFeatureByName(char.Features.RawMessage, "Shield Master") {
		return 0
	}
	return s.equippedShieldACBonus(ctx, char)
}
