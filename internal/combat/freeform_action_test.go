package combat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestFreeformAction_HappyPath(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			ActionText:  arg.ActionText,
			Status:      "pending",
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed)
	assert.Equal(t, "pending", result.PendingAction.Status)
	assert.Equal(t, "flip the table", result.PendingAction.ActionText)
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "flip the table")
	assert.Contains(t, result.DMQueueMessage, "🎭")
	assert.Contains(t, result.DMQueueMessage, "**Action**")
	assert.Contains(t, result.DMQueueMessage, "Thorn")
	assert.Contains(t, result.DMQueueMessage, "flip the table")
}

func TestFreeformAction_ActionAlreadyUsed(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.ActionUsed = true

	svc := NewService(ms)
	_, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResourceSpent))
}

func TestFreeformAction_CannotActWhileIncapacitated(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	combatant.Conditions = json.RawMessage(`[{"condition":"incapacitated"}]`)
	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "incapacitated")
}

func TestFreeformAction_CreatePendingActionFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating pending action")
}

func TestFreeformAction_UpdateTurnActionsFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// --- Cancel Freeform Action Tests ---

func TestCancelFreeformAction_HappyPath(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()
	pendingActionID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.ActionUsed = true // action was consumed by the freeform action

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          pendingActionID,
			EncounterID: encounterID,
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          arg.ID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			ActionText:  "flip the table",
			Status:      arg.Status,
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      turn,
	})

	require.NoError(t, err)
	assert.Equal(t, "cancelled", result.PendingAction.Status)
	assert.False(t, result.Turn.ActionUsed, "action should be refunded")
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "cancelled")
	assert.Contains(t, result.CombatLog, "flip the table")
	assert.Contains(t, result.DMQueueEditMessage, "~~")
	assert.Contains(t, result.DMQueueEditMessage, "Cancelled by player")
	assert.Contains(t, result.DMQueueEditMessage, "flip the table")
}

func TestCancelFreeformAction_NoPendingAction(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")

	// Default mock returns error for GetPendingActionByCombatant

	svc := NewService(ms)
	_, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      makeBasicTurn(),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoPendingAction))
}

func TestCancelFreeformAction_AlreadyResolved(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: encounterID,
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "resolved",
		}, nil
	}

	svc := NewService(ms)
	_, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      makeBasicTurn(),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrActionAlreadyResolved))
}

func TestCancelFreeformAction_UpdateStatusFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      makeBasicTurn(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating pending action status")
}

func TestFreeformAction_DMQueueMessageFormat(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	svc := NewService(ms)
	result, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})

	require.NoError(t, err)
	// Exact format from spec
	assert.Equal(t, `🎭 **Action** — Thorn: "flip the table"`, result.DMQueueMessage)
}

func TestCancelFreeformAction_RefundsActionAndPersists(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.ActionUsed = true // action was consumed

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: arg.ID, Status: arg.Status, ActionText: "flip the table"}, nil
	}

	var capturedParams refdata.UpdateTurnActionsParams
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		capturedParams = arg
		return refdata.Turn{}, nil
	}

	svc := NewService(ms)
	result, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      turn,
	})

	require.NoError(t, err)
	assert.False(t, result.Turn.ActionUsed, "action should be refunded")
	assert.False(t, capturedParams.ActionUsed, "DB should persist action_used=false")
}

func TestCancelFreeformAction_UpdateTurnFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.ActionUsed = true

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: arg.ID, Status: arg.Status, ActionText: "flip the table"}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      turn,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refunding action")
}

func TestCancelFreeformAction_DMQueueEditFormat(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")

	ms.getPendingActionByCombatantFn = func(ctx context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "flip the table",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: arg.ID, Status: arg.Status, ActionText: "flip the table"}, nil
	}

	svc := NewService(ms)
	result, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      makeBasicTurn(),
	})

	require.NoError(t, err)
	// Exact format from spec
	assert.Equal(t, `~~🎭 Action — Thorn: "flip the table"~~ Cancelled by player`, result.DMQueueEditMessage)
}
