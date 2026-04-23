package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// Errors for freeform actions.
var (
	ErrNoPendingAction       = fmt.Errorf("no pending freeform action to cancel")
	ErrActionAlreadyResolved = fmt.Errorf("freeform action already resolved by DM")
)

// FreeformActionCommand holds the inputs for a freeform action.
//
// GuildID and CampaignID are optional Phase 106a additions used to drive the
// dm-queue notifier when one is wired into the service. Existing call sites
// that leave them empty fall back to the legacy in-result message strings
// only.
type FreeformActionCommand struct {
	Combatant  refdata.Combatant
	Turn       refdata.Turn
	ActionText string
	GuildID    string
	CampaignID string
}

// FreeformActionResult holds the outputs of a freeform action.
type FreeformActionResult struct {
	Turn           refdata.Turn
	PendingAction  refdata.PendingAction
	CombatLog      string
	DMQueueMessage string
	// DMQueueItemID is the dmqueue item ID returned by the notifier when one
	// is wired. Empty when the legacy code path is used (no notifier set or
	// notifier reported the post as a no-op).
	DMQueueItemID string
}

// FreeformAction handles the /action [freeform text] command.
// Costs an action. Creates a pending action record for the DM to resolve.
// When the service has a DMNotifier wired, the action is also posted to the
// guild's #dm-queue and the returned itemID is linked into pending_actions.
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

	dmItemID, err := s.postFreeformActionToDMQueue(ctx, cmd)
	if err != nil {
		return FreeformActionResult{}, fmt.Errorf("posting to dm queue: %w", err)
	}

	pendingAction, err := s.store.CreatePendingAction(ctx, refdata.CreatePendingActionParams{
		EncounterID:   cmd.Turn.EncounterID,
		CombatantID:   cmd.Combatant.ID,
		ActionText:    cmd.ActionText,
		DmQueueItemID: nullableUUIDFromString(dmItemID),
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
		DMQueueItemID:  dmItemID,
	}, nil
}

// postFreeformActionToDMQueue dispatches a freeform-action notification
// through the wired Notifier (when present) and returns the resulting
// item ID. When no notifier is wired the call is a silent no-op so existing
// callers that don't (yet) inject one continue to work unchanged.
func (s *Service) postFreeformActionToDMQueue(ctx context.Context, cmd FreeformActionCommand) (string, error) {
	if s.dmNotifier == nil {
		return "", nil
	}
	return s.dmNotifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindFreeformAction,
		PlayerName: cmd.Combatant.DisplayName,
		Summary:    fmt.Sprintf("\"%s\"", cmd.ActionText),
		GuildID:    cmd.GuildID,
		CampaignID: cmd.CampaignID,
	})
}

// nullableUUIDFromString parses a UUID string into uuid.NullUUID. An empty
// or invalid string yields a NULL value (Valid=false).
func nullableUUIDFromString(s string) uuid.NullUUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}

// CancelFreeformActionCommand holds the inputs for cancelling a freeform action.
type CancelFreeformActionCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
}

// CancelFreeformActionResult holds the outputs of cancelling a freeform action.
type CancelFreeformActionResult struct {
	Turn               refdata.Turn
	PendingAction      refdata.PendingAction
	CombatLog          string
	DMQueueEditMessage string
}

// CancelFreeformAction handles the /action cancel command.
// Finds the combatant's pending freeform action, marks it cancelled, and refunds the action.
// Resource refund is persisted first so the player keeps their action even if the status update fails.
// When a DMNotifier is wired and the pending action carries a dm_queue_item_id,
// the same Cancel call is forwarded so the original Discord message is edited
// with the strikethrough "Cancelled by player" overlay.
func (s *Service) CancelFreeformAction(ctx context.Context, cmd CancelFreeformActionCommand) (CancelFreeformActionResult, error) {
	pendingAction, err := s.store.GetPendingActionByCombatant(ctx, cmd.Combatant.ID)
	if err != nil {
		return CancelFreeformActionResult{}, ErrNoPendingAction
	}

	if pendingAction.Status == "resolved" {
		return CancelFreeformActionResult{}, ErrActionAlreadyResolved
	}

	// Refund the action resource first so the player keeps their action even on partial failure.
	updatedTurn := RefundResource(cmd.Turn, ResourceAction)
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return CancelFreeformActionResult{}, fmt.Errorf("refunding action: %w", err)
	}

	updatedAction, err := s.store.UpdatePendingActionStatus(ctx, refdata.UpdatePendingActionStatusParams{
		ID:     pendingAction.ID,
		Status: "cancelled",
	})
	if err != nil {
		return CancelFreeformActionResult{}, fmt.Errorf("updating pending action status: %w", err)
	}

	if s.dmNotifier != nil && pendingAction.DmQueueItemID.Valid {
		// Best-effort: a notifier failure must not undo the DB state we just
		// committed. Errors are swallowed (callers can still surface them via
		// the legacy edit string fallback if needed).
		_ = s.dmNotifier.Cancel(ctx, pendingAction.DmQueueItemID.UUID.String(), "Cancelled by player")
	}

	dmEditMsg := fmt.Sprintf("~~🎭 Action — %s: \"%s\"~~ Cancelled by player",
		cmd.Combatant.DisplayName, pendingAction.ActionText)
	combatLog := fmt.Sprintf("🎭 %s cancelled freeform action: \"%s\"",
		cmd.Combatant.DisplayName, pendingAction.ActionText)

	return CancelFreeformActionResult{
		Turn:               updatedTurn,
		PendingAction:      updatedAction,
		CombatLog:          combatLog,
		DMQueueEditMessage: dmEditMsg,
	}, nil
}

// CancelExplorationFreeformAction handles /action cancel for an
// exploration-mode encounter (Phase 110a). Exploration has no Turn and no
// action economy, so the combat-mode cancel (which refunds an action
// resource) is not reusable. This variant:
//
//  1. Looks up the combatant's pending freeform action.
//  2. Returns ErrNoPendingAction / ErrActionAlreadyResolved symmetrically
//     with CancelFreeformAction so the Discord handler can surface the
//     same user-facing messages.
//  3. Marks the pending_actions row cancelled.
//  4. If a DMNotifier is wired and the row carries a dm_queue_item_id, the
//     Cancel call is forwarded so the original #dm-queue message is edited
//     with the strike-through "Cancelled by player" overlay. Notifier
//     errors are best-effort; the DB state has already been committed.
func (s *Service) CancelExplorationFreeformAction(ctx context.Context, combatantID uuid.UUID) (CancelFreeformActionResult, error) {
	pendingAction, err := s.store.GetPendingActionByCombatant(ctx, combatantID)
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

	if s.dmNotifier != nil && pendingAction.DmQueueItemID.Valid {
		// Best-effort: a notifier hiccup must not undo the DB state we
		// just committed.
		_ = s.dmNotifier.Cancel(ctx, pendingAction.DmQueueItemID.UUID.String(), "Cancelled by player")
	}

	dmEditMsg := fmt.Sprintf("~~🎭 Action — %s~~ Cancelled by player",
		pendingAction.ActionText)
	combatLog := fmt.Sprintf("🎭 cancelled freeform action: \"%s\"",
		pendingAction.ActionText)

	return CancelFreeformActionResult{
		PendingAction:      updatedAction,
		CombatLog:          combatLog,
		DMQueueEditMessage: dmEditMsg,
	}, nil
}
