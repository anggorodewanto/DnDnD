package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// SecondWindHealDice returns the Second Wind healing expression at the given
// fighter level: 1d10 + fighter level.
func SecondWindHealDice(fighterLevel int) string {
	return fmt.Sprintf("1d10+%d", fighterLevel)
}

// SecondWindCommand holds the inputs for a Second Wind bonus action.
type SecondWindCommand struct {
	Fighter refdata.Combatant
	Turn    refdata.Turn
}

// SecondWindResult holds the output of a Second Wind bonus action.
type SecondWindResult struct {
	HPRestored    int32
	HPAfter       int32
	UsesRemaining int
	CombatLog     string
	Turn          refdata.Turn
}

// SecondWind handles the /bonus second-wind command. As a bonus action the
// Fighter regains 1d10 + fighter level HP (self-heal), spending one use from
// the Second Wind pool, which recharges on a short or long rest.
func (s *Service) SecondWind(ctx context.Context, cmd SecondWindCommand, roller *dice.Roller) (SecondWindResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return SecondWindResult{}, err
	}

	if !cmd.Fighter.CharacterID.Valid {
		return SecondWindResult{}, fmt.Errorf("Second Wind requires a character (not an NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Fighter.CharacterID.UUID)
	if err != nil {
		return SecondWindResult{}, fmt.Errorf("getting character: %w", err)
	}

	fighterLevel := ClassLevelFromJSON(char.Classes, "Fighter")
	if fighterLevel < 1 {
		return SecondWindResult{}, fmt.Errorf("Second Wind requires Fighter class")
	}

	featureUses, remaining, err := ParseFeatureUses(char, FeatureKeySecondWind)
	if err != nil {
		return SecondWindResult{}, err
	}
	if remaining <= 0 {
		return SecondWindResult{}, fmt.Errorf("no Second Wind uses remaining")
	}

	newRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeySecondWind, featureUses, remaining)
	if err != nil {
		return SecondWindResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return SecondWindResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return SecondWindResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	rollResult, err := roller.Roll(SecondWindHealDice(fighterLevel))
	if err != nil {
		return SecondWindResult{}, fmt.Errorf("rolling Second Wind healing: %w", err)
	}

	hpAfter := min(cmd.Fighter.HpCurrent+int32(rollResult.Total), cmd.Fighter.HpMax)
	hpRestored := hpAfter - cmd.Fighter.HpCurrent
	if _, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        cmd.Fighter.ID,
		HpCurrent: hpAfter,
		TempHp:    cmd.Fighter.TempHp,
		IsAlive:   true,
	}); err != nil {
		return SecondWindResult{}, fmt.Errorf("updating fighter HP: %w", err)
	}
	// Parity with Lay on Hands: if a downed fighter is somehow healed from 0,
	// reset death-save tallies and drop the dying bundle. No-op when conscious.
	if _, err := s.MaybeResetDeathSavesOnHeal(ctx, cmd.Fighter, hpAfter); err != nil {
		return SecondWindResult{}, fmt.Errorf("resetting death state on heal: %w", err)
	}

	combatLog := fmt.Sprintf("\U0001f4aa  %s uses Second Wind — regains %d HP (%d use(s) remaining)",
		cmd.Fighter.DisplayName, hpRestored, newRemaining)

	return SecondWindResult{
		HPRestored:    hpRestored,
		HPAfter:       hpAfter,
		UsesRemaining: newRemaining,
		CombatLog:     combatLog,
		Turn:          updatedTurn,
	}, nil
}
