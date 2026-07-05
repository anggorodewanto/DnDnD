package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
)

// steadyAimAdvantageCondition is the transient marker Steady Aim writes on the
// rogue. It grants the rogue advantage on their attack this turn and expires at
// the start of their next turn via the generic start_of_turn condition-expiry
// machinery (mirroring the reckless marker). Unlike vex/help advantage it is not
// target-scoped, and unlike reckless it grants no advantage to incoming attacks.
const steadyAimAdvantageCondition = "steady_aim_advantage"

// SteadyAimCommand holds the inputs for the Steady Aim bonus action.
type SteadyAimCommand struct {
	Rogue refdata.Combatant
	Turn  refdata.Turn
}

// SteadyAimResult holds the output of the Steady Aim bonus action.
type SteadyAimResult struct {
	CombatLog string
	Turn      refdata.Turn
}

// SteadyAim handles the /bonus steady-aim command. As a bonus action the rogue
// gives themselves advantage on their attack roll this turn. The advantage is a
// transient marker read by DetectAdvantage; it clears at the start of the
// rogue's next turn.
//
// The 2024/Tasha's downside — speed drops to 0 for the turn — is NOT enforced:
// the engine has no per-turn movement-budget gate to zero out (the same gap that
// defers Sentinel and the Polearm Master opportunity-attack half). It is called
// out in the combat log so the table can honor it.
func (s *Service) SteadyAim(ctx context.Context, cmd SteadyAimCommand) (SteadyAimResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return SteadyAimResult{}, err
	}

	if !cmd.Rogue.CharacterID.Valid {
		return SteadyAimResult{}, fmt.Errorf("Steady Aim requires a character (not an NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Rogue.CharacterID.UUID)
	if err != nil {
		return SteadyAimResult{}, fmt.Errorf("getting character: %w", err)
	}
	if !HasFeatureByName(char.Features.RawMessage, "Steady Aim") {
		return SteadyAimResult{}, fmt.Errorf("Steady Aim requires the Steady Aim feature")
	}

	marker := CombatCondition{
		Condition:         steadyAimAdvantageCondition,
		DurationRounds:    1,
		SourceCombatantID: cmd.Rogue.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, cmd.Rogue.ID, marker); err != nil {
		return SteadyAimResult{}, fmt.Errorf("applying Steady Aim marker: %w", err)
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return SteadyAimResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return SteadyAimResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	combatLog := fmt.Sprintf("\U0001f3af  %s takes Steady Aim — advantage on their attack this turn (speed 0 until end of turn)",
		cmd.Rogue.DisplayName)

	return SteadyAimResult{CombatLog: combatLog, Turn: updatedTurn}, nil
}
