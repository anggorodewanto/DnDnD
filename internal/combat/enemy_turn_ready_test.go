package combat

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ISSUE-057: enemy_turn_ready dm-queue item lifecycle ---
//
// When a DM-controlled combatant's turn starts, postEnemyTurnReady posts an
// enemy_turn_ready prompt to #dm-queue. That item used to leak: nothing ever
// cancelled it once the NPC's turn was actually run, so it stayed `pending`
// and kept misleading the DM Console next_step ("Resolve Enemy Turn Ready
// from Ghoul: is up") long after the turn had moved on. The record/resolve
// tracker below mirrors the SR-028 pending-OA sweep.

func TestResolveEnemyTurnReady_CancelsThroughNotifier(t *testing.T) {
	encounterID := uuid.New()
	notifier := &fakeDMNotifier{}

	svc := NewService(defaultMockStore())
	svc.SetDMNotifier(notifier)

	svc.recordEnemyTurnReady(encounterID, "etr-item-1")
	svc.resolveEnemyTurnReady(context.Background(), encounterID)

	require.Len(t, notifier.cancels, 1, "the enemy_turn_ready stub should be cancelled")
	assert.Equal(t, "etr-item-1", notifier.cancels[0])

	// Second resolve is a no-op (the tracker was drained).
	svc.resolveEnemyTurnReady(context.Background(), encounterID)
	assert.Len(t, notifier.cancels, 1, "second resolve should drain to empty")
}

func TestRecordEnemyTurnReady_EmptyIDIgnored(t *testing.T) {
	encounterID := uuid.New()
	notifier := &fakeDMNotifier{}

	svc := NewService(defaultMockStore())
	svc.SetDMNotifier(notifier)

	// Post returns "" when no #dm-queue is configured for the guild.
	svc.recordEnemyTurnReady(encounterID, "")
	svc.resolveEnemyTurnReady(context.Background(), encounterID)

	assert.Empty(t, notifier.cancels, "empty item IDs are skipped")
}

func TestResolveEnemyTurnReady_NoNotifierStillDrains(t *testing.T) {
	encounterID := uuid.New()

	svc := NewService(defaultMockStore())
	// No SetDMNotifier — resolve must still drain so a later wiring of the
	// notifier doesn't re-cancel the same stale item.
	svc.recordEnemyTurnReady(encounterID, "etr-orphan-1")
	svc.resolveEnemyTurnReady(context.Background(), encounterID)

	notifier := &fakeDMNotifier{}
	svc.SetDMNotifier(notifier)
	svc.resolveEnemyTurnReady(context.Background(), encounterID)
	assert.Empty(t, notifier.cancels, "post-drain resolve should not re-cancel a stale item")
}

// TestExecuteEnemyTurn_ResolvesEnemyTurnReadyItem proves running an NPC's
// turn through the Turn Builder clears its own enemy_turn_ready stub, so the
// DM Console next_step no longer points at an enemy whose turn is already
// taken (ISSUE-057).
func TestExecuteEnemyTurn_ResolvesEnemyTurnReadyItem(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: npcID, DisplayName: "Ghoul", IsNpc: true, IsAlive: true}, nil
		},
		getActiveTurnByEncounterIDFn: func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		createActionLogFn: func(_ context.Context, _ refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(store)
	svc.SetDMNotifier(notifier)

	// An enemy_turn_ready prompt was posted when this NPC's turn started.
	svc.recordEnemyTurnReady(encounterID, "etr-ghoul-1")

	_, err := svc.ExecuteEnemyTurn(
		context.Background(),
		encounterID,
		TurnPlan{CombatantID: npcID},
		newDeterministicRoller(15, 4, 3),
	)
	require.NoError(t, err)

	require.Len(t, notifier.cancels, 1, "running the enemy turn should cancel its enemy_turn_ready stub")
	assert.Equal(t, "etr-ghoul-1", notifier.cancels[0])
}
