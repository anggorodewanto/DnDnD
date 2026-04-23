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

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeDMNotifier is a test double for dmqueue.Notifier capturing Post / Cancel
// / Resolve invocations so the freeform-action service refactor can be
// asserted without spinning up Discord or PostgreSQL.
type fakeDMNotifier struct {
	posts     []dmqueue.Event
	cancels   []string
	resolves  []string
	postID    string
	postErr   error
	cancelErr error
}

func (f *fakeDMNotifier) Post(_ context.Context, e dmqueue.Event) (string, error) {
	f.posts = append(f.posts, e)
	if f.postErr != nil {
		return "", f.postErr
	}
	id := f.postID
	if id == "" {
		id = "fake-item-id"
	}
	return id, nil
}

func (f *fakeDMNotifier) Cancel(_ context.Context, itemID, _ string) error {
	f.cancels = append(f.cancels, itemID)
	return f.cancelErr
}

func (f *fakeDMNotifier) Resolve(_ context.Context, itemID, _ string) error {
	f.resolves = append(f.resolves, itemID)
	return nil
}

func (f *fakeDMNotifier) Get(string) (dmqueue.Item, bool) { return dmqueue.Item{}, false }
func (f *fakeDMNotifier) ListPending() []dmqueue.Item     { return nil }

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

// --- Notifier integration (Phase 106a) ---

func TestFreeformAction_DispatchesToNotifierWhenWired(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	var capturedParams refdata.CreatePendingActionParams
	ms.createPendingActionFn = func(_ context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		capturedParams = arg
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			ActionText:  arg.ActionText,
			Status:      "pending",
		}, nil
	}

	notifier := &fakeDMNotifier{postID: "11111111-2222-3333-4444-555555555555"}
	svc := NewService(ms)
	svc.SetDMNotifier(notifier)

	result, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
		GuildID:    "g1",
		CampaignID: uuid.New().String(),
	})
	require.NoError(t, err)

	require.Len(t, notifier.posts, 1)
	posted := notifier.posts[0]
	assert.Equal(t, dmqueue.KindFreeformAction, posted.Kind)
	assert.Equal(t, "Thorn", posted.PlayerName)
	assert.Contains(t, posted.Summary, "flip the table")
	assert.Equal(t, "g1", posted.GuildID)

	// CreatePendingAction should be called with the dm_queue_item_id linked.
	assert.True(t, capturedParams.DmQueueItemID.Valid, "expected dm_queue_item_id to be wired into pending_action")
	assert.Equal(t, notifier.postID, capturedParams.DmQueueItemID.UUID.String())

	// DMQueueMessage stays populated for the discord handler caller.
	assert.Contains(t, result.DMQueueMessage, "Thorn")
}

// --- Exploration freeform cancel (Phase 110a) ---
//
// Exploration has no Turn / action economy, so the combat cancel path (which
// refunds an action resource) is not reusable. CancelExplorationFreeformAction
// mirrors the sentinel errors and edits #dm-queue when a notifier is wired.

func TestCancelExplorationFreeformAction_HappyPath(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()
	pendingActionID := uuid.New()

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          pendingActionID,
			EncounterID: encounterID,
			CombatantID: cid,
			ActionText:  "search the bookshelf",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(_ context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          arg.ID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			ActionText:  "search the bookshelf",
			Status:      arg.Status,
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)

	require.NoError(t, err)
	assert.Equal(t, "cancelled", result.PendingAction.Status)
	assert.Equal(t, "search the bookshelf", result.PendingAction.ActionText)
}

func TestCancelExplorationFreeformAction_NoPendingAction(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()

	// default mock returns error for GetPendingActionByCombatant

	svc := NewService(ms)
	_, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoPendingAction))
}

func TestCancelExplorationFreeformAction_AlreadyResolved(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: encounterID,
			CombatantID: cid,
			ActionText:  "search the bookshelf",
			Status:      "resolved",
		}, nil
	}

	svc := NewService(ms)
	_, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrActionAlreadyResolved))
}

func TestCancelExplorationFreeformAction_UpdateStatusFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "search the bookshelf",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(_ context.Context, _ refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating pending action status")
}

func TestCancelExplorationFreeformAction_DispatchesCancelToNotifier(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()
	dmItemID := uuid.New()

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:            uuid.New(),
			EncounterID:   encounterID,
			CombatantID:   cid,
			ActionText:    "search the bookshelf",
			Status:        "pending",
			DmQueueItemID: uuid.NullUUID{UUID: dmItemID, Valid: true},
		}, nil
	}
	ms.updatePendingActionStatusFn = func(_ context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: arg.ID, Status: arg.Status, ActionText: "search the bookshelf"}, nil
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(ms)
	svc.SetDMNotifier(notifier)

	_, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)
	require.NoError(t, err)
	require.Len(t, notifier.cancels, 1)
	assert.Equal(t, dmItemID.String(), notifier.cancels[0])
}

func TestCancelExplorationFreeformAction_DoesNotRefundTurnResource(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			CombatantID: cid,
			ActionText:  "search the bookshelf",
			Status:      "pending",
		}, nil
	}
	ms.updatePendingActionStatusFn = func(_ context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: arg.ID, Status: arg.Status, ActionText: "search the bookshelf"}, nil
	}

	var updateTurnCalled bool
	ms.updateTurnActionsFn = func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		updateTurnCalled = true
		return refdata.Turn{}, nil
	}

	svc := NewService(ms)
	_, err := svc.CancelExplorationFreeformAction(context.Background(), combatantID)
	require.NoError(t, err)
	assert.False(t, updateTurnCalled, "exploration cancel must not touch turn actions (no turn exists)")
}

func TestCancelFreeformAction_DispatchesCancelToNotifier(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()
	dmItemID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.ActionUsed = true

	ms.getPendingActionByCombatantFn = func(_ context.Context, cid uuid.UUID) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:            uuid.New(),
			EncounterID:   encounterID,
			CombatantID:   cid,
			ActionText:    "flip the table",
			Status:        "pending",
			DmQueueItemID: uuid.NullUUID{UUID: dmItemID, Valid: true},
		}, nil
	}
	ms.updatePendingActionStatusFn = func(_ context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:         arg.ID,
			Status:     arg.Status,
			ActionText: "flip the table",
		}, nil
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(ms)
	svc.SetDMNotifier(notifier)

	result, err := svc.CancelFreeformAction(context.Background(), CancelFreeformActionCommand{
		Combatant: combatant,
		Turn:      turn,
	})
	require.NoError(t, err)
	require.Len(t, notifier.cancels, 1)
	assert.Equal(t, dmItemID.String(), notifier.cancels[0])
	// DMQueueEditMessage still produced for legacy callers.
	assert.Contains(t, result.DMQueueEditMessage, "Cancelled by player")
}
