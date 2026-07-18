package combat

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ISSUE-049 (bonus-action variant): DM-side restoration of a combatant's spent
// bonus action mid-turn — the sibling of RestoreTurnAction for the main action.
//
// A player who throws away their bonus action (e.g. an off-hand attack at an
// already-dead target) has no way to reclaim it: the resource is marked spent and
// nothing reachable from the DM dashboard clears it. RestoreTurnBonusAction fills
// that gap, handing the active combatant their bonus action back so they may use
// it again this turn. Unlike the main-action restore it never touches the action,
// attack count, or movement — only the bonus-action flags.

// ErrNoBonusActionToRestore is returned when the active turn's bonus action is
// already available — there is nothing to give back.
var ErrNoBonusActionToRestore = errors.New("combatant has not spent their bonus action")

// RestoreTurnBonusAction returns the active combatant's bonus action — clearing
// bonus_action_used and the bonus-action leveled-spell flag — so they may use it
// again this turn. The main action, per-turn attack count and movement are all
// left untouched. It targets the active turn and rejects a combatantID that is not
// the one currently acting (ErrNotActiveCombatant) or whose bonus action is
// already available (ErrNoBonusActionToRestore).
func (s *Service) RestoreTurnBonusAction(ctx context.Context, encounterID, combatantID uuid.UUID) (ActionRestoration, error) {
	turn, err := s.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		return ActionRestoration{}, fmt.Errorf("getting active turn: %w", err)
	}
	if turn.CombatantID != combatantID {
		return ActionRestoration{}, ErrNotActiveCombatant
	}
	if !turn.BonusActionUsed {
		return ActionRestoration{}, ErrNoBonusActionToRestore
	}

	combatant, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return ActionRestoration{}, fmt.Errorf("getting combatant: %w", err)
	}

	turn = RefundResource(turn, ResourceBonusAction) // bonus_action_used = false
	turn.BonusActionSpellCast = false
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return ActionRestoration{}, fmt.Errorf("updating turn: %w", err)
	}

	return ActionRestoration{CombatantID: combatantID, CombatantName: combatant.DisplayName}, nil
}
