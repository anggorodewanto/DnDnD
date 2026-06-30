package combat

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ISSUE-049: DM-side restoration of a combatant's spent action mid-turn.
//
// Granting a player's undo of an action they already took (e.g. a misplaced AoE
// spell whose pending saves the DM voids via ISSUE-048) left the action itself
// spent: the cast marked action_used — and, for a leveled spell, action_spell_cast
// — true and zeroed the seeded attack count (Cast-a-Spell is not the Attack
// action). Nothing reachable from the DM dashboard set those back, so the player
// could not re-take their turn's action. dispatchUndo restores HP/position/
// conditions only. RestoreTurnAction fills that gap: it hands the active
// combatant their action back so they may act again this turn.

var (
	// ErrNotActiveCombatant is returned when restoring the action of a combatant
	// who is not the one currently acting — action economy only exists for the
	// active turn.
	ErrNotActiveCombatant = errors.New("combatant is not the active turn")
	// ErrNoActionToRestore is returned when the active turn's action is already
	// available — there is nothing to give back.
	ErrNoActionToRestore = errors.New("combatant has not spent their action")
)

// ActionRestoration is the outcome of restoring a combatant's action. It carries
// no HP — only what the #combat-log correction needs.
type ActionRestoration struct {
	CombatantID   uuid.UUID
	CombatantName string
}

// RestoreTurnAction returns the active combatant's action — clearing action_used
// and the leveled-spell flag and reseeding the per-turn attack count the action's
// use consumed — so they may act again this turn. Movement is left untouched
// (restoring the action does not refund movement already spent). It targets the
// active turn and rejects a combatantID that is not the one currently acting
// (ErrNotActiveCombatant) or whose action is already available (ErrNoActionToRestore).
func (s *Service) RestoreTurnAction(ctx context.Context, encounterID, combatantID uuid.UUID) (ActionRestoration, error) {
	turn, err := s.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		return ActionRestoration{}, fmt.Errorf("getting active turn: %w", err)
	}
	if turn.CombatantID != combatantID {
		return ActionRestoration{}, ErrNotActiveCombatant
	}
	if !turn.ActionUsed {
		return ActionRestoration{}, ErrNoActionToRestore
	}

	combatant, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return ActionRestoration{}, fmt.Errorf("getting combatant: %w", err)
	}

	// Reseed the fresh per-turn attack count so the restored action also returns
	// the weapon attack(s) the cast zeroed — the player may choose to attack
	// instead of recasting.
	_, attacks, err := s.ResolveTurnResources(ctx, combatant)
	if err != nil {
		return ActionRestoration{}, fmt.Errorf("resolving turn resources: %w", err)
	}

	turn = RefundResource(turn, ResourceAction) // action_used = false
	turn.ActionSpellCast = false
	turn.AttacksRemaining = attacks
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return ActionRestoration{}, fmt.Errorf("updating turn: %w", err)
	}

	return ActionRestoration{CombatantID: combatantID, CombatantName: combatant.DisplayName}, nil
}
