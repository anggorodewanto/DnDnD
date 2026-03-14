package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
)

// Errors for freeform actions.
var (
	ErrNoPendingAction      = fmt.Errorf("no pending freeform action to cancel")
	ErrActionAlreadyResolved = fmt.Errorf("freeform action already resolved by DM")
)

// FreeformActionCommand holds the inputs for a freeform action.
type FreeformActionCommand struct {
	Combatant  refdata.Combatant
	Turn       refdata.Turn
	ActionText string
}

// FreeformActionResult holds the outputs of a freeform action.
type FreeformActionResult struct {
	Turn           refdata.Turn
	PendingAction  refdata.PendingAction
	CombatLog      string
	DMQueueMessage string
}

// FreeformAction handles the /action [freeform text] command.
// Costs an action. Creates a pending action record for the DM to resolve.
func (s *Service) FreeformAction(ctx context.Context, cmd FreeformActionCommand) (FreeformActionResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return FreeformActionResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return FreeformActionResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return FreeformActionResult{}, err
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return FreeformActionResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	pendingAction, err := s.store.CreatePendingAction(ctx, refdata.CreatePendingActionParams{
		EncounterID: cmd.Turn.EncounterID,
		CombatantID: cmd.Combatant.ID,
		ActionText:  cmd.ActionText,
	})
	if err != nil {
		return FreeformActionResult{}, fmt.Errorf("creating pending action: %w", err)
	}

	dmQueueMsg := fmt.Sprintf("🎭 **Action** — %s: \"%s\"", cmd.Combatant.DisplayName, cmd.ActionText)
	combatLog := fmt.Sprintf("🎭 %s: \"%s\" — sent to DM queue", cmd.Combatant.DisplayName, cmd.ActionText)

	return FreeformActionResult{
		Turn:           updatedTurn,
		PendingAction:  pendingAction,
		CombatLog:      combatLog,
		DMQueueMessage: dmQueueMsg,
	}, nil
}

// CancelFreeformActionCommand holds the inputs for cancelling a freeform action.
type CancelFreeformActionCommand struct {
	Combatant refdata.Combatant
}

// CancelFreeformActionResult holds the outputs of cancelling a freeform action.
type CancelFreeformActionResult struct {
	PendingAction       refdata.PendingAction
	CombatLog           string
	DMQueueEditMessage  string
}

// CancelFreeformAction handles the /action cancel command.
// Finds the combatant's pending freeform action and marks it cancelled.
func (s *Service) CancelFreeformAction(ctx context.Context, cmd CancelFreeformActionCommand) (CancelFreeformActionResult, error) {
	pendingAction, err := s.store.GetPendingActionByCombatant(ctx, cmd.Combatant.ID)
	if err != nil {
		return CancelFreeformActionResult{}, ErrNoPendingAction
	}

	if pendingAction.Status == "resolved" {
		return CancelFreeformActionResult{}, ErrActionAlreadyResolved
	}

	updatedAction, err := s.store.UpdatePendingActionStatus(ctx, refdata.UpdatePendingActionStatusParams{
		ID:     pendingAction.ID,
		Status: "cancelled",
	})
	if err != nil {
		return CancelFreeformActionResult{}, fmt.Errorf("updating pending action status: %w", err)
	}

	dmEditMsg := fmt.Sprintf("~~🎭 Action — %s: \"%s\"~~ Cancelled by player",
		cmd.Combatant.DisplayName, pendingAction.ActionText)
	combatLog := fmt.Sprintf("🎭 %s cancelled freeform action: \"%s\"",
		cmd.Combatant.DisplayName, pendingAction.ActionText)

	return CancelFreeformActionResult{
		PendingAction:      updatedAction,
		CombatLog:          combatLog,
		DMQueueEditMessage: dmEditMsg,
	}, nil
}
